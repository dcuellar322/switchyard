package application

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/observability/domain"
)

func TestCollectAggregatesServicesAndProjectsWithBoundedConcurrency(t *testing.T) {
	t.Parallel()
	projects := make([]domain.ProjectDescriptor, 12)
	for index := range projects {
		projects[index] = domain.ProjectDescriptor{ID: fmt.Sprintf("project-%02d", index), Name: fmt.Sprintf("Project %02d", index), Driver: "process"}
	}
	runtime := &resourceRuntimeFake{observe: func(_ context.Context, project domain.ProjectDescriptor) (domain.RuntimeSnapshot, error) {
		time.Sleep(5 * time.Millisecond)
		return domain.RuntimeSnapshot{
			ProjectID: project.ID, Driver: project.Driver, State: "running", Active: true,
			Samples: []domain.MetricPoint{
				{ServiceID: "api", CPUPercent: 10, CPUAvailable: true, MemoryBytes: 100, MemoryAvailable: true, NetworkRxBytes: 3, NetworkAvailable: true, ProcessCount: 1},
				{ServiceID: "api", CPUPercent: 20, CPUAvailable: true, MemoryBytes: 200, MemoryAvailable: true, DiskWriteBytes: 5, DiskAvailable: true, ProcessCount: 2, RestartCount: 1},
				{ServiceID: "web", CPUPercent: 5, CPUAvailable: true, MemoryBytes: 50, MemoryAvailable: true, ProcessCount: 1},
			},
		}, nil
	}}
	metrics := &metricRepositoryFake{}
	storage := &storageInspectorFake{inventory: domain.StorageInventory{Connected: true, Projects: []domain.ProjectStorage{{
		ProjectID: "project-00", Summary: domain.StorageSummary{Bytes: 4096, Classification: domain.StorageEstimated},
	}}}}
	service := newResourceServiceForTest(t, resourceProjectSourceFake{projects: projects}, runtime, metrics, storage, ResourceConfig{MaximumParallelProjects: 4})
	observedAt := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return observedAt }

	if err := service.Collect(context.Background()); err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if got := runtime.maximum.Load(); got != 4 {
		t.Fatalf("maximum concurrent project samples = %d, want 4", got)
	}
	if got := runtime.calls.Load(); got != int64(len(projects)) {
		t.Fatalf("runtime sample calls = %d, want %d", got, len(projects))
	}
	points := metrics.writtenPoints()
	if len(points) != len(projects)*3 {
		t.Fatalf("written points = %d, want %d", len(points), len(projects)*3)
	}
	project := findMetricPoint(t, points, "project-00", "")
	if project.CPUPercent != 35 || project.MemoryBytes != 350 || project.ProcessCount != 4 || project.RestartCount != 1 {
		t.Fatalf("project aggregate = %#v", project)
	}
	if project.StorageBytes == nil || *project.StorageBytes != 4096 || project.StorageClassification != domain.StorageEstimated {
		t.Fatalf("project storage = %#v/%s", project.StorageBytes, project.StorageClassification)
	}
	api := findMetricPoint(t, points, "project-00", "api")
	if api.CPUPercent != 30 || api.CPUMaxPercent != 30 || api.MemoryBytes != 300 || !api.NetworkAvailable || !api.DiskAvailable {
		t.Fatalf("api aggregate = %#v", api)
	}
	if metrics.maintainCalls.Load() != 1 {
		t.Fatalf("retention maintenance calls = %d, want 1", metrics.maintainCalls.Load())
	}
}

func TestCollectStopsPromptlyWhenCallerCancels(t *testing.T) {
	t.Parallel()
	started := make(chan struct{}, 2)
	runtime := &resourceRuntimeFake{observe: func(ctx context.Context, project domain.ProjectDescriptor) (domain.RuntimeSnapshot, error) {
		started <- struct{}{}
		<-ctx.Done()
		return domain.RuntimeSnapshot{ProjectID: project.ID}, ctx.Err()
	}}
	metrics := &metricRepositoryFake{}
	service := newResourceServiceForTest(t,
		resourceProjectSourceFake{projects: []domain.ProjectDescriptor{{ID: "one", Name: "One"}, {ID: "two", Name: "Two"}}},
		runtime, metrics, &storageInspectorFake{}, ResourceConfig{MaximumParallelProjects: 2},
	)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- service.Collect(ctx) }()
	<-started
	<-started
	cancel()
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("Collect() error = %v", err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("Collect() did not return promptly after cancellation")
	}
	if runtime.active.Load() != 0 {
		t.Fatalf("active samples after cancellation = %d", runtime.active.Load())
	}
}

