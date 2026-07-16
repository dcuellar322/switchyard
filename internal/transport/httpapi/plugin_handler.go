package httpapi

import (
	"encoding/json"
	"io"
	"net/http"

	operations "switchyard.dev/switchyard/internal/operations/application"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

func (h *handler) ListPlugins(w http.ResponseWriter, r *http.Request) {
	items, err := h.plugins.List(r.Context())
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *handler) RefreshPlugins(w http.ResponseWriter, r *http.Request) {
	items, err := h.plugins.Refresh(r.Context())
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *handler) TrustPlugin(w http.ResponseWriter, r *http.Request, pluginID generated.PluginId) {
	var request generated.PluginTrustRequest
	if err := decodePluginBody(w, r, &request, 4<<10); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "REQUEST_INVALID", "Request body invalid", "Provide the exact reviewed 64-character plugin fingerprint.")
		return
	}
	item, err := h.plugins.Trust(r.Context(), pluginID, request.Fingerprint)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *handler) EnablePlugin(w http.ResponseWriter, r *http.Request, pluginID generated.PluginId) {
	var request generated.PluginEnableRequest
	if err := decodePluginBody(w, r, &request, 8<<10); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "REQUEST_INVALID", "Request body invalid", "Provide only reviewed scopes requested by this plugin.")
		return
	}
	scopes := make([]string, len(request.GrantedScopes))
	for index, scope := range request.GrantedScopes {
		scopes[index] = string(scope)
	}
	item, err := h.plugins.Enable(r.Context(), pluginID, scopes)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *handler) DisablePlugin(w http.ResponseWriter, r *http.Request, pluginID generated.PluginId) {
	item, err := h.plugins.Disable(r.Context(), pluginID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *handler) CheckPluginHealth(w http.ResponseWriter, r *http.Request, pluginID generated.PluginId) {
	item, err := h.plugins.Health(r.Context(), pluginID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *handler) ListPluginLogs(w http.ResponseWriter, r *http.Request, pluginID generated.PluginId, params generated.ListPluginLogsParams) {
	limit := 100
	if params.Limit != nil {
		limit = *params.Limit
	}
	logs, err := h.plugins.Logs(r.Context(), pluginID, limit)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

func (h *handler) InspectProjectWithPlugin(w http.ResponseWriter, r *http.Request, pluginID generated.PluginId, projectID generated.ProjectId) {
	result, err := h.plugins.Inspect(r.Context(), pluginID, projectID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) CreatePluginOperation(w http.ResponseWriter, r *http.Request, pluginID generated.PluginId, projectID generated.ProjectId, params generated.CreatePluginOperationParams) {
	var request generated.PluginOperationRequest
	if err := decodePluginBody(w, r, &request, 256<<10); err != nil || request.Action == "" {
		writeProblem(w, r, http.StatusBadRequest, "REQUEST_INVALID", "Request body invalid", "Provide one typed plugin action and bounded object input.")
		return
	}
	if err := h.plugins.ValidateOperation(r.Context(), pluginID, projectID); err != nil {
		writeApplicationError(w, r, err)
		return
	}
	input, _ := json.Marshal(map[string]any{"pluginId": pluginID, "action": request.Action, "input": request.Input})
	identity := identityFrom(r.Context())
	operation, err := h.operations.Submit(r.Context(), operations.SubmitRequest{
		ProjectID: projectID, Kind: "plugin.operate", Input: input, IdempotencyKey: params.IdempotencyKey,
		ActorType: string(identity.Access), ActorID: identity.ActorID,
	})
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, operationResponse(operation))
}

func decodePluginBody(w http.ResponseWriter, r *http.Request, target any, limit int64) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return err
	}
	return nil
}
