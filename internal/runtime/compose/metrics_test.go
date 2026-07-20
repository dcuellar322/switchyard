package compose

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"switchyard.dev/switchyard/internal/runtime/domain"
)

func TestBoundedProcessCountSaturatesWithoutWrapping(t *testing.T) {
	t.Parallel()
	if got := boundedProcessCount(math.MaxUint64); got != math.MaxInt {
		t.Fatalf("boundedProcessCount(MaxUint64) = %d, want %d", got, math.MaxInt)
	}
	if got := boundedProcessCount(4); got != 4 {
		t.Fatalf("boundedProcessCount(4) = %d", got)
	}
}

func TestMetricCalculationsUseCPUAndAllNetworks(t *testing.T) {
	t.Parallel()
	stats := container.StatsResponse{
		CPUStats:    container.CPUStats{CPUUsage: container.CPUUsage{TotalUsage: 300}, SystemUsage: 1_000, OnlineCPUs: 2},
		PreCPUStats: container.CPUStats{CPUUsage: container.CPUUsage{TotalUsage: 100}, SystemUsage: 500},
	}
	if got := cpuPercent(stats); got != 80 {
		t.Fatalf("cpuPercent = %v", got)
	}
	stats.CPUStats.CPUUsage.TotalUsage = 50
	if got := cpuPercent(stats); got != 0 {
		t.Fatalf("reset cpuPercent = %v", got)
	}
	rx, tx := networkTotals(map[string]container.NetworkStats{
		"eth0": {RxBytes: 10, TxBytes: 20}, "eth1": {RxBytes: 5, TxBytes: 7},
	})
	if rx != 15 || tx != 27 {
		t.Fatalf("network totals = %d/%d", rx, tx)
	}
}

func TestDiskTotalsIncludesWindowsAndLinuxCounters(t *testing.T) {
	t.Parallel()
	read, written, available := diskTotals(container.StatsResponse{
		StorageStats: container.StorageStats{ReadSizeBytes: 10, WriteSizeBytes: 20},
		BlkioStats: container.BlkioStats{IoServiceBytesRecursive: []container.BlkioStatEntry{
			{Op: "Read", Value: 3}, {Op: "read", Value: 4}, {Op: "Write", Value: 5}, {Op: "write", Value: 6}, {Op: "Discard", Value: 99},
		}},
	})
	if read != 17 || written != 31 || !available {
		t.Fatalf("disk totals = %d/%d available=%t", read, written, available)
	}
	read, written, available = diskTotals(container.StatsResponse{})
	if read != 0 || written != 0 || available {
		t.Fatalf("empty disk totals = %d/%d available=%t", read, written, available)
	}
}

func TestStreamMetricsSamplesEveryComposeReplicaForServiceAggregation(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	first := containerStatsFixture(now, 100, 200, 1_000, 10, 20, 30, 40, 2)
	second := containerStatsFixture(now, 200, 400, 2_000, 1, 2, 3, 4, 3)
	engine := &fakeEngine{
		containers: []container.Summary{
			{ID: "api-1", Labels: map[string]string{labelProject: "fixture", labelService: "api", labelNumber: "1"}},
			{ID: "api-2", Labels: map[string]string{labelProject: "fixture", labelService: "api", labelNumber: "2"}},
			{ID: "other", Labels: map[string]string{labelProject: "different", labelService: "api"}},
			{ID: "oneoff", Labels: map[string]string{labelProject: "fixture", labelService: "task", labelOneoff: "True"}},
		},
		stats: map[string][]byte{"api-1": mustJSON(t, first), "api-2": mustJSON(t, second)},
		inspects: map[string]container.InspectResponse{
			"api-1": {RestartCount: 1}, "api-2": {RestartCount: 2},
		},
	}
	driver := &Driver{engine: fakeConnector{engine: engine}}
	sink := &composeMetricSink{}
	project := domain.ProjectRuntime{ProjectID: "project", Kind: domain.KindCompose, Services: []domain.ServiceDeclaration{{ID: "backend", RuntimeName: "api"}}}
	if err := driver.streamMetrics(context.Background(), domain.MetricRequest{Project: project}, normalizedConfig{ProjectName: "fixture"}, sink); err != nil {
		t.Fatal(err)
	}
	if len(sink.samples) != 2 {
		t.Fatalf("replica samples = %#v", sink.samples)
	}
	if sink.samples[0].ServiceID != "backend" || sink.samples[1].ServiceID != "backend" || sink.samples[0].InstanceID == sink.samples[1].InstanceID {
		t.Fatalf("replica service identity = %#v", sink.samples)
	}
	var cpu float64
	var memory, read, written uint64
	var processes, restarts int
	for _, sample := range sink.samples {
		cpu += sample.CPUPercent
		memory += sample.MemoryBytes
		read += sample.DiskReadBytes
		written += sample.DiskWriteBytes
		processes += sample.ProcessCount
		restarts += sample.RestartCount
		if !sample.CPUAvailable || !sample.MemoryAvailable || !sample.NetworkAvailable || !sample.DiskAvailable || sample.Partial {
			t.Fatalf("replica availability = %#v", sample)
		}
	}
	if cpu != 200 || memory != 3_000 || read != 33 || written != 44 || processes != 5 || restarts != 3 {
		t.Fatalf("replica totals cpu=%v memory=%d disk=%d/%d processes=%d restarts=%d", cpu, memory, read, written, processes, restarts)
	}
}