func TestCollectRecordsPartialPointAndWarningOnProjectFailure(t *testing.T) {
	t.Parallel()
	metrics := &metricRepositoryFake{}
	service := newResourceServiceForTest(t,
		resourceProjectSourceFake{projects: []domain.ProjectDescriptor{{ID: "broken", Name: "Broken", Driver: "compose"}}},
		&resourceRuntimeFake{observe: func(context.Context, domain.ProjectDescriptor) (domain.RuntimeSnapshot, error) {
			return domain.RuntimeSnapshot{}, errors.New("engine unavailable")
		}},
		metrics, &storageInspectorFake{}, ResourceConfig{},
	)
	if err := service.Collect(context.Background()); err != nil {
		t.Fatal(err)
	}
	point := findMetricPoint(t, metrics.writtenPoints(), "broken", "")
	if !point.Partial {
		t.Fatalf("failure point = %#v, want partial", point)
	}
	service.stateMu.RLock()
	warnings := append([]string(nil), service.warnings...)
	service.stateMu.RUnlock()
	if len(warnings) != 2 || warnings[0] != "Broken resource observation is partial." || warnings[1] != "Broken resource sampling failed." {
		t.Fatalf("warnings = %#v", warnings)
	}
}

func TestCollectDoesNotPersistStoppedProjectsAndThrottlesIdleMaintenance(t *testing.T) {
	t.Parallel()
	projects := make([]domain.ProjectDescriptor, 50)
	for index := range projects {
		projects[index] = domain.ProjectDescriptor{ID: fmt.Sprintf("idle-%02d", index), Name: "Idle", Driver: "process"}
	}
	runtime := &resourceRuntimeFake{observe: func(_ context.Context, project domain.ProjectDescriptor) (domain.RuntimeSnapshot, error) {
		return domain.RuntimeSnapshot{ProjectID: project.ID, Driver: project.Driver, State: "stopped", Active: false}, nil
	}}
	metrics := &metricRepositoryFake{}
	service := newResourceServiceForTest(t, resourceProjectSourceFake{projects: projects}, runtime, metrics, &storageInspectorFake{}, ResourceConfig{})
	clock := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return clock }
	if err := service.Collect(context.Background()); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(time.Minute)
	if err := service.Collect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := len(metrics.writtenPoints()); got != 0 {
		t.Fatalf("stopped project metric points = %d, want 0", got)
	}
	if got := metrics.maintainCalls.Load(); got != 1 {
		t.Fatalf("maintenance calls after one idle minute = %d, want 1", got)
	}
	clock = clock.Add(14 * time.Minute)
	if err := service.Collect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := metrics.maintainCalls.Load(); got != 2 {
		t.Fatalf("maintenance calls after fifteen idle minutes = %d, want 2", got)
	}
}

func TestEvaluateBudgetRequiresConsecutiveFreshSamples(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	storage := int64(2_000)
	points := []domain.MetricPoint{
		{Timestamp: now, CPUPercent: 90, CPUAvailable: true, MemoryBytes: 900, MemoryAvailable: true, StorageBytes: &storage},
		{Timestamp: now.Add(-time.Second), CPUPercent: 80, CPUAvailable: true, MemoryBytes: 800, MemoryAvailable: true, StorageBytes: &storage},
		{Timestamp: now.Add(-2 * time.Second), CPUPercent: 70, CPUAvailable: true, MemoryBytes: 700, MemoryAvailable: true, StorageBytes: &storage},
	}
	config := resourceTestConfig(ResourceConfig{SustainedSamples: 3})
	warnings := evaluateBudget(domain.ResourceBudget{CPUPercent: 60, MemoryBytes: 600, StorageBytes: 1_000}, points, config)
	if len(warnings) != 3 {
		t.Fatalf("budget warnings = %#v, want CPU, memory, and storage", warnings)
	}
	if warnings[0].Samples != 3 || !warnings[0].SustainedFrom.Equal(points[2].Timestamp) {
		t.Fatalf("budget warning evidence = %#v", warnings[0])
	}
	points[1].CPUPercent = 1
	warnings = evaluateBudget(domain.ResourceBudget{CPUPercent: 60}, points, config)
	if len(warnings) != 0 {
		t.Fatalf("non-consecutive warnings = %#v", warnings)
	}
	points[1].CPUPercent = 80
	points[2].Timestamp = now.Add(-2 * time.Minute)
	warnings = evaluateBudget(domain.ResourceBudget{CPUPercent: 60}, points, config)
	if len(warnings) != 0 {
		t.Fatalf("stale warnings = %#v", warnings)
	}
}

