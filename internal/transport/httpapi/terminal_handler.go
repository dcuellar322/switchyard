package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	terminalDomain "switchyard.dev/switchyard/internal/terminal/domain"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

const maximumTerminalRequestBytes = 16 << 10

func (h *handler) ListTerminalSessions(w http.ResponseWriter, r *http.Request, params generated.ListTerminalSessionsParams) {
	projectID := ""
	if params.ProjectId != nil {
		projectID = *params.ProjectId
	}
	items, err := h.terminals.List(r.Context(), projectID, terminalOwner(r))
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, terminalSessionResponses(items, false))
}

func (h *handler) CreateTerminalSession(w http.ResponseWriter, r *http.Request, _ generated.CreateTerminalSessionParams) {
	var request generated.TerminalSessionCreate
	if !decodeTerminalRequest(w, r, &request) {
		return
	}
	create, ok := terminalCreateRequest(w, r, request)
	if !ok {
		return
	}
	session, err := h.terminals.Create(r.Context(), create, terminalOwner(r))
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, terminalSessionResponse(session))
}

func (h *handler) GetTerminalSession(w http.ResponseWriter, r *http.Request, sessionID generated.TerminalSessionId) {
	session, err := h.terminals.Get(r.Context(), sessionID, terminalOwner(r))
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, terminalSessionResponse(session))
}

func (h *handler) TerminateTerminalSession(w http.ResponseWriter, r *http.Request, sessionID generated.TerminalSessionId, _ generated.TerminateTerminalSessionParams) {
	session, err := h.terminals.Terminate(r.Context(), sessionID, terminalOwner(r))
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, terminalSessionResponse(session))
}

func (h *handler) ListAgentSessions(w http.ResponseWriter, r *http.Request, params generated.ListAgentSessionsParams) {
	projectID := ""
	if params.ProjectId != nil {
		projectID = *params.ProjectId
	}
	items, err := h.terminals.List(r.Context(), projectID, terminalOwner(r))
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, terminalSessionResponses(items, true))
}

func (h *handler) CreateAgentSession(w http.ResponseWriter, r *http.Request, _ generated.CreateAgentSessionParams) {
	var request generated.AgentSessionCreate
	if !decodeTerminalRequest(w, r, &request) {
		return
	}
	create := terminalDomain.CreateRequest{
		ProjectID: request.ProjectId, Kind: terminalDomain.KindAgent, Provider: string(request.Provider),
		Columns: terminalDimension(request.Columns), Rows: terminalDimension(request.Rows),
	}
	if request.EnvironmentId != nil {
		create.EnvironmentID = *request.EnvironmentId
	}
	if err := create.Validate(); err != nil {
		writeProblem(w, r, http.StatusUnprocessableEntity, "TERMINAL_REQUEST_INVALID", "Agent session request invalid", err.Error())
		return
	}
	session, err := h.terminals.Create(r.Context(), create, terminalOwner(r))
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, terminalSessionResponse(session))
}

func terminalOwner(r *http.Request) terminalDomain.Owner {
	identity := identityFrom(r.Context())
	return terminalDomain.Owner{Type: string(identity.Access), ID: identity.ActorID}
}

func decodeTerminalRequest(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maximumTerminalRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "REQUEST_INVALID", "Request body invalid", "Provide one supported terminal launch request.")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeProblem(w, r, http.StatusBadRequest, "REQUEST_INVALID", "Request body invalid", "Provide exactly one JSON document.")
		return false
	}
	return true
}

func terminalCreateRequest(w http.ResponseWriter, r *http.Request, request generated.TerminalSessionCreate) (terminalDomain.CreateRequest, bool) {
	create := terminalDomain.CreateRequest{
		ProjectID: request.ProjectId, Kind: terminalDomain.Kind(request.Kind),
		Columns: terminalDimension(request.Columns), Rows: terminalDimension(request.Rows),
	}
	if request.EnvironmentId != nil {
		create.EnvironmentID = *request.EnvironmentId
	}
	if request.Provider != nil {
		create.Provider = string(*request.Provider)
	}
	if request.ServiceId != nil {
		create.ServiceID = *request.ServiceId
	}
	if request.ActionId != nil {
		create.ActionID = *request.ActionId
	}
	if request.Shell != nil {
		create.Shell = string(*request.Shell)
	}
	if request.DatabaseClient != nil {
		create.DatabaseClient = string(*request.DatabaseClient)
	}
	if request.Columns < 0 || request.Columns > 65535 || request.Rows < 0 || request.Rows > 65535 {
		writeProblem(w, r, http.StatusUnprocessableEntity, "TERMINAL_REQUEST_INVALID", "Terminal session request invalid", "Terminal dimensions are outside the supported range.")
		return terminalDomain.CreateRequest{}, false
	}
	if err := create.Validate(); err != nil {
		writeProblem(w, r, http.StatusUnprocessableEntity, "TERMINAL_REQUEST_INVALID", "Terminal session request invalid", err.Error())
		return terminalDomain.CreateRequest{}, false
	}
	return create, true
}

func terminalDimension(value int) uint16 {
	if value < 0 || value > 65535 {
		return 0
	}
	return uint16(value)
}

func terminalSessionResponses(items []terminalDomain.Session, agentsOnly bool) []generated.TerminalSession {
	response := make([]generated.TerminalSession, 0, len(items))
	for _, item := range items {
		if agentsOnly && item.Kind != terminalDomain.KindAgent {
			continue
		}
		response = append(response, terminalSessionResponse(item))
	}
	return response
}

func terminalSessionResponse(item terminalDomain.Session) generated.TerminalSession {
	return generated.TerminalSession{
		Id: item.ID, ProjectId: item.ProjectID, Kind: generated.TerminalSessionKind(item.Kind), DisplayName: item.DisplayName,
		Owner: generated.TerminalSessionOwner{Type: item.Owner.Type, Id: item.Owner.ID}, WorkingDirectory: item.WorkingDirectory,
		Status: generated.TerminalSessionStatus(item.Status), PersistencePolicy: generated.TerminalSessionPersistencePolicy(item.PersistencePolicy),
		CapturePolicy: generated.TerminalSessionCapturePolicy(item.CapturePolicy), OutputBytes: item.OutputBytes, OutputTruncated: item.OutputTruncated,
		EnvironmentId: optionalNonEmptyString(item.EnvironmentID), Provider: optionalNonEmptyString(item.Provider), ServiceId: optionalNonEmptyString(item.ServiceID),
		ActionId: optionalNonEmptyString(item.ActionID), ErrorCode: optionalNonEmptyString(item.ErrorCode), ExitCode: item.ExitCode,
		CreatedAt: item.CreatedAt, LastAttachedAt: item.LastAttachedAt, DetachedAt: item.DetachedAt, LastOutputAt: item.LastOutputAt,
		FinishedAt: item.FinishedAt,
	}
}

func optionalNonEmptyString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
