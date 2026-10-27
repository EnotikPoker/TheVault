package session

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"thevault/backend/internal/vault"
)

func TestMemoryStoreCopiesAndRollback(t *testing.T) {
	var store MemoryStore
	openedAt := time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)
	explanations := [vault.SystemCount]vault.Explanation{{ID: vault.SystemPower, Condition: "original"}}
	input := Session{
		ID: "test", OpenedAt: &openedAt, Explanations: &explanations,
		History: []Attempt{{Number: 1}},
	}
	want := clone(input)
	if err := store.Create(input); err != nil {
		t.Fatal(err)
	}
	input.History[0].Number = 999
	*input.OpenedAt = time.Time{}
	input.Explanations[0].Condition = "changed"
	stored, err := store.Get(input.ID)
	if err != nil || !reflect.DeepEqual(stored, want) {
		t.Fatalf("Create retained caller-owned data: %+v, %v", stored, err)
	}
	failure := errors.New("update failed")
	if _, err := store.Update(input.ID, func(game *Session) error {
		game.History[0].Number = 2
		*game.OpenedAt = time.Time{}
		game.Explanations[0].Condition = "failed update"
		return failure
	}); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	stored, err = store.Get(input.ID)
	if err != nil || !reflect.DeepEqual(stored, want) {
		t.Fatal("failed update changed the stored game")
	}
	var retained *Session
	updated, err := store.Update(input.ID, func(game *Session) error {
		game.History[0].Number = 3
		retained = game
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	updated.History[0].Number = 4
	retained.Explanations[0].Condition = "retained callback data"
	stored, err = store.Get(input.ID)
	if err != nil || stored.History[0].Number != 3 || stored.Explanations[0].Condition != "original" {
		t.Fatal("Update retained caller-owned data")
	}
	stored.History[0].Number = 5
	again, err := store.Get(input.ID)
	if err != nil || again.History[0].Number != 3 {
		t.Fatal("Get returned live storage data")
	}
}

func TestMemoryStoreDoesNotOverwriteExistingSession(t *testing.T) {
	store := NewMemoryStore()
	if err := store.Create(Session{ID: "test", History: []Attempt{{Number: 1}}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(Session{ID: "test"}); !errors.Is(err, ErrIDConflict) {
		t.Fatalf("duplicate creation: %v", err)
	}
	got, err := store.Get("test")
	if err != nil || len(got.History) != 1 {
		t.Fatal("duplicate creation overwrote an existing session")
	}
	if _, err := store.Update("missing", func(*Session) error {
		t.Fatal("callback ran for a missing session")
		return nil
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing update: %v", err)
	}
}
