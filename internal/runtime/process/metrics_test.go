package process

import (
	"context"
	"errors"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

func TestStreamMetricsAggregatesVerifiedProcessTreeByService(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newMemoryRunStore()
	startedAt := time.Date(2026, 7, 16, 11, 0, 0, 0, time.UTC)
	apiParent := processIdentity(10, 10, "api-parent", startedAt)
	apiChild := processIdentity(11, 10, "api-child", startedAt.Add(time.Second))
	worker := processIdentity(20, 20, "worker", startedAt)
	for _, run := range []domain.RunRecord{
		{ID: "run-api", ProjectID: "project-process", ServiceID: "api", RuntimeDriver: domain.KindProcess, StartedAt: startedAt, RestartCount: 2},
		{ID: "run-worker", ProjectID: "project-process", ServiceID: "worker", RuntimeDriver: domain.KindProcess, StartedAt: startedAt},
	} {
		if err := store.CreateRun(ctx, run); err != nil {
			t.Fatal(err)
		}
	}
	apiParent.RunID, apiChild.RunID, worker.RunID = "run-api", "run-api", "run-worker"
	for _, identity := range []domain.ProcessIdentity{apiParent, apiChild, worker} {
		if err := store.RecordProcess(ctx, identity); err != nil {
			t.Fatal(err)
		}
	}
	inspector := inspectorFake{
		snapshots: map[int32]domain.ProcessIdentity{10: apiParent, 11: apiChild, 20: worker},
		usages: map[int32]processUsage{
			10: {cpuPercent: 5, cpuAvailable: true, memoryBytes: 100, memoryLimit: 1_000, memoryAvailable: true, diskReadBytes: 10, diskWriteBytes: 20, diskAvailable: true},
			11: {cpuPercent: 7, cpuAvailable: true, memoryBytes: 200, memoryLimit: 1_000, memoryAvailable: true, diskReadBytes: 2, diskWriteBytes: 3, diskAvailable: true},
			20: {cpuPercent: 11, cpuAvailable: true, memoryBytes: 300, memoryLimit: 1_000, memoryAvailable: true},
		},
	}
	driver := newDriver(ctx, store, inspector, &secretResolverFake{})
	driver.now = func() time.Time { return startedAt.Add(time.Hour) }
	sink := &metricSinkFake{}
	project := processProject("/repo", []servicePlan{
		{service: domain.ServiceDeclaration{ID: "api"}, definition: domain.ProcessDefinition{ID: "api", Command: []string{"api"}}},
		{service: domain.ServiceDeclaration{ID: "worker"}, definition: domain.ProcessDefinition{ID: "worker", Command: []string{"worker"}}},
	})
	if err := driver.StreamMetrics(ctx, domain.MetricRequest{Project: project}, sink); err != nil {
		t.Fatal(err)
	}
	if len(sink.samples) != 2 || sink.samples[0].ServiceID != "api" || sink.samples[1].ServiceID != "worker" {
		t.Fatalf("samples = %#v", sink.samples)
	}
	api := sink.samples[0]
	if api.CPUPercent != 12 || !api.CPUAvailable || api.MemoryBytes != 300 || !api.MemoryAvailable || api.MemoryLimit != 1_000 || api.DiskReadBytes != 12 || api.DiskWriteBytes != 23 || !api.DiskAvailable || api.ProcessCount != 2 || api.RestartCount != 2 {
		t.Fatalf("api process-tree sample = %#v", api)
	}
	if api.Timestamp != startedAt.Add(time.Hour) || api.ProjectID != "project-process" {
		t.Fatalf("api identity/timestamp = %#v", api)
	}
}

func TestStreamMetricsMarksPartialUsageAndHonorsServiceFilter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newMemoryRunStore()
	startedAt := time.Now().UTC().Add(-time.Minute)
	identities := []domain.ProcessIdentity{
		processIdentity(10, 10, "one", startedAt),
		processIdentity(11, 10, "two", startedAt.Add(time.Second)),
	}
	if err := store.CreateRun(ctx, domain.RunRecord{ID: "run-api", ProjectID: "project-process", ServiceID: "api", RuntimeDriver: domain.KindProcess, StartedAt: startedAt}); err != nil {
		t.Fatal(err)
	}
	for index := range identities {
		identities[index].RunID = "run-api"
		if err := store.RecordProcess(ctx, identities[index]); err != nil {
			t.Fatal(err)
		}
	}
	inspector := inspectorFake{
		snapshots: map[int32]domain.ProcessIdentity{10: identities[0], 11: identities[1]},
		usages:    map[int32]processUsage{10: {cpuPercent: 5, cpuAvailable: true, memoryBytes: 100, memoryAvailable: true}},
		usageErrs: map[int32]error{11: errors.New("process exited while sampling")},
	}
	driver := newDriver(ctx, store, inspector, &secretResolverFake{})
	sink := &metricSinkFake{}
	project := processProject("/repo", []servicePlan{{service: domain.ServiceDeclaration{ID: "api"}, definition: domain.ProcessDefinition{ID: "api", Command: []string{"api"}}}})
	if err := driver.StreamMetrics(ctx, domain.MetricRequest{Project: project, Service: "api"}, sink); err != nil {
		t.Fatal(err)
	}
	if len(sink.samples) != 1 || !sink.samples[0].Partial || sink.samples[0].ProcessCount != 1 || sink.samples[0].MemoryBytes != 100 {
		t.Fatalf("partial sample = %#v", sink.samples)
	}
	sink.samples = nil
	if err := driver.StreamMetrics(ctx, domain.MetricRequest{Project: project, Service: "worker"}, sink); err != nil {
		t.Fatal(err)
	}
	if len(sink.samples) != 0 {
		t.Fatalf("filtered samples = %#v", sink.samples)
	}
}

