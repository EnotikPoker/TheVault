package blackboxgame

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"thevault/backend/internal/blackbox"
)

func TestGameLifecycle(t *testing.T) {
	svc, _, clock, board := fixture(t)
	ctx := context.Background()
	created, err := svc.Create(ctx, "practice")
	if err != nil || created.Status != "locked" || created.Score != 0 || created.Marks == nil || created.Scans == nil {
		t.Fatalf("new game: %+v, %v", created, err)
	}
	var entry blackbox.Port
	var reverse blackbox.Port
	for _, port := range blackbox.Ports() {
		trace, _ := board.Trace(port)
		if trace.Result.Exit != nil {
			entry, reverse = port, *trace.Result.Exit
			break
		}
	}
	clock.Add(2000)
	scanned, err := svc.Scan(ctx, created.ID, entry)
	if err != nil || scanned.ScanCount != 1 || scanned.Score != 1 || len(scanned.Scans[0].Path) != 0 || scanned.Solution != nil {
		t.Fatalf("scan: %+v, %v", scanned, err)
	}
	for _, port := range []blackbox.Port{entry, reverse} {
		repeated, err := svc.Scan(ctx, created.ID, port)
		if err != nil || !reflect.DeepEqual(repeated, scanned) {
			t.Fatalf("known scan was charged twice: %+v, %v", repeated, err)
		}
	}
	marks := []Mark{{Cell: blackbox.Cell{Row: 0, Col: 0}, State: "suspected"}}
	marked, err := svc.SetMarks(ctx, created.ID, marks)
	if err != nil || marked.Score != 1 || !reflect.DeepEqual(marked.Marks, marks) {
		t.Fatalf("marks: %+v, %v", marked, err)
	}
	wrong := []blackbox.Cell{{Row: 0, Col: 0}, {Row: 0, Col: 1}, {Row: 0, Col: 2}}
	wrongID := strings.Repeat("a", 32)
	failed, err := svc.Answer(ctx, created.ID, wrongID, wrong)
	if err != nil || failed.WrongAnswers != 1 || failed.Score != 6 || failed.Status != "locked" {
		t.Fatalf("wrong answer: %+v, %v", failed, err)
	}
	assertPrivate(t, failed)
	repeated, err := svc.Answer(ctx, created.ID, wrongID, []blackbox.Cell{wrong[2], wrong[0], wrong[1]})
	if err != nil || repeated.Score != 6 || len(repeated.Answers) != 1 {
		t.Fatal("idempotent retry charged another penalty")
	}
	if _, err := svc.Answer(ctx, created.ID, wrongID, board.Cells()); !errors.Is(err, ErrConflict) {
		t.Fatalf("reused request ID: %v", err)
	}
	clock.Add(3000)
	opened, err := svc.Answer(ctx, created.ID, strings.Repeat("b", 32), board.Cells())
	if err != nil || opened.Status != "opened" || opened.Score != 6 || opened.ElapsedMs != 5000 || !reflect.DeepEqual(opened.Solution, board.Cells()) || len(opened.Scans[0].Path) == 0 {
		t.Fatalf("opened: %+v, %v", opened, err)
	}
	clock.Add(3600000)
	after, err := svc.Get(ctx, created.ID)
	if err != nil || !reflect.DeepEqual(after, opened) {
		t.Fatal("opened result did not stay fixed")
	}
	if _, err := svc.SetMarks(ctx, created.ID, []Mark{}); !errors.Is(err, ErrOpened) {
		t.Fatalf("marks after opening: %v", err)
	}
	if _, err := svc.Answer(ctx, created.ID, strings.Repeat("c", 32), wrong); !errors.Is(err, ErrOpened) {
		t.Fatalf("new answer after opening: %v", err)
	}
	if _, err := svc.Answer(ctx, created.ID, strings.Repeat("b", 32), board.Cells()); err != nil {
		t.Fatalf("successful answer retry: %v", err)
	}
	for _, port := range blackbox.Ports() {
		if port != entry && port != reverse {
			if _, err := svc.Scan(ctx, created.ID, port); !errors.Is(err, ErrOpened) {
				t.Fatalf("new scan after opening: %v", err)
			}
			break
		}
	}
}

func TestMoscowDateAndConcurrentDaily(t *testing.T) {
	svc, repo, clock, _ := fixture(t)
	clock.Store(time.Date(2026, 10, 28, 20, 59, 59, 0, time.UTC).UnixMilli())
	info := svc.Daily()
	if info.Date != "2026-10-28" || !info.ResetsAt.Equal(time.Date(2026, 10, 28, 21, 0, 0, 0, time.UTC)) {
		t.Fatalf("Moscow boundary: %+v", info)
	}
	const count = 32
	results := make(chan Snapshot, count)
	errorsCh := make(chan error, count)
	for range count {
		go func() {
			state, err := svc.Create(context.Background(), "daily")
			results <- state
			errorsCh <- err
		}()
	}
	var oldID string
	for range count {
		state := <-results
		if err := <-errorsCh; err != nil || state.Date != info.Date {
			t.Fatalf("daily creation: %+v %v", state, err)
		}
		oldID = state.ID
	}
	if len(repo.daily) != 1 {
		t.Fatal("multiple puzzles created for one day")
	}
	clock.Add(2000)
	if svc.Daily().Date != "2026-10-29" {
		t.Fatal("day did not advance at Moscow midnight")
	}
	old, err := svc.Get(context.Background(), oldID)
	if err != nil || old.Date != "2026-10-28" {
		t.Fatal("old daily game expired at midnight")
	}
}

