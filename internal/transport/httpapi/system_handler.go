package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"switchyard.dev/switchyard/internal/system/application"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

type systemQuery interface {
	Get(ctx context.Context) (application.Info, error)
}

type handler struct {
	system systemQuery
}

func (h *handler) GetSystem(w http.ResponseWriter, r *http.Request) {
	info, err := h.system.Get(r.Context())
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "SYSTEM_STATUS_UNAVAILABLE", "System status unavailable")
		return
	}
	response := generated.SystemInfo{
		Status:                generated.Ready,
		Version:               info.Version,
		Commit:                info.Commit,
		BuiltAt:               info.BuiltAt,
		ApiVersion:            info.APIVersion,
		DatabaseSchemaVersion: info.DatabaseSchemaVersion,
		StartedAt:             info.StartedAt,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}