func TestHistoryChoosesBoundedTierAndRejectsInvalidWindows(t *testing.T) {
	t.Parallel()
	metrics := &metricRepositoryFake{history: []domain.MetricPoint{{ProjectID: "project", ServiceID: "api"}}}
	service := newResourceServiceForTest(t, resourceProjectSourceFake{}, &resourceRuntimeFake{}, metrics, &storageInspectorFake{}, ResourceConfig{MaximumHistoryPoints: 100})
	to := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)

	history, err := service.History(context.Background(), "project", "api", "auto", to.Add(-30*time.Minute), to, 100)
	if err != nil {
		t.Fatal(err)
	}
	if history.ResolutionSeconds != 60 || metrics.historyResolution != 60 || metrics.historyLimit != 100 {
		t.Fatalf("history = %#v, repository resolution/limit = %d/%d", history, metrics.historyResolution, metrics.historyLimit)
	}
	history, err = service.History(context.Background(), "project", "", "auto", to.Add(-3*time.Hour), to, 10)
	if err != nil {
		t.Fatal(err)
	}
	if history.ResolutionSeconds != 900 {
		t.Fatalf("three-hour resolution = %d, want 900", history.ResolutionSeconds)
	}
	invalid := []struct {
		project, resolution string
		from, to            time.Time
		max                 int
	}{
		{"", "auto", to.Add(-time.Minute), to, 10},
		{"project", "raw", to.Add(-2 * time.Hour), to, 10},
		{"project", "daily", to.Add(-time.Hour), to, 10},
		{"project", "auto", to, to, 10},
		{"project", "auto", to.Add(-time.Hour), to, 101},
	}
	for _, item := range invalid {
		if _, err := service.History(context.Background(), item.project, "", item.resolution, item.from, item.to, item.max); !errors.Is(err, ErrInvalidResourceQuery) {
			t.Errorf("History(%q, %q) error = %v, want ErrInvalidResourceQuery", item.project, item.resolution, err)
		}
	}
}

func TestCleanupPreviewIsExactFilteredAndNeverExecutable(t *testing.T) {
	t.Parallel()
	known := int64(100)
	inventory := domain.StorageInventory{Connected: true, ObservedAt: time.Now().UTC(), Resources: []domain.StorageResource{
		{Kind: "image", ID: "shared", ProjectIDs: []string{"one", "two"}, Bytes: &known, Reclaimable: true, Classification: domain.StorageShared},
		{Kind: "volume", ID: "unknown", ProjectIDs: []string{"one"}, Reclaimable: true, Classification: domain.StorageUnknown},
		{Kind: "container", ID: "active", ProjectIDs: []string{"one"}, Bytes: &known, Reclaimable: false, Classification: domain.StorageExclusive},
		{Kind: "image", ID: "other", ProjectIDs: []string{"two"}, Bytes: &known, Reclaimable: true, Classification: domain.StorageEstimated},
	}}
	service := newResourceServiceForTest(t, resourceProjectSourceFake{}, &resourceRuntimeFake{}, &metricRepositoryFake{}, &storageInspectorFake{inventory: inventory}, ResourceConfig{})
	preview, err := service.CleanupPreview(context.Background(), "one")
	if err != nil {
		t.Fatal(err)
	}
	if preview.Executable || preview.Risk != "destructive" || preview.EstimatedBytes != 100 || preview.UnknownSizes != 1 {
		t.Fatalf("preview = %#v", preview)
	}
	if len(preview.Resources) != 2 || preview.Resources[0].ID != "shared" || preview.Resources[1].ID != "unknown" {
		t.Fatalf("preview resources = %#v", preview.Resources)
	}
	disconnected := errors.New("storage observation failed")
	service = newResourceServiceForTest(t, resourceProjectSourceFake{}, &resourceRuntimeFake{}, &metricRepositoryFake{}, &storageInspectorFake{err: disconnected}, ResourceConfig{})
	if _, err := service.CleanupPreview(context.Background(), "one"); !errors.Is(err, disconnected) {
		t.Fatalf("disconnected cleanup preview error = %v", err)
	}
}

func TestRunExitsWithoutReportingCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reported := make(chan error, 1)
	service := newResourceServiceForTest(t,
		resourceProjectSourceFake{err: context.Canceled}, &resourceRuntimeFake{}, &metricRepositoryFake{}, &storageInspectorFake{}, ResourceConfig{},
	)
	done := make(chan struct{})
	go func() {
		service.Run(ctx, func(err error) { reported <- err })
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("Run() did not exit for a canceled context")
	}
	select {
	case err := <-reported:
		t.Fatalf("reported cancellation = %v", err)
	default:
	}
}

func BenchmarkResourceSamplerFiftyIdleProjects(b *testing.B) {
	projects := make([]domain.ProjectDescriptor, 50)
	for index := range projects {
		projects[index] = domain.ProjectDescriptor{ID: fmt.Sprintf("idle-%02d", index), Name: "Idle", Driver: "process"}
	}
	service, err := NewResourceService(
		resourceProjectSourceFake{projects: projects},
		&resourceRuntimeFake{observe: func(_ context.Context, project domain.ProjectDescriptor) (domain.RuntimeSnapshot, error) {
			return domain.RuntimeSnapshot{ProjectID: project.ID, Driver: project.Driver, State: "stopped"}, nil
		}},
		&metricRepositoryFake{}, &storageInspectorFake{}, resourceTestConfig(ResourceConfig{MaximumParallelProjects: 4}),
	)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := service.Collect(context.Background()); err != nil {
			b.Fatal(err)
		}
	}
}
