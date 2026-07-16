package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	observability "switchyard.dev/switchyard/internal/observability/domain"
	runtime "switchyard.dev/switchyard/internal/runtime/domain"
)

type healthSourceStub struct{ project runtime.ProjectRuntime }

func (s healthSourceStub) ResolveRuntime(context.Context, string) (runtime.ProjectRuntime, error) {
	return s.project, nil
}
func (s healthSourceStub) ListRuntimeProjectIDs(context.Context) ([]string, error) {
	return []string{s.project.ProjectID}, nil
}

type healthObserverStub struct {
	mu    sync.Mutex
	calls int
}

func (s *healthObserverStub) Inspect(context.Context, string) (runtime.Observation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	state := "starting"
	if s.calls > 1 {
		state = "running"
	}
	return runtime.Observation{ProjectID: "project-1", State: runtime.StateRunning, Services: []runtime.ServiceObservation{{ID: "api", State: state}}, ObservedAt: time.Now().UTC()}, nil
}

type healthRepositoryStub struct {
	mu      sync.Mutex
	results []observability.HealthResult
}

func (s *healthRepositoryStub) AppendHealth(_ context.Context, result observability.HealthResult) error {
	s.mu.Lock()
	s.results = append(s.results, result)
	s.mu.Unlock()
	return nil
}
func (s *healthRepositoryStub) LatestHealth(context.Context, string) ([]observability.HealthResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]observability.HealthResult(nil), s.results...), nil
}
func (*healthRepositoryStub) PruneHealth(context.Context, time.Time) error { return nil }

type observationEvaluator struct{}

func (observationEvaluator) Evaluate(_ context.Context, _ runtime.ProjectRuntime, observation runtime.Observation, _ runtime.HealthCheckDefinition) observability.HealthResult {
	status := observability.StatusUnhealthy
	if observation.Services[0].State == "running" {
		status = observability.StatusHealthy
	}
	return observability.HealthResult{Status: status, Message: string(status), ObservedAt: time.Now().UTC()}
}

func TestWaitRequiredReobservesBetweenRetries(t *testing.T) {
	t.Parallel()
	project := runtime.ProjectRuntime{ProjectID: "project-1", Services: []runtime.ServiceDeclaration{{ID: "api", HealthChecks: []runtime.HealthCheckDefinition{{
		ID: "alive", ServiceID: "api", Type: "process", Required: true, Retries: 1,
	}}}}}
	observer := &healthObserverStub{}
	repository := &healthRepositoryStub{}
	service := NewHealthService(healthSourceStub{project: project}, observer, repository, observationEvaluator{})
	if err := service.WaitRequired(context.Background(), project.ProjectID); err != nil {
		t.Fatalf("WaitRequired() error = %v", err)
	}
	if observer.calls != 2 || len(repository.results) != 1 || repository.results[0].Status != observability.StatusHealthy {
		t.Fatalf("calls = %d, results = %#v", observer.calls, repository.results)
	}
}

type alwaysUnhealthy struct{}

func (alwaysUnhealthy) Evaluate(context.Context, runtime.ProjectRuntime, runtime.Observation, runtime.HealthCheckDefinition) observability.HealthResult {
	return observability.HealthResult{Status: observability.StatusUnhealthy, Message: "not ready", ObservedAt: time.Now().UTC()}
}

func TestWaitRequiredReportsReadinessFailureWithoutRuntimeStop(t *testing.T) {
	t.Parallel()
	project := runtime.ProjectRuntime{ProjectID: "project-1", Services: []runtime.ServiceDeclaration{{ID: "api", HealthChecks: []runtime.HealthCheckDefinition{{
		ID: "ready", ServiceID: "api", Type: "http", Required: true,
	}}}}}
	service := NewHealthService(healthSourceStub{project: project}, &healthObserverStub{}, &healthRepositoryStub{}, alwaysUnhealthy{})
	err := service.WaitRequired(context.Background(), project.ProjectID)
	if !errors.Is(err, ErrRequiredHealthChecks) {
		t.Fatalf("WaitRequired() error = %v", err)
	}
}

func TestCompositeAnyUsesMemberResults(t *testing.T) {
	t.Parallel()
	results := map[string]observability.HealthResult{
		"api\x00one": {Status: observability.StatusUnhealthy},
		"api\x00two": {Status: observability.StatusHealthy},
	}
	result := compositeResult("project-1", runtime.HealthCheckDefinition{ID: "ready", ServiceID: "api", Mode: "any", Members: []string{"one", "two"}}, results, time.Now())
	if result.Status != observability.StatusHealthy || result.ProjectID != "project-1" {
		t.Fatalf("result = %#v", result)
	}
}
