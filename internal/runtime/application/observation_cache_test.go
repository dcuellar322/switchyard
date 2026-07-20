package application

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

func TestInspectCoalescesConcurrentObservationsAndReturnsIndependentValues(t *testing.T) {
	t.Parallel()
	driver := &countingDriver{started: make(chan struct{}), release: make(chan struct{})}
	service := NewService(singleProjectSource(), driver)

	const callers = 8
	results := make(chan domain.Observation, callers)
	var wait sync.WaitGroup
	for range callers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			observation, err := service.Inspect(context.Background(), "project-1")
			if err != nil {
				t.Errorf("Inspect() error = %v", err)
				return
			}
			results <- observation
		}()
	}
	<-driver.started
	if got := driver.inspections.Load(); got != 1 {
		t.Fatalf("concurrent driver inspections = %d, want 1", got)
	}
	close(driver.release)
	wait.Wait()
	close(results)

	first := <-results
	first.Services[0].State = "mutated"
	first.Services[0].Ports[0].HostPort = 1
	second, err := service.Inspect(context.Background(), "project-1")
	if err != nil {
		t.Fatal(err)
	}
	if second.Services[0].State != "running" || second.Services[0].Ports[0].HostPort != 8080 {
		t.Fatalf("cached observation was aliased: %#v", second.Services[0])
	}
	if got := driver.inspections.Load(); got != 1 {
		t.Fatalf("cached driver inspections = %d, want 1", got)
	}
}

func TestExecuteInvalidatesCachedObservation(t *testing.T) {
	t.Parallel()
	driver := &countingDriver{}
	service := NewService(singleProjectSource(), driver)
	if _, err := service.Inspect(context.Background(), "project-1"); err != nil {
		t.Fatal(err)
	}
	if err := service.Execute(context.Background(), domain.Plan{
		ProjectID: "project-1", Driver: domain.KindCompose,
	}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Inspect(context.Background(), "project-1"); err != nil {
		t.Fatal(err)
	}
	if got := driver.inspections.Load(); got != 2 {
		t.Fatalf("driver inspections after lifecycle = %d, want 2", got)
	}
}

func singleProjectSource() projectSourceStub {
	return projectSourceStub{projects: map[string]domain.ProjectRuntime{
		"project-1": {ProjectID: "project-1", Kind: domain.KindCompose},
	}}
}

type countingDriver struct {
	inspections atomic.Int32
	started     chan struct{}
	release     chan struct{}
	startOnce   sync.Once
}

func (*countingDriver) Kind() domain.Kind { return domain.KindCompose }

func (d *countingDriver) Inspect(context.Context, domain.ProjectRuntime) (domain.Observation, error) {
	d.inspections.Add(1)
	if d.started != nil {
		d.startOnce.Do(func() { close(d.started) })
		<-d.release
	}
	return domain.Observation{Services: []domain.ServiceObservation{{
		ID: "web", State: "running", Ports: []domain.PublishedPort{{HostPort: 8080}},
	}}}, nil
}

func (*countingDriver) Plan(context.Context, domain.PlanRequest) (domain.Plan, error) {
	return domain.Plan{}, nil
}
func (*countingDriver) Execute(context.Context, domain.Plan, domain.ProgressSink) error { return nil }
func (*countingDriver) StreamLogs(context.Context, domain.LogRequest, domain.LogSink) error {
	return nil
}
func (*countingDriver) StreamMetrics(context.Context, domain.MetricRequest, domain.MetricSink) error {
	return nil
}
func (*countingDriver) WatchEvents(context.Context, domain.ProjectRuntime, domain.EventSink) error {
	return nil
}
