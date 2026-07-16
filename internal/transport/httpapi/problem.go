package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	actions "switchyard.dev/switchyard/internal/actions/application"
	agents "switchyard.dev/switchyard/internal/agents/application"
	catalog "switchyard.dev/switchyard/internal/catalog/application"
	"switchyard.dev/switchyard/internal/foundation/correlation"
	resources "switchyard.dev/switchyard/internal/observability/application"
	operations "switchyard.dev/switchyard/internal/operations/application"
	ports "switchyard.dev/switchyard/internal/ports/application"
	runtime "switchyard.dev/switchyard/internal/runtime/application"
	runtimeDomain "switchyard.dev/switchyard/internal/runtime/domain"
	session "switchyard.dev/switchyard/internal/session/application"
	sourcecontrol "switchyard.dev/switchyard/internal/sourcecontrol/application"
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
	if writeSpecializedError(w, r, err) {
		return
	}
	switch {
	case errors.Is(err, catalog.ErrNotFound):
		writeProblem(w, r, http.StatusNotFound, "CATALOG_NOT_FOUND", "Catalog entity not found", "No project or proposal exists for this identifier.")
	case errors.Is(err, catalog.ErrInvalidProposal):
		writeProblem(w, r, http.StatusUnprocessableEntity, "PROPOSAL_INVALID", "Manifest proposal invalid", err.Error())
	case errors.Is(err, catalog.ErrAlreadyReviewed):
		writeProblem(w, r, http.StatusConflict, "PROPOSAL_REVIEWED", "Manifest proposal already reviewed", "Create a new proposal before making another trust decision.")
	case errors.Is(err, catalog.ErrHumanApprovalRequired):
		writeProblem(w, r, http.StatusForbidden, "HUMAN_APPROVAL_REQUIRED", "Human approval required", "Assisted manifest proposals cannot be accepted by an agent identity.")
	case errors.Is(err, operations.ErrInvalidRequest):
		writeProblem(w, r, http.StatusBadRequest, "REQUEST_INVALID", "Request invalid", "One or more request parameters are outside their supported range.")
	case errors.Is(err, operations.ErrNotFound):
		writeProblem(w, r, http.StatusNotFound, "OPERATION_NOT_FOUND", "Operation not found", "No durable operation exists for this identifier.")
	case errors.Is(err, actions.ErrActionNotFound):
		writeProblem(w, r, http.StatusNotFound, "ACTION_NOT_FOUND", "Action not found", "No trusted project action has this identifier.")
	case errors.Is(err, actions.ErrConfirmationRequired):
		writeProblem(w, r, http.StatusConflict, "ACTION_CONFIRMATION_REQUIRED", "Action confirmation required", "Destructive actions require explicit confirmation.")
	case errors.Is(err, actions.ErrWorkingDirEscape):
		writeProblem(w, r, http.StatusForbidden, "ACTION_PATH_DENIED", "Action path denied", "The working directory escapes the trusted project root without explicit permission.")
	case errors.Is(err, actions.ErrProjectUntrusted):
		writeProblem(w, r, http.StatusForbidden, "PROJECT_UNTRUSTED", "Project is not trusted", "Approve the project manifest before using project actions.")
	case errors.Is(err, ports.ErrNoPortAvailable):
		writeProblem(w, r, http.StatusConflict, "PORT_RANGE_EXHAUSTED", "Port range exhausted", "No free port remains in the requested range.")
	case errors.Is(err, sourcecontrol.ErrProjectUntrusted):
		writeProblem(w, r, http.StatusForbidden, "PROJECT_UNTRUSTED", "Project is not trusted", "Approve the project manifest before reading repository state.")
	case errors.Is(err, runtime.ErrProjectUntrusted):
		writeProblem(w, r, http.StatusForbidden, "PROJECT_UNTRUSTED", "Project is not trusted", "Approve the project manifest before using runtime capabilities.")
	case errors.Is(err, runtimeDomain.ErrUnsupportedDriver):
		writeProblem(w, r, http.StatusUnprocessableEntity, "RUNTIME_UNSUPPORTED", "Runtime driver unsupported", err.Error())
	case errors.Is(err, runtimeDomain.ErrRuntimeUnavailable):
		writeProblem(w, r, http.StatusServiceUnavailable, "DOCKER_ENGINE_UNAVAILABLE", "Docker Engine unavailable", err.Error())
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

func writeSpecializedError(w http.ResponseWriter, r *http.Request, err error) bool {
	return writeAgentError(w, r, err) || writeResourceError(w, r, err)
}

func writeResourceError(w http.ResponseWriter, r *http.Request, err error) bool {
	if !errors.Is(err, resources.ErrInvalidResourceQuery) {
		return false
	}
	writeProblem(w, r, http.StatusBadRequest, "RESOURCE_QUERY_INVALID", "Resource query invalid", "Use a retained time range, supported resolution, and no more than 1000 points.")
	return true
}

func writeAgentError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, agents.ErrProviderUnavailable):
		writeProblem(w, r, http.StatusServiceUnavailable, "AI_PROVIDER_UNAVAILABLE", "AI provider unavailable", err.Error())
	case errors.Is(err, agents.ErrProviderOutput):
		writeProblem(w, r, http.StatusUnprocessableEntity, "AI_PROVIDER_OUTPUT_REJECTED", "AI provider output rejected", err.Error())
	case errors.Is(err, agents.ErrRunNotFound):
		writeProblem(w, r, http.StatusNotFound, "AI_RUN_NOT_FOUND", "Assisted onboarding run not found", "No assisted onboarding receipt exists for this operation.")
	case errors.Is(err, agents.ErrInvalidLimits):
		writeProblem(w, r, http.StatusBadRequest, "AI_LIMITS_INVALID", "AI generation limits invalid", "Use supported byte, timeout, turn, token, and cost ceilings.")
	default:
		return false
	}
	return true
}
