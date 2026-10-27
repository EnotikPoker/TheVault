package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"thevault/backend/internal/session"
	"thevault/backend/internal/vault"
)

type gameAPI struct {
	sessions sessionService
}

func (api gameAPI) create(w http.ResponseWriter, r *http.Request) {
	decoder := requestDecoder(w, r)
	var options map[string]json.RawMessage
	err := decoder.Decode(&options)
	if err != io.EOF && (err != nil || options == nil || len(options) != 0 || !endOfJSON(decoder)) {
		writeError(w, http.StatusBadRequest, "invalid_request", "Expected an empty body or an empty JSON object.")
		return
	}
	state, err := api.sessions.Create()
	if err != nil {
		writeServiceError(w, err)
		return
	}
	w.Header().Set("Location", "/api/v1/vaults/"+url.PathEscape(state.ID))
	writeJSON(w, http.StatusCreated, response(state))
}

func (api gameAPI) get(w http.ResponseWriter, r *http.Request) {
	state, err := api.sessions.Get(r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response(state))
}

func (api gameAPI) check(w http.ResponseWriter, r *http.Request) {
	decoder := requestDecoder(w, r)
	var body struct {
		Switches []*bool `json:"switches"`
	}
	if err := decoder.Decode(&body); err != nil || len(body.Switches) != vault.SwitchCount || !endOfJSON(decoder) {
		writeInvalidCombination(w)
		return
	}
	var combination vault.Combination
	for index, enabled := range body.Switches {
		if enabled == nil {
			writeInvalidCombination(w)
			return
		}
		combination[index] = *enabled
	}
	state, err := api.sessions.Check(r.PathValue("id"), combination)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response(state))
}

func requestDecoder(w http.ResponseWriter, r *http.Request) *json.Decoder {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	decoder.DisallowUnknownFields()
	return decoder
}

func endOfJSON(decoder *json.Decoder) bool {
	var extra json.RawMessage
	return decoder.Decode(&extra) == io.EOF
}

func writeInvalidCombination(w http.ResponseWriter) {
	writeError(w, http.StatusBadRequest, "invalid_request", "Expected switches to contain exactly five booleans.")
}

type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, session.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Session not found.")
	case errors.Is(err, session.ErrAlreadyOpened):
		writeError(w, http.StatusConflict, "already_opened", "The vault is already open.")
	default:
		slog.Error("session request failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error.")
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	var body errorResponse
	body.Error.Code = code
	body.Error.Message = message
	writeJSON(w, status, body)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	data, err := json.Marshal(body)
	if err != nil {
		slog.Error("encode API response", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error.")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(append(data, '\n'))
}
