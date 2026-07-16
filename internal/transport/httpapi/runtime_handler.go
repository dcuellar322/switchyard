package httpapi

import (
	"encoding/json"
	"net/http"

	operations "switchyard.dev/switchyard/internal/operations/application"
	"switchyard.dev/switchyard/internal/runtime/domain"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

func (h *handler) GetProjectRuntime(w http.ResponseWriter, r *http.Request, projectID generated.ProjectId) {
	observation, err := h.runtime.Inspect(r.Context(), projectID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, observation)
}

func (h *handler) PlanProjectRuntime(w http.ResponseWriter, r *http.Request, projectID generated.ProjectId) {
	request, ok := decodeRuntimeRequest(w, r)
	if !ok {
		return
	}
	plan, err := h.runtime.Plan(r.Context(), projectID, domain.Action(request.Action), request.RemoveVolumes != nil && *request.RemoveVolumes)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (h *handler) CreateProjectOperation(
	w http.ResponseWriter,
	r *http.Request,
	projectID generated.ProjectId,
	params generated.CreateProjectOperationParams,
) {
	request, ok := decodeRuntimeRequest(w, r)
	if !ok {
		return
	}
	action, err := domain.ParseAction(string(request.Action))
	if err != nil {
		writeProblem(w, r, http.StatusBadRequest, "RUNTIME_ACTION_INVALID", "Runtime action invalid", err.Error())
		return
	}
	removeVolumes := request.RemoveVolumes != nil && *request.RemoveVolumes
	if _, err := h.runtime.Plan(r.Context(), projectID, action, removeVolumes); err != nil {
		writeApplicationError(w, r, err)
		return
	}
	input, _ := json.Marshal(map[string]any{"action": action, "removeVolumes": removeVolumes})
	identity := identityFrom(r.Context())
	operation, err := h.operations.Submit(r.Context(), operations.SubmitRequest{
		ProjectID: projectID, Kind: "runtime." + string(action), Input: input,
		IdempotencyKey: params.IdempotencyKey, ActorType: string(identity.Access), ActorID: identity.ActorID,
	})
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, operationResponse(operation))
}

func (h *handler) GetProjectLogs(
	w http.ResponseWriter,
	r *http.Request,
	projectID generated.ProjectId,
	params generated.GetProjectLogsParams,
) {
	service, since, tail := "", "", 200
	if params.Service != nil {
		service = *params.Service
	}
	if params.Since != nil {
		since = *params.Since
	}
	if params.Tail != nil {
		tail = *params.Tail
	}
	entries, err := h.runtime.Logs(r.Context(), projectID, service, since, tail)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (h *handler) GetProjectMetrics(
	w http.ResponseWriter,
	r *http.Request,
	projectID generated.ProjectId,
	params generated.GetProjectMetricsParams,
) {
	service := ""
	if params.Service != nil {
		service = *params.Service
	}
	samples, err := h.runtime.Metrics(r.Context(), projectID, service)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, samples)
}

func decodeRuntimeRequest(w http.ResponseWriter, r *http.Request) (generated.RuntimeActionRequest, bool) {
	var request generated.RuntimeActionRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "REQUEST_INVALID", "Request body invalid", "Provide one supported runtime action and optional volume-removal flag.")
		return generated.RuntimeActionRequest{}, false
	}
	if _, err := domain.ParseAction(string(request.Action)); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "RUNTIME_ACTION_INVALID", "Runtime action invalid", err.Error())
		return generated.RuntimeActionRequest{}, false
	}
	return request, true
}
