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

	actionsDomain "switchyard.dev/switchyard/internal/actions/domain"
	operations "switchyard.dev/switchyard/internal/operations/application"
	operationsDomain "switchyard.dev/switchyard/internal/operations/domain"
	portsDomain "switchyard.dev/switchyard/internal/ports/domain"
	session "switchyard.dev/switchyard/internal/session/application"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

type portServiceStub struct{ registry portsDomain.Registry }

func (s portServiceStub) Registry(context.Context) (portsDomain.Registry, error) {
	return s.registry, nil
}
func (portServiceStub) Suggest(context.Context, int, int, string, string, []int) (portsDomain.Suggestion, error) {
	return portsDomain.Suggestion{}, nil
}

type actionServiceStub struct{ actions actionsDomain.ProjectActions }

func (s actionServiceStub) List(context.Context, string) (actionsDomain.ProjectActions, error) {
	return s.actions, nil
}

type recordingOperations struct {
	operationStub
	request operations.SubmitRequest
}

func (s *recordingOperations) Submit(_ context.Context, request operations.SubmitRequest) (operationsDomain.Operation, error) {
	s.request = request
	return operationsDomain.Operation{ID: "operation-1", ProjectID: request.ProjectID, Kind: request.Kind,
		State: operationsDomain.StateQueued, RequestedAt: time.Now(), UpdatedAt: time.Now()}, nil
}

func TestPortRegistrySerializesRequiredCollectionsAsArrays(t *testing.T) {
	t.Parallel()
	handler := &handler{ports: portServiceStub{registry: portsDomain.Registry{ObservedAt: time.Now()}}}
	response := httptest.NewRecorder()
	handler.GetPortRegistry(response, httptest.NewRequest(http.MethodGet, "/api/v1/ports", nil))
	var payload map[string]json.RawMessage
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"facts", "conflicts", "warnings"} {
		if string(payload[key]) != "[]" {
			t.Fatalf("%s = %s", key, payload[key])
		}
	}
}

func TestDestructiveActionRequiresConfirmationBeforeQueue(t *testing.T) {
	t.Parallel()
	operations := &recordingOperations{}
	handler := &handler{operations: operations, actions: actionServiceStub{actions: actionsDomain.ProjectActions{Actions: []actionsDomain.Definition{
		{ID: "destroy", Type: "command", Risk: actionsDomain.RiskDestructive},
	}}}}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/projects/project/actions/destroy/operations", strings.NewReader(`{}`))
	response := httptest.NewRecorder()
	handler.CreateActionOperation(response, request, "project", "destroy", generated.CreateActionOperationParams{IdempotencyKey: "request-key"})
	if response.Code != http.StatusConflict || operations.request.Kind != "" {
		t.Fatalf("status=%d request=%#v", response.Code, operations.request)
	}
}

func TestConfirmedActionQueuesActorBoundOperation(t *testing.T) {
	t.Parallel()
	operations := &recordingOperations{}
	handler := &handler{operations: operations, actions: actionServiceStub{actions: actionsDomain.ProjectActions{Actions: []actionsDomain.Definition{
		{ID: "tests", Type: "tests.run", Risk: actionsDomain.RiskMutating},
	}}}}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/projects/project/actions/tests/operations", strings.NewReader(`{"confirmRisk":false}`))
	response := httptest.NewRecorder()
	handler.CreateActionOperation(response, request, "project", "tests", generated.CreateActionOperationParams{IdempotencyKey: "request-key"})
	if response.Code != http.StatusAccepted || operations.request.Kind != "action.run" || !strings.Contains(string(operations.request.Input), `"actionId":"tests"`) {
		t.Fatalf("status=%d request=%#v", response.Code, operations.request)
	}
}

func TestAgentIdentityHeadersBindOperationAuditActor(t *testing.T) {
	t.Parallel()
	operations := &recordingOperations{}
	ipc := NewIPC(Dependencies{
		System: systemStub{}, Operations: operations, Runtime: runtimeStub{}, Sessions: session.NewManager(),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/projects/project/operations", strings.NewReader(`{"action":"start"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(idempotencyHeader, "agent-request-key")
	request.Header.Set(actorTypeHeader, "agent")
	request.Header.Set(actorIDHeader, "codex/reviewer")
	response := httptest.NewRecorder()
	ipc.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted || operations.request.ActorType != "agent" || operations.request.ActorID != "codex/reviewer" {
		t.Fatalf("status=%d request=%#v body=%s", response.Code, operations.request, response.Body.String())
	}
}

func TestInvalidAgentIdentityIsRejectedAtIPCBoundary(t *testing.T) {
	t.Parallel()
	ipc := NewIPC(Dependencies{System: systemStub{}, Sessions: session.NewManager(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/system", nil)
	request.Header.Set(actorTypeHeader, "agent")
	request.Header.Set(actorIDHeader, "contains spaces")
	response := httptest.NewRecorder()
	ipc.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "ACTOR_IDENTITY_INVALID") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
