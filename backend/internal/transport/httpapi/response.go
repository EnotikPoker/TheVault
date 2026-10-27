package httpapi

import (
	"time"

	"thevault/backend/internal/session"
	"thevault/backend/internal/vault"
)

type indicatorResponse struct {
	ID     vault.SystemID `json:"id"`
	Passed *bool          `json:"passed"`
}

type attemptResponse struct {
	Attempt    int                                  `json:"attempt"`
	Switches   vault.Combination                    `json:"switches"`
	Indicators [vault.SystemCount]indicatorResponse `json:"indicators"`
	ElapsedMs  int64                                `json:"elapsedMs"`
}

type explanationResponse struct {
	ID          vault.SystemID `json:"id"`
	Condition   string         `json:"condition"`
	Observation string         `json:"observation"`
}

type stateResponse struct {
	ID           string                               `json:"id"`
	CreatedAt    time.Time                            `json:"createdAt"`
	OpenedAt     *time.Time                           `json:"openedAt,omitempty"`
	Status       session.Status                       `json:"status"`
	Attempts     int                                  `json:"attempts"`
	ElapsedMs    int64                                `json:"elapsedMs"`
	Switches     *vault.Combination                   `json:"switches"`
	Indicators   [vault.SystemCount]indicatorResponse `json:"indicators"`
	History      []attemptResponse                    `json:"history"`
	Explanations []explanationResponse                `json:"explanations,omitempty"`
}

func response(state session.Snapshot) stateResponse {
	body := stateResponse{
		ID: state.ID, CreatedAt: state.CreatedAt.UTC(), Status: state.Status,
		Attempts: state.Attempts, ElapsedMs: state.Elapsed.Milliseconds(),
		Indicators: [vault.SystemCount]indicatorResponse{
			{ID: vault.SystemPower}, {ID: vault.SystemSync}, {ID: vault.SystemAccess},
		},
		History: make([]attemptResponse, len(state.History)),
	}
	for index, attempt := range state.History {
		body.History[index] = attemptResponse{
			Attempt: attempt.Number, Switches: attempt.Switches,
			Indicators: indicators(attempt.Indicators), ElapsedMs: attempt.Elapsed.Milliseconds(),
		}
	}
	if len(body.History) != 0 {
		last := body.History[len(body.History)-1]
		body.Switches = &last.Switches
		body.Indicators = last.Indicators
	}
	if state.Status == session.Opened {
		if state.OpenedAt != nil {
			openedAt := state.OpenedAt.UTC()
			body.OpenedAt = &openedAt
		}
		if state.Explanations != nil {
			for _, explanation := range state.Explanations {
				body.Explanations = append(body.Explanations, explanationResponse{
					ID: explanation.ID, Condition: explanation.Condition, Observation: explanation.Observation,
				})
			}
		}
	}
	return body
}

func indicators(results [vault.SystemCount]vault.IndicatorResult) [vault.SystemCount]indicatorResponse {
	var indicators [vault.SystemCount]indicatorResponse
	for index, result := range results {
		passed := result.Passed
		indicators[index] = indicatorResponse{ID: result.ID, Passed: &passed}
	}
	return indicators
}
