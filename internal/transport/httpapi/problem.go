package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	actions "switchyard.dev/switchyard/internal/actions/application"
	agents "switchyard.dev/switchyard/internal/agents/application"
	catalog "switchyard.dev/switchyard/internal/catalog/application"
	diagnostics "switchyard.dev/switchyard/internal/diagnostics/application"
	environments "switchyard.dev/switchyard/internal/environments/application"
	"switchyard.dev/switchyard/internal/foundation/correlation"
	resources "switchyard.dev/switchyard/internal/observability/application"
	operations "switchyard.dev/switchyard/internal/operations/application"
	plugins "switchyard.dev/switchyard/internal/plugins/application"
	ports "switchyard.dev/switchyard/internal/ports/application"
	runtime "switchyard.dev/switchyard/internal/runtime/application"
	runtimeDomain "switchyard.dev/switchyard/internal/runtime/domain"
	session "switchyard.dev/switchyard/internal/session/application"
	sourcecontrol "switchyard.dev/switchyard/internal/sourcecontrol/application"
	terminalAdapters "switchyard.dev/switchyard/internal/terminal/adapters"
	terminalApplication "switchyard.dev/switchyard/internal/terminal/application"
	workspace "switchyard.dev/switchyard/internal/workspace/application"
	workspaceDomain "switchyard.dev/switchyard/internal/workspace/domain"
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
	return writeDiagnosticError(w, r, err) || writeTerminalError(w, r, err) || writeAgentError(w, r, err) || writePluginError(w, r, err) || writeResourceError(w, r, err) || writeWorkspaceError(w, r, err) || writeEnvironmentError(w, r, err)
}

func writeDiagnosticError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, diagnostics.ErrDiagnosisNotFound):
		writeProblem(w, r, http.StatusNotFound, "DIAGNOSIS_NOT_FOUND", "Diagnosis not found", "Run a fresh project diagnosis or verify the local identifier.")
	case errors.Is(err, diagnostics.ErrInvalidFeedback):
		writeProblem(w, r, http.StatusUnprocessableEntity, "DIAGNOSTIC_FEEDBACK_INVALID", "Diagnostic feedback invalid", "Review an existing hypothesis with accurate or false_positive.")
	case errors.Is(err, diagnostics.ErrActionNotSuggested):
		writeProblem(w, r, http.StatusForbidden, "DIAGNOSTIC_ACTION_DENIED", "Diagnostic action denied", "Only an existing approved action cited by this diagnosis can run.")
	case errors.Is(err, diagnostics.ErrInvalidRecipe):
		writeProblem(w, r, http.StatusUnprocessableEntity, "AUTOMATION_RECIPE_INVALID", "Automation recipe invalid", err.Error())
	case errors.Is(err, diagnostics.ErrRecipeNotFound):
		writeProblem(w, r, http.StatusNotFound, "AUTOMATION_RECIPE_NOT_FOUND", "Automation recipe not found", "No saved recipe exists for this identifier.")
	default:
		return false
	}
	return true
}

func writePluginError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, plugins.ErrNotFound):
		writeProblem(w, r, http.StatusNotFound, "PLUGIN_NOT_FOUND", "Plugin not found", "Refresh discovery and verify the plugin package is installed.")
	case errors.Is(err, plugins.ErrTrustRequired):
		writeProblem(w, r, http.StatusConflict, "PLUGIN_TRUST_REQUIRED", "Plugin trust required", "Review the package fingerprint, capabilities, and requested scopes before trusting it.")
	case errors.Is(err, plugins.ErrFingerprint):
		writeProblem(w, r, http.StatusConflict, "PLUGIN_IDENTITY_CHANGED", "Plugin identity changed", "Refresh discovery and review the current executable fingerprint before enabling it.")
	case errors.Is(err, plugins.ErrDisabled):
		writeProblem(w, r, http.StatusConflict, "PLUGIN_DISABLED", "Plugin is disabled", "Enable the plugin with an explicit subset of its requested scopes.")
	case errors.Is(err, plugins.ErrPermissionDenied):
		writeProblem(w, r, http.StatusForbidden, "PLUGIN_SCOPE_DENIED", "Plugin scope denied", err.Error())
	case errors.Is(err, plugins.ErrInvocation):
		writeProblem(w, r, http.StatusBadGateway, "PLUGIN_UNAVAILABLE", "Plugin process unavailable", err.Error())
	case errors.Is(err, plugins.ErrDiscovery):
		writeProblem(w, r, http.StatusUnprocessableEntity, "PLUGIN_DISCOVERY_INVALID", "Plugin package invalid", err.Error())
	default:
		return false
	}
	return true
}

func writeTerminalError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, terminalApplication.ErrNotFound):
		writeProblem(w, r, http.StatusNotFound, "TERMINAL_SESSION_NOT_FOUND", "Terminal session not found", "No terminal session exists for this identifier.")
	case errors.Is(err, terminalApplication.ErrOwnerMismatch):
		writeProblem(w, r, http.StatusForbidden, "TERMINAL_SESSION_FORBIDDEN", "Terminal session access denied", "The terminal session belongs to another authenticated local actor.")
	case errors.Is(err, terminalApplication.ErrNotActive):
		writeProblem(w, r, http.StatusConflict, "TERMINAL_SESSION_INACTIVE", "Terminal session is not active", "Only a live daemon-owned session can be attached or terminated.")
	case errors.Is(err, terminalApplication.ErrLaunchInvalid), errors.Is(err, terminalAdapters.ErrUnsupportedTarget), errors.Is(err, terminalAdapters.ErrInteractiveActionRequired):
		writeProblem(w, r, http.StatusUnprocessableEntity, "TERMINAL_LAUNCH_INVALID", "Terminal launch rejected", err.Error())
	default:
		return false
	}
	return true
}

func writeEnvironmentError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, environments.ErrNotFound):
		writeProblem(w, r, http.StatusNotFound, "ENVIRONMENT_NOT_FOUND", "Environment not found", "No registered project environment exists for this identifier.")
	case errors.Is(err, environments.ErrProjectUntrusted):
		writeProblem(w, r, http.StatusForbidden, "PROJECT_UNTRUSTED", "Project is not trusted", "Approve the project manifest before registering worktrees.")
	case errors.Is(err, environments.ErrNoWorktrees):
		writeProblem(w, r, http.StatusUnprocessableEntity, "WORKTREES_UNAVAILABLE", "No Git worktrees found", "The trusted repository did not return a worktree inventory.")
	case errors.Is(err, environments.ErrRuntimeConflict):
		writeProblem(w, r, http.StatusConflict, "ENVIRONMENT_RUNTIME_CONFLICT", "Environment runtime conflict", err.Error())
	default:
		return false
	}
	return true
}

func writeWorkspaceError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, workspace.ErrNotFound):
		writeProblem(w, r, http.StatusNotFound, "WORKSPACE_NOT_FOUND", "Workspace not found", "No workspace exists for this identifier.")
	case errors.Is(err, workspace.ErrRevisionConflict):
		writeProblem(w, r, http.StatusConflict, "WORKSPACE_REVISION_CONFLICT", "Workspace changed", "Reload the workspace graph before applying another edit.")
	case errors.Is(err, workspace.ErrInvalidRequest), errors.Is(err, workspaceDomain.ErrInvalidWorkspace), errors.Is(err, workspaceDomain.ErrDependencyCycle), errors.Is(err, workspaceDomain.ErrUnknownProfile):
		writeProblem(w, r, http.StatusUnprocessableEntity, "WORKSPACE_INVALID", "Workspace invalid", err.Error())
	default:
		return false
	}
	return true
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
