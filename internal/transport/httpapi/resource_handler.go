package httpapi

import (
	"net/http"

	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

func (h *handler) GetResourceOverview(w http.ResponseWriter, r *http.Request) {
	overview, err := h.resources.Overview(r.Context())
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (h *handler) GetStorageInventory(w http.ResponseWriter, r *http.Request) {
	inventory, err := h.resources.Storage(r.Context())
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, inventory)
}

func (h *handler) GetCleanupPreview(w http.ResponseWriter, r *http.Request, params generated.GetCleanupPreviewParams) {
	projectID := ""
	if params.ProjectId != nil {
		projectID = *params.ProjectId
	}
	preview, err := h.resources.CleanupPreview(r.Context(), projectID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (h *handler) GetMetricHistory(w http.ResponseWriter, r *http.Request, projectID generated.ProjectId, params generated.GetMetricHistoryParams) {
	service, resolution, maxPoints := "", "auto", 0
	if params.Service != nil {
		service = *params.Service
	}
	if params.Resolution != nil {
		resolution = string(*params.Resolution)
	}
	if params.MaxPoints != nil {
		maxPoints = *params.MaxPoints
	}
	history, err := h.resources.History(r.Context(), projectID, service, resolution, params.From, params.To, maxPoints)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, history)
}
