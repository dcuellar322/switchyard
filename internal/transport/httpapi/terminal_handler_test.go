package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	session "switchyard.dev/switchyard/internal/session/application"
	terminalApplication "switchyard.dev/switchyard/internal/terminal/application"
	terminalDomain "switchyard.dev/switchyard/internal/terminal/domain"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

func TestTerminalHandlersTranslateTypedRequestAndAuthenticatedOwner(t *testing.T) {
	t.Parallel()
	service := &terminalServiceStub{}
	handler := NewIPC(Dependencies{Terminals: service, Sessions: session.NewManager(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/terminal-sessions", strings.NewReader(`{"projectId":"project-one","environmentId":"environment-one","kind":"database","serviceId":"db","databaseClient":"psql","columns":120,"rows":36}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(idempotencyHeader, "terminal-create-key")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if service.owner != (terminalDomain.Owner{Type: "ipc", ID: "ipc"}) || service.request.Kind != terminalDomain.KindDatabase || service.request.DatabaseClient != "psql" || service.request.EnvironmentID != "environment-one" {
		t.Fatalf("request = %#v, owner = %#v", service.request, service.owner)
	}
	var session generated.TerminalSession
	if err := json.NewDecoder(response.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	if session.Id != "terminal-created" || session.CapturePolicy != generated.TerminalSessionCapturePolicy(terminalDomain.CaptureUserVisibleOutput) {
		t.Fatalf("response = %#v", session)
	}
}

func TestTerminalHandlersRejectInvalidDimensionsBeforeApplication(t *testing.T) {
	t.Parallel()
	service := &terminalServiceStub{}
	handler := NewIPC(Dependencies{Terminals: service, Sessions: session.NewManager(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/terminal-sessions", strings.NewReader(`{"projectId":"project-one","kind":"shell","columns":70000,"rows":24}`))
	request.Header.Set(idempotencyHeader, "terminal-create-key")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity || service.createCalls != 0 {
		t.Fatalf("status = %d, calls = %d, body = %s", response.Code, service.createCalls, response.Body.String())
	}
}

func TestAgentListContainsOnlyAgentSessions(t *testing.T) {
	t.Parallel()
	service := &terminalServiceStub{items: []terminalDomain.Session{
		persistedTerminal("terminal-shell", terminalDomain.KindShell),
		persistedTerminal("terminal-agent", terminalDomain.KindAgent),
	}}
	handler := NewIPC(Dependencies{Terminals: service, Sessions: session.NewManager(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/agents/sessions?projectId=project-one", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var sessions []generated.TerminalSession
	if err := json.NewDecoder(response.Body).Decode(&sessions); err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || sessions[0].Kind != generated.TerminalSessionKind(terminalDomain.KindAgent) {
		t.Fatalf("sessions = %#v", sessions)
	}
}

type terminalServiceStub struct {
	request     terminalDomain.CreateRequest
	owner       terminalDomain.Owner
	items       []terminalDomain.Session
	createCalls int
}

func (s *terminalServiceStub) Create(_ context.Context, request terminalDomain.CreateRequest, owner terminalDomain.Owner) (terminalDomain.Session, error) {
	s.request, s.owner = request, owner
	s.createCalls++
	item := persistedTerminal("terminal-created", request.Kind)
	item.Owner, item.ProjectID, item.EnvironmentID = owner, request.ProjectID, request.EnvironmentID
	return item, nil
}
func (s *terminalServiceStub) List(context.Context, string, terminalDomain.Owner) ([]terminalDomain.Session, error) {
	return s.items, nil
}
func (s *terminalServiceStub) Get(_ context.Context, id string, _ terminalDomain.Owner) (terminalDomain.Session, error) {
	return persistedTerminal(id, terminalDomain.KindShell), nil
}
func (s *terminalServiceStub) Terminate(_ context.Context, id string, _ terminalDomain.Owner) (terminalDomain.Session, error) {
	item := persistedTerminal(id, terminalDomain.KindShell)
	item.Status = terminalDomain.StatusTerminated
	return item, nil
}
func (s *terminalServiceStub) Attach(context.Context, string, terminalDomain.Owner) (*terminalApplication.Attachment, error) {
	return nil, terminalApplication.ErrNotActive
}

func persistedTerminal(id string, kind terminalDomain.Kind) terminalDomain.Session {
	return terminalDomain.Session{
		ID: id, ProjectID: "project-one", Kind: kind, DisplayName: "Project shell", Owner: terminalDomain.Owner{Type: "ipc", ID: "ipc"},
		WorkingDirectory: "/tmp/project", Status: terminalDomain.StatusActive, PersistencePolicy: terminalDomain.PersistenceDetachUntilIdle,
		CapturePolicy: terminalDomain.CaptureUserVisibleOutput, CreatedAt: time.Now().UTC(),
	}
}
