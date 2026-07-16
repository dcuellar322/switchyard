package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	diagnostics "switchyard.dev/switchyard/internal/diagnostics/application"
	diagnosticsDomain "switchyard.dev/switchyard/internal/diagnostics/domain"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

type diagnosticServiceStub struct {
	authorized diagnosticsDomain.Diagnosis
	err        error
}

func (diagnosticServiceStub) Diagnose(context.Context, string, string) (diagnosticsDomain.Diagnosis, error) {
	return diagnosticsDomain.Diagnosis{}, nil
}
func (diagnosticServiceStub) Get(context.Context, string) (diagnosticsDomain.Diagnosis, error) {
	return diagnosticsDomain.Diagnosis{}, nil
}
func (diagnosticServiceStub) Latest(context.Context, string) (diagnosticsDomain.Diagnosis, error) {
	return diagnosticsDomain.Diagnosis{}, nil
}
func (diagnosticServiceStub) RecordFeedback(context.Context, string, string, string, string) (diagnosticsDomain.Feedback, error) {
	return diagnosticsDomain.Feedback{}, nil
}
func (s diagnosticServiceStub) AuthorizeAction(context.Context, string, string) (diagnosticsDomain.Diagnosis, error) {
	return s.authorized, s.err
}
func (diagnosticServiceStub) Notifications(context.Context, string, bool, int) ([]diagnosticsDomain.Notification, error) {
	return []diagnosticsDomain.Notification{}, nil
}
func (diagnosticServiceStub) Acknowledge(context.Context, string) (diagnosticsDomain.Notification, error) {
	return diagnosticsDomain.Notification{}, nil
}

func TestDiagnosticActionCannotLaunchAnArbitraryAction(t *testing.T) {
	t.Parallel()
	operations := &recordingOperations{}
	handler := &handler{operations: operations, diagnostics: diagnosticServiceStub{err: diagnostics.ErrActionNotSuggested}}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/diagnoses/diagnosis-1/actions/destroy", nil)
	response := httptest.NewRecorder()
	handler.CreateDiagnosticActionOperation(response, request, "diagnosis-1", "destroy", generated.CreateDiagnosticActionOperationParams{IdempotencyKey: "request-key"})
	if response.Code != http.StatusForbidden || operations.request.Kind != "" || !strings.Contains(response.Body.String(), "DIAGNOSTIC_ACTION_DENIED") {
		t.Fatalf("status=%d request=%#v body=%s", response.Code, operations.request, response.Body.String())
	}
}

func TestDiagnosticActionQueuesOnlyTheAuthorizedProjectAction(t *testing.T) {
	t.Parallel()
	operations := &recordingOperations{}
	handler := &handler{operations: operations, diagnostics: diagnosticServiceStub{authorized: diagnosticsDomain.Diagnosis{ProjectID: "project-1"}}}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/diagnoses/diagnosis-1/actions/tests", nil)
	response := httptest.NewRecorder()
	handler.CreateDiagnosticActionOperation(response, request, "diagnosis-1", "tests", generated.CreateDiagnosticActionOperationParams{IdempotencyKey: "request-key"})
	if response.Code != http.StatusAccepted || operations.request.ProjectID != "project-1" || operations.request.Kind != "action.run" {
		t.Fatalf("status=%d request=%#v body=%s", response.Code, operations.request, response.Body.String())
	}
	input := string(operations.request.Input)
	if !strings.Contains(input, `"actionId":"tests"`) || !strings.Contains(input, `"confirmRisk":false`) || strings.Contains(input, "destroy") {
		t.Fatalf("input=%s", input)
	}
}

var _ diagnosticService = diagnosticServiceStub{}