func TestConcurrentActionsAreIdempotent(t *testing.T) {
	svc, _, _, _ := fixture(t)
	ctx := context.Background()
	game, err := svc.Create(ctx, "practice")
	if err != nil {
		t.Fatal(err)
	}
	const count = 50
	errorsCh := make(chan error, count)
	for range count {
		go func() {
			_, err := svc.Scan(ctx, game.ID, blackbox.Port{Side: "top", Index: 0})
			errorsCh <- err
		}()
	}
	for range count {
		if err := <-errorsCh; err != nil {
			t.Fatal(err)
		}
	}
	for range count {
		go func() {
			_, err := svc.Answer(ctx, game.ID, strings.Repeat("a", 32), []blackbox.Cell{{Row: 0, Col: 0}, {Row: 0, Col: 1}, {Row: 0, Col: 2}})
			errorsCh <- err
		}()
	}
	for range count {
		if err := <-errorsCh; err != nil {
			t.Fatal(err)
		}
	}
	state, _ := svc.Get(ctx, game.ID)
	if state.ScanCount != 1 || state.WrongAnswers != 1 || state.Score != 6 {
		t.Fatalf("duplicate charges: %+v", state)
	}
}

func TestInvalidActionsAndSnapshotIsolation(t *testing.T) {
	svc, _, _, _ := fixture(t)
	ctx := context.Background()
	if _, err := svc.Create(ctx, "invalid"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	game, _ := svc.Create(ctx, "practice")
	for _, marks := range [][]Mark{nil, {{Cell: blackbox.Cell{Row: -1, Col: 0}, State: "empty"}}, {{Cell: blackbox.Cell{Row: 0, Col: 0}, State: "wrong"}}, {{Cell: blackbox.Cell{Row: 0, Col: 0}, State: "empty"}, {Cell: blackbox.Cell{Row: 0, Col: 0}, State: "suspected"}}} {
		if _, err := svc.SetMarks(ctx, game.ID, marks); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid marks: %v", err)
		}
	}
	if _, err := svc.Answer(ctx, game.ID, "bad", []blackbox.Cell{{Row: 0, Col: 0}, {Row: 0, Col: 1}, {Row: 0, Col: 2}}); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := svc.Scan(ctx, game.ID, blackbox.Port{Side: "top", Index: 99}); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := svc.Get(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	marked, _ := svc.SetMarks(ctx, game.ID, []Mark{{Cell: blackbox.Cell{Row: 0, Col: 0}, State: "suspected"}})
	marked.Marks[0].State = "changed"
	again, _ := svc.Get(ctx, game.ID)
	if again.Marks[0].State != "suspected" || again.Score != 0 {
		t.Fatal("snapshot mutation or invalid requests changed the game")
	}
}

func assertPrivate(t *testing.T, state Snapshot) {
	t.Helper()
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"board"`, `"layout"`, `"seed"`, `"solution"`, `"path"`} {
		if strings.Contains(string(data), field) {
			t.Fatalf("private data leaked: %s", data)
		}
	}
}

type memoryRepo struct {
	mu    sync.Mutex
	games map[string]Session
	daily map[string]blackbox.Board
}

func copyGame(game Session) Session {
	data, _ := json.Marshal(game)
	var copy Session
	if err := json.Unmarshal(data, &copy); err != nil {
		panic(err)
	}
	copy.Board = game.Board
	return copy
}

func (r *memoryRepo) Create(_ context.Context, game Session) (Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if game.Mode == "daily" {
		if board, ok := r.daily[game.Date]; ok {
			game.Board = board
		} else {
			r.daily[game.Date] = game.Board
		}
	}
	r.games[game.ID] = copyGame(game)
	return copyGame(game), nil
}

func (r *memoryRepo) Get(_ context.Context, id string) (Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	game, ok := r.games[id]
	if !ok {
		return game, ErrNotFound
	}
	return copyGame(game), nil
}

func (r *memoryRepo) Update(_ context.Context, id string, update func(*Session) error) (Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	game, ok := r.games[id]
	if !ok {
		return game, ErrNotFound
	}
	working := copyGame(game)
	if err := update(&working); err != nil {
		return Session{}, err
	}
	r.games[id] = copyGame(working)
	return working, nil
}

func fixture(t *testing.T) (*Service, *memoryRepo, *atomic.Int64, blackbox.Board) {
	t.Helper()
	board, err := blackbox.NewBoard([]blackbox.Cell{{Row: 1, Col: 2}, {Row: 3, Col: 4}, {Row: 5, Col: 0}})
	if err != nil {
		t.Fatal(err)
	}
	repo := &memoryRepo{games: make(map[string]Session), daily: make(map[string]blackbox.Board)}
	clock := &atomic.Int64{}
	var ids atomic.Int64
	svc := NewService(repo, Options{
		Now:      func() time.Time { return time.UnixMilli(clock.Load()).UTC() },
		NewID:    func() (string, error) { return fmt.Sprintf("%032x", ids.Add(1)), nil },
		NewBoard: func() (blackbox.Board, error) { return board, nil },
	})
	return svc, repo, clock, board
}
