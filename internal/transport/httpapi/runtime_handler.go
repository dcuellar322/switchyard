package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"

	observabilityDomain "switchyard.dev/switchyard/internal/observability/domain"
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
	if h.health != nil {
		health, healthErr := h.health.Get(r.Context(), projectID)
		if healthErr == nil && health.Status == "unhealthy" &&
			(observation.State == domain.StateRunning || observation.State == domain.StateRunningExternal || observation.State == domain.StatePartiallyRunning) {
			observation.State = domain.StateDegraded
		} else if healthErr == nil && health.Status == "unknown" && health.ObserverState == "connected" && len(health.Results) > 0 &&
			(observation.State == domain.StateRunning || observation.State == domain.StateRunningExternal) {
			observation.State = domain.StateStarting
		}
	}
	if observation.Services == nil {
		observation.Services = []domain.ServiceObservation{}
	}
	for index := range observation.Services {
		if observation.Services[index].Ports == nil {
			observation.Services[index].Ports = []domain.PublishedPort{}
		}
	}
	writeJSON(w, http.StatusOK, observation)
}

func (h *handler) GetProjectHealth(w http.ResponseWriter, r *http.Request, projectID generated.ProjectId) {
	if h.health == nil {
		writeProblem(w, r, http.StatusServiceUnavailable, "HEALTH_UNAVAILABLE", "Health unavailable", "The health observer is not configured.")
		return
	}
	health, err := h.health.Get(r.Context(), projectID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	if health.Results == nil {
		health.Results = []observabilityDomain.HealthResult{}
	}
	writeJSON(w, http.StatusOK, health)
}

func (h *handler) PlanProjectRuntime(w http.ResponseWriter, r *http.Request, projectID generated.ProjectId) {
	request, ok := decodeRuntimeRequest(w, r)
	if !ok {
		return
	}
	plan, err := h.runtime.PlanServices(r.Context(), projectID, domain.Action(request.Action), request.RemoveVolumes != nil && *request.RemoveVolumes, runtimeServices(request))
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
	services := runtimeServices(request)
	if _, err := h.runtime.PlanServices(r.Context(), projectID, action, removeVolumes, services); err != nil {
		writeApplicationError(w, r, err)
		return
	}
	input, _ := json.Marshal(map[string]any{"action": action, "removeVolumes": removeVolumes, "services": services})
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

func runtimeServices(request generated.RuntimeActionRequest) []string {
	if request.Services == nil {
		return nil
	}
	return *request.Services
}

func (h *handler) GetProjectLogs(
	w http.ResponseWriter,
	r *http.Request,
	projectID generated.ProjectId,
	params generated.GetProjectLogsParams,
) {
	service, since, runID, operationID, tail := "", "", "", "", 200
	if params.Service != nil {
		service = *params.Service
	}
	if params.Since != nil {
		since = *params.Since
	}
	if params.RunId != nil {
		runID = *params.RunId
	}
	if params.OperationId != nil {
		operationID = *params.OperationId
	}
	if params.Tail != nil {
		tail = *params.Tail
	}
	entries, err := h.logs.Logs(r.Context(), projectID, service, since, runID, operationID, tail)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (h *handler) ExportProjectLogs(
	w http.ResponseWriter,
	r *http.Request,
	projectID generated.ProjectId,
	params generated.ExportProjectLogsParams,
) {
	service, runID, operationID := "", "", ""
	if params.Service != nil {
		service = *params.Service
	}
	if params.RunId != nil {
		runID = *params.RunId
	}
	if params.OperationId != nil {
		operationID = *params.OperationId
	}
	format := string(params.Format)
	var output bytes.Buffer
	if err := h.logs.Export(r.Context(), projectID, service, runID, operationID, format, &output); err != nil {
		writeApplicationError(w, r, err)
		return
	}
	if format == "ndjson" {
		w.Header().Set("Content-Type", "application/x-ndjson")
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}
	w.Header().Set("Content-Disposition", `attachment; filename="switchyard-logs.`+format+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(output.Bytes())
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
