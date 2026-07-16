package httpapi

import (
	"encoding/json"
	"net/http"

	actionsDomain "switchyard.dev/switchyard/internal/actions/domain"
	operations "switchyard.dev/switchyard/internal/operations/application"
	portsDomain "switchyard.dev/switchyard/internal/ports/domain"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

func (h *handler) GetPortRegistry(w http.ResponseWriter, r *http.Request) {
	registry, err := h.ports.Registry(r.Context())
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	if registry.Facts == nil {
		registry.Facts = []portsDomain.Fact{}
	}
	if registry.Conflicts == nil {
		registry.Conflicts = []portsDomain.Conflict{}
	}
	if registry.Warnings == nil {
		registry.Warnings = []string{}
	}
	writeJSON(w, http.StatusOK, registry)
}

func (h *handler) CreatePortSuggestion(w http.ResponseWriter, r *http.Request, _ generated.CreatePortSuggestionParams) {
	var request generated.PortSuggestionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "REQUEST_INVALID", "Request body invalid", "Provide one preferred port range and protocol.")
		return
	}
	projectID := ""
	if request.ProjectId != nil {
		projectID = *request.ProjectId
	}
	excluded := []int{}
	if request.Excluded != nil {
		excluded = *request.Excluded
	}
	suggestion, err := h.ports.Suggest(r.Context(), request.RangeStart, request.RangeEnd, string(request.Protocol), projectID, excluded)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, suggestion)
}

func (h *handler) GetProjectGit(w http.ResponseWriter, r *http.Request, projectID generated.ProjectId) {
	state, err := h.git.Get(r.Context(), projectID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (h *handler) ListProjectActions(w http.ResponseWriter, r *http.Request, projectID generated.ProjectId) {
	actions, err := h.actions.List(r.Context(), projectID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, actions)
}

func (h *handler) CreateActionOperation(
	w http.ResponseWriter,
	r *http.Request,
	projectID generated.ProjectId,
	actionID generated.ActionId,
	params generated.CreateActionOperationParams,
) {
	var request generated.ActionExecutionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "REQUEST_INVALID", "Request body invalid", "Provide explicit action authorization flags.")
		return
	}
	definitions, err := h.actions.List(r.Context(), projectID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	definition, found := actionDefinition(definitions.Actions, actionID)
	if !found {
		writeProblem(w, r, http.StatusNotFound, "ACTION_NOT_FOUND", "Action not found", "No trusted project action has this identifier.")
		return
	}
	confirm := request.ConfirmRisk != nil && *request.ConfirmRisk
	if definition.Risk == actionsDomain.RiskDestructive && !confirm {
		writeProblem(w, r, http.StatusConflict, "ACTION_CONFIRMATION_REQUIRED", "Action confirmation required", "Destructive actions require explicit confirmation.")
		return
	}
	allowOutside := request.AllowOutsideRoot != nil && *request.AllowOutsideRoot
	identity := identityFrom(r.Context())
	input, _ := json.Marshal(map[string]any{
		"actionId": actionID, "confirmRisk": confirm, "allowOutsideRoot": allowOutside,
		"actorType": string(identity.Access), "actorId": identity.ActorID,
	})
	operation, err := h.operations.Submit(r.Context(), operations.SubmitRequest{
		ProjectID: projectID, Kind: "action.run", Input: input, IdempotencyKey: params.IdempotencyKey,
		ActorType: string(identity.Access), ActorID: identity.ActorID,
	})
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, operationResponse(operation))
}

func actionDefinition(actions []actionsDomain.Definition, id string) (actionsDomain.Definition, bool) {
	for _, action := range actions {
		if action.ID == id {
			return action, true
		}
	}
	return actionsDomain.Definition{}, false
}
