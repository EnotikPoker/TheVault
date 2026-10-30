package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"thevault/backend/internal/blackbox"
	"thevault/backend/internal/blackboxgame"
)

type blackboxService interface {
	Daily() blackboxgame.DailyInfo
	Create(context.Context, string) (blackboxgame.Snapshot, error)
	Get(context.Context, string) (blackboxgame.Snapshot, error)
	Scan(context.Context, string, blackbox.Port) (blackboxgame.Snapshot, error)
	SetMarks(context.Context, string, []blackboxgame.Mark) (blackboxgame.Snapshot, error)
	Answer(context.Context, string, string, []blackbox.Cell) (blackboxgame.Snapshot, error)
}

func WithBlackBox(service blackboxService) func(*http.ServeMux) {
	return func(mux *http.ServeMux) {
		api := blackboxAPI{service: service}
		mux.HandleFunc("GET /api/v1/blackbox/daily", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, service.Daily())
		})
		mux.HandleFunc("POST /api/v1/blackbox/sessions", api.create)
		mux.HandleFunc("GET /api/v1/blackbox/sessions/{id}", api.get)
		mux.HandleFunc("PUT /api/v1/blackbox/sessions/{id}/scans/{side}/{index}", api.scan)
		mux.HandleFunc("PUT /api/v1/blackbox/sessions/{id}/marks", api.marks)
		mux.HandleFunc("POST /api/v1/blackbox/sessions/{id}/answers", api.answer)
	}
}

type blackboxAPI struct{ service blackboxService }

func (api blackboxAPI) create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Mode string `json:"mode"`
	}
	if !decodeRequest(w, r, &body) {
		return
	}
	state, err := api.service.Create(r.Context(), body.Mode)
	if err != nil {
		writeBlackBoxError(w, err)
		return
	}
	w.Header().Set("Location", "/api/v1/blackbox/sessions/"+state.ID)
	writeJSON(w, http.StatusCreated, state)
}

func (api blackboxAPI) get(w http.ResponseWriter, r *http.Request) {
	state, err := api.service.Get(r.Context(), r.PathValue("id"))
	writeBlackBoxState(w, state, err)
}

func (api blackboxAPI) scan(w http.ResponseWriter, r *http.Request) {
	index, err := strconv.Atoi(r.PathValue("index"))
	decoder := requestDecoder(w, r)
	var body map[string]json.RawMessage
	decodeErr := decoder.Decode(&body)
	if err != nil || (decodeErr != io.EOF && (decodeErr != nil || body == nil || len(body) != 0 || !endOfJSON(decoder))) {
		writeBlackBoxError(w, blackboxgame.ErrInvalid)
		return
	}
	state, err := api.service.Scan(r.Context(), r.PathValue("id"), blackbox.Port{Side: r.PathValue("side"), Index: index})
	writeBlackBoxState(w, state, err)
}

type cellRequest struct {
	Row *int `json:"row"`
	Col *int `json:"col"`
}

func (api blackboxAPI) marks(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Marks []struct {
			cellRequest
			State string `json:"state"`
		} `json:"marks"`
	}
	if !decodeRequest(w, r, &body) {
		return
	}
	if body.Marks == nil {
		writeBlackBoxError(w, blackboxgame.ErrInvalid)
		return
	}
	marks := make([]blackboxgame.Mark, 0, len(body.Marks))
	for _, mark := range body.Marks {
		if mark.Row == nil || mark.Col == nil {
			writeBlackBoxError(w, blackboxgame.ErrInvalid)
			return
		}
		marks = append(marks, blackboxgame.Mark{Cell: blackbox.Cell{Row: *mark.Row, Col: *mark.Col}, State: mark.State})
	}
	state, err := api.service.SetMarks(r.Context(), r.PathValue("id"), marks)
	writeBlackBoxState(w, state, err)
}

func (api blackboxAPI) answer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RequestID string        `json:"requestId"`
		Cells     []cellRequest `json:"cells"`
	}
	if !decodeRequest(w, r, &body) {
		return
	}
	cells := make([]blackbox.Cell, 0, len(body.Cells))
	for _, cell := range body.Cells {
		if cell.Row == nil || cell.Col == nil {
			writeBlackBoxError(w, blackboxgame.ErrInvalid)
			return
		}
		cells = append(cells, blackbox.Cell{Row: *cell.Row, Col: *cell.Col})
	}
	state, err := api.service.Answer(r.Context(), r.PathValue("id"), body.RequestID, cells)
	writeBlackBoxState(w, state, err)
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := requestDecoder(w, r)
	if err := decoder.Decode(target); err != nil || !endOfJSON(decoder) {
		writeBlackBoxError(w, blackboxgame.ErrInvalid)
		return false
	}
	return true
}

func writeBlackBoxState(w http.ResponseWriter, state blackboxgame.Snapshot, err error) {
	if err != nil {
		writeBlackBoxError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func writeBlackBoxError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, blackboxgame.ErrInvalid):
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid Black Box request.")
	case errors.Is(err, blackboxgame.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Session not found.")
	case errors.Is(err, blackboxgame.ErrOpened):
		writeError(w, http.StatusConflict, "already_opened", "The vault is already open.")
	case errors.Is(err, blackboxgame.ErrConflict):
		writeError(w, http.StatusConflict, "request_conflict", "Request ID already used with different cells.")
	default:
		slog.Error("Black Box request failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error.")
	}
}
