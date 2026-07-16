package httpapi

import (
	"net/http"

	"switchyard.dev/switchyard/internal/operations/domain"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

func (h *handler) GetOperation(w http.ResponseWriter, r *http.Request, operationID generated.OperationId) {
	operation, err := h.operations.Get(r.Context(), operationID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, operationResponse(operation))
}

func (h *handler) CancelOperation(
	w http.ResponseWriter,
	r *http.Request,
	operationID generated.OperationId,
	params generated.CancelOperationParams,
) {
	identity := identityFrom(r.Context())
	operation, err := h.operations.Cancel(
		r.Context(), operationID, string(identity.Access), identity.ActorID, params.IdempotencyKey,
	)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, operationResponse(operation))
}

func operationResponse(operation domain.Operation) generated.Operation {
	response := generated.Operation{
		Id: operation.ID, ProjectId: operation.ProjectID, Kind: operation.Kind,
		State: generated.OperationState(operation.State), CancellationRequested: operation.CancellationRequested,
		RequestedAt: operation.RequestedAt, StartedAt: operation.StartedAt,
		FinishedAt: operation.FinishedAt, UpdatedAt: operation.UpdatedAt,
	}
	if operation.ErrorCode != "" {
		response.ErrorCode = &operation.ErrorCode
	}
	if operation.ErrorMessage != "" {
		response.ErrorMessage = &operation.ErrorMessage
	}
	return response
}
