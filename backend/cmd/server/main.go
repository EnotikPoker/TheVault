package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"thevault/backend/internal/blackboxgame"
	"thevault/backend/internal/postgres"
	"thevault/backend/internal/session"
	"thevault/backend/internal/transport/httpapi"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("DATABASE_URL is required; start PostgreSQL with make db-up")
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return errors.New("invalid DATABASE_URL")
	}
	databaseCtx, cancelDatabase := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelDatabase()
	pool, err := pgxpool.NewWithConfig(databaseCtx, config)
	if err != nil {
		return errors.New("could not initialize PostgreSQL connection")
	}
	defer pool.Close()
	if err := pool.Ping(databaseCtx); err != nil {
		return errors.New("PostgreSQL unavailable; check DATABASE_URL and make db-up")
	}
	if err := postgres.Migrate(databaseCtx, pool); err != nil {
		return err
	}
	blackboxSessions := blackboxgame.NewService(postgres.NewStore(pool), blackboxgame.Options{})
	sessions := session.NewService(session.NewMemoryStore(), session.Options{})
	server := &http.Server{
		Addr:              addr,
		Handler:           httpapi.NewHandler(sessions, httpapi.WithBlackBox(blackboxSessions)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errorsCh := make(chan error, 1)
	go func() {
		slog.Info("HTTP server listening", "addr", addr)
		errorsCh <- server.ListenAndServe()
	}()

	select {
	case err := <-errorsCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		if err := <-errorsCh; !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
