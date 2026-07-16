package application

import (
	"context"
	"errors"
	"testing"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

func TestServiceRoutesPlansAndBoundsLogs(t *testing.T) {
	t.Parallel()
	project := domain.ProjectRuntime{ProjectID: "project-1", Kind: domain.KindCompose}
	driver := &driverStub{kind: domain.KindCompose}
	service := NewService(projectSourceStub{projects: map[string]domain.ProjectRuntime{"project-1": project}}, driver)
	plan, err := service.Plan(context.Background(), "project-1", domain.ActionStop, false)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Action != domain.ActionStop || driver.planned.Project.ProjectID != "project-1" {
		t.Fatalf("plan = %#v, request = %#v", plan, driver.planned)
	}
	if _, err := service.Logs(context.Background(), "project-1", "", "", 10_001); err == nil {
		t.Fatal("oversized log request accepted")
	}
}

func TestRefreshWatchesAttachesProjectIdentity(t *testing.T) {
	t.Parallel()
	project := domain.ProjectRuntime{ProjectID: "project-1", Kind: domain.KindCompose, ManifestHash: "revision-1"}
	driver := &driverStub{kind: domain.KindCompose, watch: func(ctx context.Context, _ domain.ProjectRuntime, sink domain.EventSink) error {
		return sink.WriteRuntimeEvent(ctx, domain.RuntimeEvent{ProjectIdentity: "compose-project"})
	}}
	service := NewService(projectSourceStub{projects: map[string]domain.ProjectRuntime{"project-1": project}}, driver)
	results := make(chan watchResult, 1)
	sink := &eventSinkStub{events: make(chan domain.RuntimeEvent, 1)}
	active := make(map[string]activeWatch)
	service.refreshWatches(context.Background(), sink, active, results, func(string, error) {})
	event := <-sink.events
	if event.ProjectID != "project-1" {
		t.Fatalf("event = %#v", event)
	}
	result := <-results
	if result.err != nil || result.projectID != "project-1" {
		t.Fatalf("result = %#v", result)
	}
	active["project-1"].cancel()
}

type projectSourceStub struct {
	projects map[string]domain.ProjectRuntime
}

func (s projectSourceStub) ResolveRuntime(_ context.Context, id string) (domain.ProjectRuntime, error) {
	project, ok := s.projects[id]
	if !ok {
		return domain.ProjectRuntime{}, errors.New("missing project")
	}
	return project, nil
}

func (s projectSourceStub) ListRuntimeProjectIDs(context.Context) ([]string, error) {
	ids := make([]string, 0, len(s.projects))
	for id := range s.projects {
		ids = append(ids, id)
	}
	return ids, nil
}

type driverStub struct {
	kind    domain.Kind
	planned domain.PlanRequest
	watch   func(context.Context, domain.ProjectRuntime, domain.EventSink) error
}

func (d *driverStub) Kind() domain.Kind { return d.kind }
func (d *driverStub) Inspect(context.Context, domain.ProjectRuntime) (domain.Observation, error) {
	return domain.Observation{}, nil
}
func (d *driverStub) Plan(_ context.Context, request domain.PlanRequest) (domain.Plan, error) {
	d.planned = request
	return domain.Plan{ProjectID: request.Project.ProjectID, Driver: d.kind, Action: request.Action}, nil
}
func (*driverStub) Execute(context.Context, domain.Plan, domain.ProgressSink) error { return nil }
func (*driverStub) StreamLogs(context.Context, domain.LogRequest, domain.LogSink) error {
	return nil
}
func (*driverStub) StreamMetrics(context.Context, domain.MetricRequest, domain.MetricSink) error {
	return nil
}
func (d *driverStub) WatchEvents(ctx context.Context, project domain.ProjectRuntime, sink domain.EventSink) error {
	if d.watch == nil {
		return nil
	}
	return d.watch(ctx, project, sink)
}

type eventSinkStub struct {
	events chan domain.RuntimeEvent
}

func (s *eventSinkStub) WriteRuntimeEvent(_ context.Context, event domain.RuntimeEvent) error {
	s.events <- event
	return nil
}
