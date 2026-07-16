package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	catalog "switchyard.dev/switchyard/internal/catalog/application"
	"switchyard.dev/switchyard/internal/foundation/correlation"
	operations "switchyard.dev/switchyard/internal/operations/application"
	session "switchyard.dev/switchyard/internal/session/application"
)

type problemDetails struct {
	Type          string `json:"type"`
	Title         string `json:"title"`
	Status        int    `json:"status"`
	Detail        string `json:"detail,omitempty"`
	Code          string `json:"code"`
	CorrelationID string `json:"correlationId"`
}

func writeProblem(w http.ResponseWriter, r *http.Request, status int, code, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problemDetails{
		Type:          "about:blank",
		Title:         title,
		Status:        status,
		Detail:        detail,
		Code:          code,
		CorrelationID: correlation.ID(r.Context()),
	})
}

func writeApplicationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, catalog.ErrNotFound):
		writeProblem(w, r, http.StatusNotFound, "CATALOG_NOT_FOUND", "Catalog entity not found", "No project or proposal exists for this identifier.")
	case errors.Is(err, catalog.ErrInvalidProposal):
		writeProblem(w, r, http.StatusUnprocessableEntity, "PROPOSAL_INVALID", "Manifest proposal invalid", err.Error())
	case errors.Is(err, catalog.ErrAlreadyReviewed):
		writeProblem(w, r, http.StatusConflict, "PROPOSAL_REVIEWED", "Manifest proposal already reviewed", "Create a new proposal before making another trust decision.")
	case errors.Is(err, operations.ErrNotFound):
		writeProblem(w, r, http.StatusNotFound, "OPERATION_NOT_FOUND", "Operation not found", "No durable operation exists for this identifier.")
	case errors.Is(err, session.ErrInvalidBootstrap):
		writeProblem(w, r, http.StatusUnauthorized, "BOOTSTRAP_INVALID", "Bootstrap token invalid", "The token is unknown, expired, or already used.")
	case errors.Is(err, session.ErrInvalidSession):
		writeProblem(w, r, http.StatusUnauthorized, "SESSION_INVALID", "Browser session invalid", "Launch the UI again to create a fresh session.")
	case errors.Is(err, session.ErrInvalidCSRF):
		writeProblem(w, r, http.StatusForbidden, "CSRF_INVALID", "CSRF token invalid", "Mutations require the session CSRF token.")
	default:
		writeProblem(w, r, http.StatusInternalServerError, "INTERNAL", "Internal server error", "The request could not be completed.")
	}
}