func TestStreamMetricsRetainsPartialMarkerWhenOneReplicaFails(t *testing.T) {
	t.Parallel()
	engine := &fakeEngine{
		containers: []container.Summary{
			{ID: "good", Labels: map[string]string{labelProject: "fixture", labelService: "api"}},
			{ID: "bad", Labels: map[string]string{labelProject: "fixture", labelService: "api"}},
		},
		stats:    map[string][]byte{"good": mustJSON(t, containerStatsFixture(time.Now().UTC(), 1, 2, 100, 0, 0, 0, 0, 1))},
		statsErr: map[string]error{"bad": errors.New("stats stream unavailable")},
		inspects: map[string]container.InspectResponse{"good": {}},
	}
	driver := &Driver{engine: fakeConnector{engine: engine}}
	sink := &composeMetricSink{}
	project := domain.ProjectRuntime{ProjectID: "project", Services: []domain.ServiceDeclaration{{ID: "api", RuntimeName: "api"}}}
	if err := driver.streamMetrics(context.Background(), domain.MetricRequest{Project: project}, normalizedConfig{ProjectName: "fixture"}, sink); err != nil {
		t.Fatal(err)
	}
	if len(sink.samples) != 2 {
		t.Fatalf("samples = %#v", sink.samples)
	}
	partial := 0
	for _, sample := range sink.samples {
		if sample.Partial {
			partial++
		}
	}
	if partial != 1 {
		t.Fatalf("partial replica samples = %#v", sink.samples)
	}
}

func containerStatsFixture(readAt time.Time, cpuDelta, systemDelta, memory, rx, tx, diskRead, diskWrite, processes uint64) container.StatsResponse {
	return container.StatsResponse{
		Read:         readAt,
		CPUStats:     container.CPUStats{CPUUsage: container.CPUUsage{TotalUsage: cpuDelta + 100}, SystemUsage: systemDelta + 500, OnlineCPUs: 2},
		PreCPUStats:  container.CPUStats{CPUUsage: container.CPUUsage{TotalUsage: 100}, SystemUsage: 500},
		MemoryStats:  container.MemoryStats{Usage: memory, Limit: 4_000},
		Networks:     map[string]container.NetworkStats{"eth0": {RxBytes: rx, TxBytes: tx}},
		StorageStats: container.StorageStats{ReadSizeBytes: diskRead, WriteSizeBytes: diskWrite},
		PidsStats:    container.PidsStats{Current: processes},
	}
}

func mustJSON(t testing.TB, value any) []byte {
	t.Helper()
	result, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

type composeMetricSink struct{ samples []domain.MetricSample }

func (s *composeMetricSink) WriteMetric(_ context.Context, sample domain.MetricSample) error {
	s.samples = append(s.samples, sample)
	return nil
}