func TestStreamMetricsRejectsReusedProcessIdentityAndEndedRuns(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := newMemoryRunStore()
	startedAt := time.Now().UTC().Add(-time.Hour)
	stored := processIdentity(10, 10, "stored", startedAt)
	stored.RunID = "stale"
	if err := store.CreateRun(ctx, domain.RunRecord{ID: "stale", ProjectID: "project-process", ServiceID: "api", RuntimeDriver: domain.KindProcess, StartedAt: startedAt}); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordProcess(ctx, stored); err != nil {
		t.Fatal(err)
	}
	endedAt := time.Now().UTC()
	ended := processIdentity(20, 20, "ended", startedAt)
	ended.RunID = "ended"
	if err := store.CreateRun(ctx, domain.RunRecord{ID: "ended", ProjectID: "project-process", ServiceID: "worker", RuntimeDriver: domain.KindProcess, StartedAt: startedAt, EndedAt: &endedAt}); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordProcess(ctx, ended); err != nil {
		t.Fatal(err)
	}
	reused := stored
	reused.Fingerprint = "different"
	driver := newDriver(ctx, store, inspectorFake{snapshots: map[int32]domain.ProcessIdentity{10: reused, 20: ended}}, &secretResolverFake{})
	sink := &metricSinkFake{}
	if err := driver.StreamMetrics(ctx, domain.MetricRequest{Project: processProject("/repo", nil)}, sink); err != nil {
		t.Fatal(err)
	}
	if len(sink.samples) != 0 {
		t.Fatalf("unverified or ended samples = %#v", sink.samples)
	}
}

func processIdentity(pid, group int32, fingerprint string, startedAt time.Time) domain.ProcessIdentity {
	return domain.ProcessIdentity{
		PID: pid, ProcessGroup: group, Executable: "/usr/bin/process", WorkingDirectory: "/repo",
		StartedAt: startedAt, Fingerprint: fingerprint, ObservedAt: startedAt,
	}
}
