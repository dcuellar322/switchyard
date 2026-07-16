package httpapi

import (
	"encoding/json"
	"net/http"

	"switchyard.dev/switchyard/internal/foundation/correlation"
)

type problemDetails struct {
	Type          string `json:"type"`
	Title         string `json:"title"`
	Status        int    `json:"status"`
	Code          string `json:"code"`
	CorrelationID string `json:"correlationId"`
}

func writeProblem(w http.ResponseWriter, r *http.Request, status int, code, title string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problemDetails{
		Type:          "about:blank",
		Title:         title,
		Status:        status,
		Code:          code,
		CorrelationID: correlation.ID(r.Context()),
	})
}
