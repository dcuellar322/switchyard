package application

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/observability/domain"
)

type resourceProjectSourceFake struct {
	projects []domain.ProjectDescriptor
	err      error
}

func (f resourceProjectSourceFake) ListResourceProjects(context.Context) ([]domain.ProjectDescriptor, error) {
	return append([]domain.ProjectDescriptor(nil), f.projects...), f.err
}

type resourceRuntimeFake struct {
	observe func(context.Context, domain.ProjectDescriptor) (domain.RuntimeSnapshot, error)
	calls   atomic.Int64
	active  atomic.Int64
	maximum atomic.Int64
}

func (f *resourceRuntimeFake) ObserveResources(ctx context.Context, project domain.ProjectDescriptor) (domain.RuntimeSnapshot, error) {
	f.calls.Add(1)
	active := f.active.Add(1)
	defer f.active.Add(-1)
	for {
		maximum := f.maximum.Load()
		if active <= maximum || f.maximum.CompareAndSwap(maximum, active) {
			break
		}
	}
	if f.observe == nil {
		return domain.RuntimeSnapshot{ProjectID: project.ID, State: "stopped"}, nil
	}
	return f.observe(ctx, project)
}

type metricRepositoryFake struct {
	mu                sync.Mutex
	writes            []domain.MetricPoint
	latest            map[string][]domain.MetricPoint
	recent            map[string][]domain.MetricPoint
	history           []domain.MetricPoint
	footprint         domain.Footprint
	historyResolution int
	historyLimit      int
	maintainCalls     atomic.Int64
}

func (f *metricRepositoryFake) WriteMetricPoints(_ context.Context, points []domain.MetricPoint) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes = append(f.writes, points...)
	return nil
}

func (f *metricRepositoryFake) LatestMetricPoints(context.Context, []string) (map[string][]domain.MetricPoint, error) {
	return f.latest, nil
}

func (f *metricRepositoryFake) RecentProjectMetricPoints(context.Context, []string, int) (map[string][]domain.MetricPoint, error) {
	return f.recent, nil
}

func (f *metricRepositoryFake) MetricHistory(_ context.Context, _, _ string, _, _ time.Time, resolution, limit int) ([]domain.MetricPoint, error) {
	f.historyResolution, f.historyLimit = resolution, limit
	return append([]domain.MetricPoint(nil), f.history...), nil
}

func (f *metricRepositoryFake) MaintainMetricHistory(context.Context, time.Time, ResourceConfig) error {
	f.maintainCalls.Add(1)
	return nil
}

func (f *metricRepositoryFake) ResourceFootprint(context.Context) (domain.Footprint, error) {
	return f.footprint, nil
}

func (f *metricRepositoryFake) writtenPoints() []domain.MetricPoint {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.MetricPoint(nil), f.writes...)
}

type storageInspectorFake struct {
	inventory domain.StorageInventory
	err       error
}

func (f *storageInspectorFake) InspectStorage(context.Context, []domain.ProjectDescriptor) (domain.StorageInventory, error) {
	return f.inventory, f.err
}

func newResourceServiceForTest(t testing.TB, projects ResourceProjectSource, runtime ResourceRuntimeSource, metrics MetricRepository, storage StorageInspector, override ResourceConfig) *ResourceService {
	t.Helper()
	service, err := NewResourceService(projects, runtime, metrics, storage, resourceTestConfig(override))
	if err != nil {
		t.Fatalf("NewResourceService() error = %v", err)
	}
	return service
}

func resourceTestConfig(override ResourceConfig) ResourceConfig {
	config := ResourceConfig{
		SampleInterval: time.Second, ProjectTimeout: time.Second,
		RawRetention: time.Hour, MinuteRetention: 2 * time.Hour, QuarterHourRetention: 4 * time.Hour,
		MaximumHistoryPoints: 100, MaximumParallelProjects: 4, SustainedSamples: 3,
	}
	if override.MaximumParallelProjects != 0 {
		config.MaximumParallelProjects = override.MaximumParallelProjects
	}
	if override.MaximumHistoryPoints != 0 {
		config.MaximumHistoryPoints = override.MaximumHistoryPoints
	}
	if override.SustainedSamples != 0 {
		config.SustainedSamples = override.SustainedSamples
	}
	return config
}

func findMetricPoint(t testing.TB, points []domain.MetricPoint, projectID, serviceID string) domain.MetricPoint {
	t.Helper()
	for _, point := range points {
		if point.ProjectID == projectID && point.ServiceID == serviceID {
			return point
		}
	}
	t.Fatalf("metric point %s/%s not found in %#v", projectID, serviceID, points)
	return domain.MetricPoint{}
}
