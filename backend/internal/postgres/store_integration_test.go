//go:build integration

package postgres_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"thevault/backend/internal/blackbox"
	"thevault/backend/internal/blackboxgame"
	"thevault/backend/internal/postgres"
	"thevault/backend/internal/session"
	"thevault/backend/internal/transport/httpapi"
)

func TestPostgresAndHTTP(t *testing.T) {
	ctx := context.Background()
	pool, reopen := database(t)
	if err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := postgres.Migrate(ctx, pool); err != nil {
		t.Fatal("migration is not idempotent:", err)
	}
	var seeds, offset atomic.Int64
	start := time.Date(2026, 10, 28, 20, 59, 59, 0, time.UTC)
	options := blackboxgame.Options{
		Now:      func() time.Time { return start.Add(time.Duration(offset.Load()) * time.Millisecond) },
		NewBoard: func() (blackbox.Board, error) { return blackbox.Generate(seeds.Add(1)) },
	}
	store := postgres.NewStore(pool)
	svc := blackboxgame.NewService(store, options)
	const creations = 16
	games := make(chan blackboxgame.Snapshot, creations)
	errorsCh := make(chan error, creations)
	for range creations {
		go func() {
			game, err := svc.Create(ctx, "daily")
			games <- game
			errorsCh <- err
		}()
	}
	var game blackboxgame.Snapshot
	var sharedBoard blackbox.Board
	for index := range creations {
		game = <-games
		if err := <-errorsCh; err != nil {
			t.Fatal(err)
		}
		stored, err := store.Get(ctx, game.ID)
		if err != nil {
			t.Fatal(err)
		}
		if index == 0 {
			sharedBoard = stored.Board
		} else if stored.Board != sharedBoard {
			t.Fatal("concurrent daily sessions have different layouts")
		}
	}
	var puzzleCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM blackbox_puzzles").Scan(&puzzleCount); err != nil || puzzleCount != 1 {
		t.Fatalf("daily uniqueness: %d, %v", puzzleCount, err)
	}
	server := httptest.NewServer(httpapi.NewHandler(session.NewService(session.NewMemoryStore(), session.Options{}), httpapi.WithBlackBox(svc)))
	defer server.Close()
	client := server.Client()
	client.Timeout = 10 * time.Second
	call := func(method, path string, body any, status int) blackboxgame.Snapshot {
		t.Helper()
		var data []byte
		if body != nil {
			data, _ = json.Marshal(body)
		}
		req, _ := http.NewRequest(method, server.URL+path, bytes.NewReader(data))
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var raw json.RawMessage
		if err := json.NewDecoder(response.Body).Decode(&raw); err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != status {
			t.Fatalf("%s %s: %d %s", method, path, response.StatusCode, raw)
		}
		var state blackboxgame.Snapshot
		if status < 300 {
			if err := json.Unmarshal(raw, &state); err != nil {
				t.Fatal(err)
			}
			if state.Status == "locked" && (bytes.Contains(raw, []byte(`"solution"`)) || bytes.Contains(raw, []byte(`"path"`))) {
				t.Fatal("private layout leaked before opening")
			}
		}
		return state
	}
	path := "/api/v1/blackbox/sessions/" + game.ID
	call("POST", "/api/v1/blackbox/sessions", map[string]any{"mode": "practice", "seed": 42}, 400)
	call("GET", "/api/v1/blackbox/sessions/missing", nil, 404)
	call("PUT", path+"/marks", map[string]any{"marks": []any{map[string]any{"state": "suspected"}}}, 400)
	marked := call("PUT", path+"/marks", map[string]any{"marks": []any{map[string]any{"row": 0, "col": 0, "state": "suspected"}}}, 200)
	if len(marked.Marks) != 1 || marked.Score != 0 {
		t.Fatal("marking spent points or lost the note")
	}
	var entry, reverse blackbox.Port
	for _, port := range blackbox.Ports() {
		trace, _ := sharedBoard.Trace(port)
		if trace.Result.Exit != nil {
			entry, reverse = port, *trace.Result.Exit
			break
		}
	}
	const actions = 24
	for index := range actions {
		go func() {
			port := entry
			if index%2 == 1 {
				port = reverse
			}
			_, err := svc.Scan(ctx, game.ID, port)
			errorsCh <- err
		}()
	}
	for range actions {
		if err := <-errorsCh; err != nil {
			t.Fatal(err)
		}
	}
	state := call("GET", path, nil, 200)
	if state.ScanCount != 1 || state.Score != 1 {
		t.Fatal("reverse or concurrent scans were charged twice")
	}
	wrong := []blackbox.Cell{{Row: 0, Col: 0}, {Row: 0, Col: 1}, {Row: 0, Col: 2}}
	if sharedBoard.Matches(wrong) {
		wrong = []blackbox.Cell{{Row: 5, Col: 0}, {Row: 5, Col: 1}, {Row: 5, Col: 2}}
	}
	requestID := strings.Repeat("a", 32)
	for range actions {
		go func() {
			_, err := svc.Answer(ctx, game.ID, requestID, wrong)
			errorsCh <- err
		}()
	}
	for range actions {
		if err := <-errorsCh; err != nil {
			t.Fatal(err)
		}
	}
	state = call("GET", path, nil, 200)
	if state.WrongAnswers != 1 || state.Score != 6 || len(state.Answers) != 1 {
		t.Fatal("idempotent concurrent answer was charged twice")
	}
	call("POST", path+"/answers", map[string]any{"requestId": requestID, "cells": sharedBoard.Cells()}, 409)
	before, _ := store.Get(ctx, game.ID)
	rollback := errors.New("rollback")
	if _, err := store.Update(ctx, game.ID, func(game *blackboxgame.Session) error {
		game.Marks = nil
		return rollback
	}); !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	after, _ := store.Get(ctx, game.ID)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("failed update was committed")
	}
	offset.Store(5000)
	answer := map[string]any{"requestId": strings.Repeat("b", 32), "cells": sharedBoard.Cells()}
	opened := call("POST", path+"/answers", answer, 200)
	if opened.Status != "opened" || len(opened.Solution) != 3 || len(opened.Scans[0].Path) == 0 || opened.ElapsedMs != 5000 {
		t.Fatalf("opening: %+v", opened)
	}
	call("POST", path+"/answers", answer, 200)
	call("PUT", path+"/marks", map[string]any{"marks": []any{}}, 409)
	pool.Close()
	newPool := reopen()
	newSvc := blackboxgame.NewService(postgres.NewStore(newPool), options)
	offset.Store(3600000)
	restored, err := newSvc.Get(ctx, game.ID)
	if err != nil || !reflect.DeepEqual(restored, opened) {
		t.Fatalf("state lost after backend restart: %+v, %v", restored, err)
	}
	next, err := newSvc.Create(ctx, "daily")
	if err != nil || next.Date != "2026-10-29" || restored.Date != "2026-10-28" {
		t.Fatal("Moscow date rollover or old daily session is incorrect")
	}
}

func database(t *testing.T) (*pgxpool.Pool, func() *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL is required for integration tests")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	schema := "test_blackbox_" + hex.EncodeToString(random[:])
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	open := func() *pgxpool.Pool {
		config, err := pgxpool.ParseConfig(dsn)
		if err != nil {
			t.Fatal(err)
		}
		config.ConnConfig.RuntimeParams["search_path"] = schema
		pool, err := pgxpool.NewWithConfig(ctx, config)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(pool.Close)
		return pool
	}
	return open(), open
}
