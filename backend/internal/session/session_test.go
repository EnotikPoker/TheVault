package session

import (
	"encoding/hex"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"thevault/backend/internal/vault"
)

func TestSessionLifecycle(t *testing.T) {
	service, clock, winning, failing := testService(t)
	created, err := service.Create()
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != Locked || created.Attempts != 0 || created.Elapsed != 0 || created.History == nil || len(created.History) != 0 {
		t.Fatalf("unexpected new session: %+v", created)
	}
	if created.OpenedAt != nil || created.Explanations != nil || !created.CreatedAt.Equal(clock.Now()) {
		t.Fatal("new session has an opening time, explanations, or an incorrect creation time")
	}
	clock.Advance(1500 * time.Millisecond)
	current, err := service.Get(created.ID)
	if err != nil || current.Elapsed != 1500*time.Millisecond || current.Attempts != 0 {
		t.Fatalf("reading session: %+v, %v", current, err)
	}
	for number := 1; number <= 2; number++ {
		checked, err := service.Check(created.ID, failing)
		if err != nil {
			t.Fatal(err)
		}
		if checked.Attempts != number || len(checked.History) != number || checked.Status != Locked || checked.Explanations != nil {
			t.Fatalf("unexpected failed check: %+v", checked)
		}
		last := checked.History[number-1]
		if last.Number != number || last.Switches != failing || last.Elapsed != 1500*time.Millisecond {
			t.Fatalf("incorrect attempt: %+v", last)
		}
	}
	clock.Advance(3500 * time.Millisecond)
	opened, err := service.Check(created.ID, winning)
	if err != nil {
		t.Fatal(err)
	}
	if opened.Status != Opened || opened.Attempts != 3 || opened.Elapsed != 5*time.Second || opened.Explanations == nil {
		t.Fatalf("unexpected opened session: %+v", opened)
	}
	if opened.OpenedAt == nil || !opened.OpenedAt.Equal(created.CreatedAt.Add(5*time.Second)) {
		t.Fatal("opening time was not fixed")
	}
	for index, indicator := range opened.History[2].Indicators {
		if !indicator.Passed || opened.Explanations[index].ID != indicator.ID {
			t.Fatal("solution indicators and explanations do not match")
		}
	}
	clock.Advance(time.Hour)
	if _, err := service.Check(created.ID, failing); !errors.Is(err, ErrAlreadyOpened) {
		t.Fatalf("check after opening: %v", err)
	}
	after, err := service.Get(created.ID)
	if err != nil || !reflect.DeepEqual(after, opened) {
		t.Fatalf("opened session changed: %+v, %v", after, err)
	}
}

func TestSnapshotsAreIndependent(t *testing.T) {
	service, _, winning, failing := testService(t)
	created, err := service.Create()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Check(created.ID, failing); err != nil {
		t.Fatal(err)
	}
	want, err := service.Check(created.ID, winning)
	if err != nil {
		t.Fatal(err)
	}
	copy, err := service.Get(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	copy.History[0].Number = 999
	copy.History[0].Switches[0] = !copy.History[0].Switches[0]
	copy.History[0].Indicators[0].Passed = !copy.History[0].Indicators[0].Passed
	*copy.OpenedAt = time.Time{}
	copy.Explanations[0].Condition = "changed"
	got, err := service.Get(created.ID)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("mutating a snapshot changed the session: %+v, %v", got, err)
	}
}

func TestConcurrentFailedChecks(t *testing.T) {
	service, _, _, failing := testService(t)
	created, err := service.Create()
	if err != nil {
		t.Fatal(err)
	}
	type outcome struct {
		attempts int
		err      error
	}
	const requests = 100
	results := make(chan outcome, requests)
	for range requests {
		go func() {
			checked, err := service.Check(created.ID, failing)
			if err == nil {
				// A response can be modified while other requests are reading the store.
				checked.History[0].Number = -1
			}
			results <- outcome{attempts: checked.Attempts, err: err}
		}()
	}
	seen := make(map[int]bool)
	for range requests {
		result := <-results
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.attempts < 1 || result.attempts > requests || seen[result.attempts] {
			t.Fatalf("duplicate or invalid attempt count: %d", result.attempts)
		}
		seen[result.attempts] = true
	}
	got, err := service.Get(created.ID)
	if err != nil || got.Attempts != requests || len(got.History) != requests {
		t.Fatalf("lost concurrent checks: %+v, %v", got, err)
	}
	for index, attempt := range got.History {
		if attempt.Number != index+1 || attempt.Switches != failing {
			t.Fatalf("incorrect history at %d: %+v", index, attempt)
		}
	}
}

