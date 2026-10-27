// Package httpapi exposes the HTTP interface of The Vault backend.
package httpapi

import (
	"net/http"

	"thevault/backend/internal/session"
	"thevault/backend/internal/vault"
)

type sessionService interface {
	Create() (session.Snapshot, error)
	Get(string) (session.Snapshot, error)
	Check(string, vault.Combination) (session.Snapshot, error)
}

// NewHandler returns the API router.
func NewHandler(sessions sessionService) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	api := gameAPI{sessions: sessions}
	mux.HandleFunc("POST /api/v1/vaults", api.create)
	mux.HandleFunc("GET /api/v1/vaults/{id}", api.get)
	mux.HandleFunc("POST /api/v1/vaults/{id}/checks", api.check)
	return mux
}
