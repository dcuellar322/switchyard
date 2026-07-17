// Package application coordinates trusted projects with runtime drivers.
package application

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

// ProjectSource resolves trusted catalog projects into driver-neutral runtime inputs.
type ProjectSource interface {
	ResolveRuntime(context.Context, string) (domain.ProjectRuntime, error)
	ListRuntimeProjectIDs(context.Context) ([]string, error)
}

// Driver is the application-facing runtime adapter contract.
type Driver interface {
	Kind() domain.Kind
	Inspect(context.Context, domain.ProjectRuntime) (domain.Observation, error)
	Plan(context.Context, domain.PlanRequest) (domain.Plan, error)
	Execute(context.Context, domain.Plan, domain.ProgressSink) error
	StreamLogs(context.Context, domain.LogRequest, domain.LogSink) error
	StreamMetrics(context.Context, domain.MetricRequest, domain.MetricSink) error
	WatchEvents(context.Context, domain.ProjectRuntime, domain.EventSink) error
}

// Service exposes runtime use cases independent of HTTP, CLI, Docker, and persistence.
type Service struct {
	projects ProjectSource
	drivers  map[domain.Kind]Driver
}

// NewService constructs runtime use cases from explicit project and driver boundaries.
func NewService(projects ProjectSource, drivers ...Driver) *Service {
	byKind := make(map[domain.Kind]Driver, len(drivers))
	for _, driver := range drivers {
		byKind[driver.Kind()] = driver
	}
	return &Service{projects: projects, drivers: byKind}
}

// Inspect returns a live observation without persisting derived state as authority.
func (s *Service) Inspect(ctx context.Context, projectID string) (domain.Observation, error) {
	project, driver, err := s.resolve(ctx, projectID)
	if err != nil {
		return domain.Observation{}, err
	}
	return driver.Inspect(ctx, project)
}

// Plan returns a side-effect-free lifecycle preview.
func (s *Service) Plan(ctx context.Context, projectID string, action domain.Action, removeVolumes bool) (domain.Plan, error) {
	return s.PlanServices(ctx, projectID, action, removeVolumes, nil)
}

// PlanServices returns a side-effect-free preview scoped to declared service IDs.
func (s *Service) PlanServices(ctx context.Context, projectID string, action domain.Action, removeVolumes bool, services []string) (domain.Plan, error) {
	return s.PlanSelection(ctx, projectID, action, removeVolumes, services, nil)
}

// PlanSelection returns a side-effect-free preview scoped to declared service IDs and trusted Compose profiles.
func (s *Service) PlanSelection(ctx context.Context, projectID string, action domain.Action, removeVolumes bool, services, profiles []string) (domain.Plan, error) {
	project, driver, err := s.resolve(ctx, projectID)
	if err != nil {
		return domain.Plan{}, err
	}
	return driver.Plan(ctx, domain.PlanRequest{Project: project, Action: action, RemoveVolumes: removeVolumes, Services: services, Profiles: profiles})
}

// Execute applies a previously produced plan through its owning driver.
func (s *Service) Execute(ctx context.Context, plan domain.Plan, sink domain.ProgressSink) error {
	driver, ok := s.drivers[plan.Driver]
	if !ok {
		return fmt.Errorf("%w: %s", domain.ErrUnsupportedDriver, plan.Driver)
	}
	return driver.Execute(ctx, plan, sink)
}

// Logs collects a bounded snapshot while preserving the streaming driver contract.
func (s *Service) Logs(ctx context.Context, projectID, service, since string, tail int) ([]domain.LogEntry, error) {
	if tail < 1 || tail > 10_000 {
		return nil, errors.New("log tail must be between 1 and 10000")
	}
	project, driver, err := s.resolve(ctx, projectID)
	if err != nil {
		return nil, err
	}
	sink := &logCollector{limit: tail}
	err = driver.StreamLogs(ctx, domain.LogRequest{Project: project, Service: service, Since: since, Tail: tail}, sink)
	return sink.entries, err
}

// FollowLogs streams a raw driver feed into a caller-owned redaction and persistence sink.
func (s *Service) FollowLogs(ctx context.Context, projectID, service, since string, tail int, sink domain.LogSink) error {
	project, driver, err := s.resolve(ctx, projectID)
	if err != nil {
		return err
	}
	return driver.StreamLogs(ctx, domain.LogRequest{Project: project, Service: service, Since: since, Tail: tail, Follow: true}, sink)
}

// ListProjectIDs returns trusted projects eligible for runtime observation.
func (s *Service) ListProjectIDs(ctx context.Context) ([]string, error) {
	return s.projects.ListRuntimeProjectIDs(ctx)
}

// Metrics returns one current sample per selected service.
func (s *Service) Metrics(ctx context.Context, projectID, service string) ([]domain.MetricSample, error) {
	project, driver, err := s.resolve(ctx, projectID)
	if err != nil {
		return nil, err
	}
	sink := &metricCollector{}
	err = driver.StreamMetrics(ctx, domain.MetricRequest{Project: project, Service: service}, sink)
	return sink.samples, err
}

// WatchProject subscribes to label-targeted runtime events for one project.
func (s *Service) WatchProject(ctx context.Context, projectID string, sink domain.EventSink) error {
	project, driver, err := s.resolve(ctx, projectID)
	if err != nil {
		return err
	}
	return driver.WatchEvents(ctx, project, sink)
}

func (s *Service) resolve(ctx context.Context, projectID string) (domain.ProjectRuntime, Driver, error) {
	project, err := s.projects.ResolveRuntime(ctx, projectID)
	if err != nil {
		return domain.ProjectRuntime{}, nil, err
	}
	driver, ok := s.drivers[project.Kind]
	if !ok {
		return domain.ProjectRuntime{}, nil, fmt.Errorf("%w: %s", domain.ErrUnsupportedDriver, project.Kind)
	}
	return project, driver, nil
}

type logCollector struct {
	mu      sync.Mutex
	limit   int
	entries []domain.LogEntry
}

func (s *logCollector) WriteLog(_ context.Context, entry domain.LogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.entries) >= s.limit {
		return nil
	}
	s.entries = append(s.entries, entry)
	return nil
}

type metricCollector struct {
	mu      sync.Mutex
	samples []domain.MetricSample
}

func (s *metricCollector) WriteMetric(_ context.Context, sample domain.MetricSample) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.samples = append(s.samples, sample)
	return nil
}