func TestConcurrentOpening(t *testing.T) {
	service, clock, winning, _ := testService(t)
	created, err := service.Create()
	if err != nil {
		t.Fatal(err)
	}
	clock.Advance(time.Second)
	const requests = 100
	results := make(chan error, requests)
	for range requests {
		go func() {
			_, err := service.Check(created.ID, winning)
			results <- err
		}()
	}
	accepted := 0
	for range requests {
		if err := <-results; err == nil {
			accepted++
		} else if !errors.Is(err, ErrAlreadyOpened) {
			t.Fatal(err)
		}
	}
	got, err := service.Get(created.ID)
	if err != nil || accepted != 1 || got.Attempts != 1 || len(got.History) != 1 || got.Status != Opened || got.Elapsed != time.Second {
		t.Fatalf("opening was not atomic: accepted=%d, state=%+v, error=%v", accepted, got, err)
	}
}

func TestMissingSessions(t *testing.T) {
	service, _, _, failing := testService(t)
	if _, err := service.Get("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get: %v", err)
	}
	if _, err := service.Check("missing", failing); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Check: %v", err)
	}
}

func TestCreationFailuresDoNotStoreSessions(t *testing.T) {
	failure := errors.New("creation failed")
	for _, stage := range []string{"id", "vault"} {
		t.Run(stage, func(t *testing.T) {
			store := NewMemoryStore()
			options := Options{NewID: func() (string, error) { return "test-id", nil }}
			if stage == "id" {
				options.NewID = func() (string, error) { return "", failure }
				options.NewVault = func() (vault.Vault, error) {
					t.Fatal("vault generation should not follow a failed ID generation")
					return vault.Vault{}, nil
				}
			} else {
				options.NewVault = func() (vault.Vault, error) { return vault.Vault{}, failure }
			}
			if _, err := NewService(store, options).Create(); !errors.Is(err, failure) {
				t.Fatalf("Create: %v", err)
			}
			if _, err := store.Get("test-id"); !errors.Is(err, ErrNotFound) {
				t.Fatal("failed creation left a session in the store")
			}
		})
	}
}

func TestDefaultServiceCreatesIndependentGames(t *testing.T) {
	service := NewService(NewMemoryStore(), Options{})
	first, err := service.Create()
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Create()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{first.ID, second.ID} {
		decoded, err := hex.DecodeString(id)
		if err != nil || len(decoded) != 16 {
			t.Fatalf("invalid random ID: %q", id)
		}
	}
	if first.ID == second.ID {
		t.Fatal("different games have the same ID")
	}
	opened := false
	for state := range 1 << vault.SwitchCount {
		checked, err := service.Check(first.ID, combination(state))
		if err != nil {
			t.Fatal(err)
		}
		if checked.Status == Opened {
			opened = true
			break
		}
	}
	if !opened {
		t.Fatal("default service generated a vault without a solution")
	}
	untouched, err := service.Get(second.ID)
	if err != nil || untouched.Attempts != 0 || untouched.Status != Locked {
		t.Fatalf("checking one game changed another: %+v, %v", untouched, err)
	}
}

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Advance(duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(duration)
}

func testService(t *testing.T) (*Service, *testClock, vault.Combination, vault.Combination) {
	t.Helper()
	puzzle, err := vault.Generate(42)
	if err != nil {
		t.Fatal(err)
	}
	var winning, failing vault.Combination
	for state := range 1 << vault.SwitchCount {
		candidate := combination(state)
		if puzzle.Check(candidate).Opened {
			winning = candidate
		} else {
			failing = candidate
		}
	}
	clock := &testClock{now: time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)}
	service := NewService(NewMemoryStore(), Options{
		Now: clock.Now, NewVault: func() (vault.Vault, error) { return puzzle, nil },
	})
	return service, clock, winning, failing
}

func combination(state int) vault.Combination {
	var result vault.Combination
	for index := range result {
		result[index] = state&(1<<index) != 0
	}
	return result
}
