package bootstrap

import (
	"context"
	"errors"
	"testing"

	operationsApplication "switchyard.dev/switchyard/internal/operations/application"
	operationsDomain "switchyard.dev/switchyard/internal/operations/domain"
	runtimeApplication "switchyard.dev/switchyard/internal/runtime/application"
	runtimeDomain "switchyard.dev/switchyard/internal/runtime/domain"
)

func TestValidateLoopbackAddress(t *testing.T) {
	t.Parallel()

	for _, address := range []string{"127.0.0.1:19616", "[::1]:19616", "localhost:19616"} {
		if err := validateLoopbackAddress(address); err != nil {
			t.Errorf("validateLoopbackAddress(%q) error = %v", address, err)
		}
	}
	for _, address := range []string{"0.0.0.0:19616", "192.0.2.1:19616", "missing-port"} {
		if err := validateLoopbackAddress(address); err == nil {
			t.Errorf("validateLoopbackAddress(%q) error = nil", address)
		}
	}
}

type runtimeSourceStub struct{ project runtimeDomain.ProjectRuntime }

func (s runtimeSourceStub) ResolveRuntime(context.Context, string) (runtimeDomain.ProjectRuntime, error) {
	return s.project, nil
}
func (s runtimeSourceStub) ListRuntimeProjectIDs(context.Context) ([]string, error) {
	return []string{s.project.ProjectID}, nil
}

type runtimeDriverStub struct{ executed runtimeDomain.Plan }

func (*runtimeDriverStub) Kind() runtimeDomain.Kind { return runtimeDomain.KindProcess }
func (*runtimeDriverStub) Inspect(context.Context, runtimeDomain.ProjectRuntime) (runtimeDomain.Observation, error) {
	return runtimeDomain.Observation{}, nil
}
func (*runtimeDriverStub) Plan(_ context.Context, request runtimeDomain.PlanRequest) (runtimeDomain.Plan, error) {
	return runtimeDomain.Plan{ProjectID: request.Project.ProjectID, Driver: runtimeDomain.KindProcess, Action: request.Action}, nil
}
func (s *runtimeDriverStub) Execute(_ context.Context, plan runtimeDomain.Plan, _ runtimeDomain.ProgressSink) error {
	s.executed = plan
	return nil
}
func (*runtimeDriverStub) StreamLogs(context.Context, runtimeDomain.LogRequest, runtimeDomain.LogSink) error {
	return nil
}
func (*runtimeDriverStub) StreamMetrics(context.Context, runtimeDomain.MetricRequest, runtimeDomain.MetricSink) error {
	return nil
}
func (*runtimeDriverStub) WatchEvents(context.Context, runtimeDomain.ProjectRuntime, runtimeDomain.EventSink) error {
	return nil
}

type requiredHealthStub struct {
	projectID string
	err       error
}

func (s *requiredHealthStub) WaitRequired(_ context.Context, projectID string) error {
	s.projectID = projectID
	return s.err
}

type progressStub struct{}

func (progressStub) Step(context.Context, string, string, string) error { return nil }

func TestExecuteRuntimeOperationCorrelatesAndWaitsForRequiredHealth(t *testing.T) {
	t.Parallel()
	driver := &runtimeDriverStub{}
	service := runtimeApplication.NewService(runtimeSourceStub{project: runtimeDomain.ProjectRuntime{ProjectID: "project-1", Kind: runtimeDomain.KindProcess}}, driver)
	healthErr := errors.New("not ready")
	health := &requiredHealthStub{err: healthErr}
	operation := operationsDomain.Operation{ID: "op-1", ProjectID: "project-1", Kind: "runtime.start", Input: []byte(`{"action":"start"}`)}
	err := executeRuntimeOperation(context.Background(), service, health, operation, operationsApplication.Progress(progressStub{}))
	if !errors.Is(err, healthErr) || health.projectID != "project-1" {
		t.Fatalf("error = %v, health project = %q", err, health.projectID)
	}
	if driver.executed.OperationID != "op-1" || driver.executed.Action != runtimeDomain.ActionStart {
		t.Fatalf("executed plan = %#v", driver.executed)
	}
}
