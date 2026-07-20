package adapters

import (
	"context"
	"errors"
	"testing"
	"time"

	manifest "switchyard.dev/switchyard/internal/manifest/domain"
	observability "switchyard.dev/switchyard/internal/observability/domain"
	runtime "switchyard.dev/switchyard/internal/runtime/domain"
)

func TestResourceBudgetRejectsInvalidSignedThresholds(t *testing.T) {
	t.Parallel()
	for _, warnings := range []manifest.ResourceWarnings{{MemoryMiB: -1}, {StorageGiB: -1}} {
		if _, err := resourceBudget(warnings); err == nil {
			t.Fatalf("resourceBudget(%#v) accepted an invalid signed threshold", warnings)
		}
	}
	budget, err := resourceBudget(manifest.ResourceWarnings{MemoryMiB: 512, StorageGiB: 10, CPUPercent: 80})
	if err != nil || budget.MemoryBytes != 512<<20 || budget.StorageBytes != 10<<30 || budget.CPUPercent != 80 {
		t.Fatalf("resourceBudget(valid) = %#v, %v", budget, err)
	}
}

func TestResourceRuntimeSourceSkipsMetricsAndHealthForStoppedProject(t *testing.T) {
	t.Parallel()
	runtimeSource := &runtimeMetricsFake{observation: runtime.Observation{ProjectID: "project", Driver: runtime.KindProcess, State: runtime.StateStopped}}
	health := &latestHealthFake{results: []observability.HealthResult{{ProjectID: "project", ServiceID: "api", LatencyMS: 25}}}
	source := &ResourceRuntimeSource{runtime: runtimeSource, health: health}

	snapshot, err := source.ObserveResources(context.Background(), observability.ProjectDescriptor{ID: "project", Driver: "process"})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Active || snapshot.State != string(runtime.StateStopped) || len(snapshot.Samples) != 0 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	if runtimeSource.metricCalls != 0 || health.calls != 0 {
		t.Fatalf("inactive calls metrics/health = %d/%d, want 0/0", runtimeSource.metricCalls, health.calls)
	}
}

func TestResourceRuntimeSourceCombinesActiveRuntimeMetricsAndLatestHealth(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	runtimeSource := &runtimeMetricsFake{
		observation: runtime.Observation{ProjectID: "project", Driver: runtime.KindCompose, State: runtime.StateRunning},
		samples: []runtime.MetricSample{{
			Timestamp: now, ProjectID: "project", ServiceID: "api", CPUPercent: 12.5,
			CPUAvailable: true, MemoryBytes: 512, MemoryLimit: 1024, MemoryAvailable: true, NetworkRxBytes: 10, NetworkTxBytes: 20, NetworkAvailable: true,
			DiskReadBytes: 30, DiskWriteBytes: 40, DiskAvailable: true, ProcessCount: 2, RestartCount: 1,
		}},
	}
	health := &latestHealthFake{results: []observability.HealthResult{
		{ProjectID: "project", ServiceID: "api", LatencyMS: 12},
		{ProjectID: "project", ServiceID: "api", LatencyMS: 27},
	}}
	source := &ResourceRuntimeSource{runtime: runtimeSource, health: health}

	snapshot, err := source.ObserveResources(context.Background(), observability.ProjectDescriptor{ID: "project", Driver: "compose"})
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.Active || snapshot.Partial || len(snapshot.Samples) != 1 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	point := snapshot.Samples[0]
	if point.CPUPercent != 12.5 || !point.CPUAvailable || point.MemoryBytes != 512 || !point.MemoryAvailable || point.HealthLatencyMS != 27 || !point.HealthAvailable || !point.NetworkAvailable || !point.DiskAvailable {
		t.Fatalf("metric point = %#v", point)
	}
	if runtimeSource.metricCalls != 1 || health.calls != 1 {
		t.Fatalf("active calls metrics/health = %d/%d, want 1/1", runtimeSource.metricCalls, health.calls)
	}
}

func TestResourceRuntimeSourceMarksHealthAndDriverPartialEvidence(t *testing.T) {
	t.Parallel()
	runtimeSource := &runtimeMetricsFake{
		observation: runtime.Observation{ProjectID: "project", Driver: runtime.KindCompose, State: runtime.StateDegraded},
		samples:     []runtime.MetricSample{{ProjectID: "project", ServiceID: "api", Partial: true}},
	}
	source := &ResourceRuntimeSource{runtime: runtimeSource, health: &latestHealthFake{err: errors.New("health database unavailable")}}

	snapshot, err := source.ObserveResources(context.Background(), observability.ProjectDescriptor{ID: "project"})
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.Partial || len(snapshot.Warnings) != 1 || snapshot.Warnings[0] != "Health latency is unavailable." || !snapshot.Samples[0].Partial {
		t.Fatalf("partial snapshot = %#v", snapshot)
	}
}

func TestResourceRuntimeSourceReturnsObservationAndMetricErrors(t *testing.T) {
	t.Parallel()
	inspectErr := errors.New("inspect failed")
	source := &ResourceRuntimeSource{runtime: &runtimeMetricsFake{inspectErr: inspectErr}}
	if _, err := source.ObserveResources(context.Background(), observability.ProjectDescriptor{ID: "project"}); !errors.Is(err, inspectErr) {
		t.Fatalf("inspection error = %v", err)
	}
	metricErr := errors.New("stats failed")
	source = &ResourceRuntimeSource{runtime: &runtimeMetricsFake{
		observation: runtime.Observation{ProjectID: "project", Driver: runtime.KindProcess, State: runtime.StateRunning}, metricErr: metricErr,
	}}
	if _, err := source.ObserveResources(context.Background(), observability.ProjectDescriptor{ID: "project"}); !errors.Is(err, metricErr) {
		t.Fatalf("metrics error = %v", err)
	}
}

func TestActiveRuntimeStatesAreExplicit(t *testing.T) {
	t.Parallel()
	active := []runtime.ProjectState{runtime.StateStarting, runtime.StateRunning, runtime.StateRunningExternal, runtime.StatePartiallyRunning, runtime.StateDegraded, runtime.StatePaused}
	for _, state := range active {
		if !activeRuntime(state) {
			t.Errorf("activeRuntime(%q) = false", state)
		}
	}
	inactive := []runtime.ProjectState{runtime.StateUnknown, runtime.StateStopped, runtime.StateStopping, runtime.StateFailed}
	for _, state := range inactive {
		if activeRuntime(state) {
			t.Errorf("activeRuntime(%q) = true", state)
		}
	}
}

type runtimeMetricsFake struct {
	observation runtime.Observation
	inspectErr  error
	samples     []runtime.MetricSample
	metricErr   error
	metricCalls int
}

func (f *runtimeMetricsFake) Inspect(context.Context, string) (runtime.Observation, error) {
	return f.observation, f.inspectErr
}

func (f *runtimeMetricsFake) Metrics(context.Context, string, string) ([]runtime.MetricSample, error) {
	f.metricCalls++
	return append([]runtime.MetricSample(nil), f.samples...), f.metricErr
}

type latestHealthFake struct {
	results []observability.HealthResult
	err     error
	calls   int
}

func (f *latestHealthFake) LatestResults(context.Context, string) ([]observability.HealthResult, error) {
	f.calls++
	return append([]observability.HealthResult(nil), f.results...), f.err
}
