package compose

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"switchyard.dev/switchyard/internal/runtime/domain"
)

func (d *Driver) streamMetrics(ctx context.Context, request domain.MetricRequest, config normalizedConfig, sink domain.MetricSink) error {
	engine, _, _, err := d.engine.Connect(ctx, config.Connection)
	if err != nil {
		return err
	}
	defer func() { _ = engine.Close() }()
	containers, err := engine.ContainerList(ctx, client.ContainerListOptions{Filters: logFilters(config.ProjectName, request.Service)})
	if err != nil {
		return fmt.Errorf("list Compose containers for metrics: %w", err)
	}
	items := composeContainers(containers.Items, config.ProjectName, config.Services)
	sampled := 0
	var lastErr error
	for _, item := range items {
		if err := sampleContainer(ctx, engine, request.Project, item, sink); err != nil {
			lastErr = err
			if writeErr := sink.WriteMetric(ctx, domain.MetricSample{
				Timestamp: time.Now().UTC(), ProjectID: request.Project.ProjectID,
				ServiceID: productServiceID(request.Project, item.Labels[labelService]), InstanceID: item.ID, Partial: true,
			}); writeErr != nil {
				return writeErr
			}
			continue
		}
		sampled++
	}
	if sampled == 0 && len(items) > 0 {
		return lastErr
	}
	return nil
}

func sampleContainer(ctx context.Context, engine engineClient, project domain.ProjectRuntime, item container.Summary, sink domain.MetricSink) error {
	result, err := engine.ContainerStats(ctx, item.ID, client.ContainerStatsOptions{IncludePreviousSample: true})
	if err != nil {
		return fmt.Errorf("read Compose container metrics: %w", err)
	}
	defer func() { _ = result.Body.Close() }()
	var stats container.StatsResponse
	if err := json.NewDecoder(result.Body).Decode(&stats); err != nil {
		return fmt.Errorf("decode Compose container metrics: %w", err)
	}
	timestamp := stats.Read.UTC()
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}
	rx, tx := networkTotals(stats.Networks)
	read, written, diskAvailable := diskTotals(stats)
	restartCount, partial := 0, false
	inspect, inspectErr := engine.ContainerInspect(ctx, item.ID, client.ContainerInspectOptions{})
	if inspectErr == nil {
		restartCount = inspect.Container.RestartCount
	} else {
		partial = true
	}
	return sink.WriteMetric(ctx, domain.MetricSample{
		Timestamp: timestamp, ProjectID: project.ProjectID,
		ServiceID: productServiceID(project, item.Labels[labelService]), InstanceID: item.ID,
		CPUPercent: cpuPercent(stats), CPUAvailable: cpuStatsAvailable(stats), MemoryBytes: stats.MemoryStats.Usage,
		MemoryLimit: stats.MemoryStats.Limit, MemoryAvailable: true, NetworkRxBytes: rx, NetworkTxBytes: tx, NetworkAvailable: stats.Networks != nil,
		DiskReadBytes: read, DiskWriteBytes: written, DiskAvailable: diskAvailable,
		ProcessCount: max(1, boundedProcessCount(stats.PidsStats.Current)), RestartCount: restartCount, Partial: partial,
	})
}

func boundedProcessCount(value uint64) int {
	if value > uint64(math.MaxInt) {
		return math.MaxInt
	}
	return int(value)
}

func cpuStatsAvailable(stats container.StatsResponse) bool {
	return stats.CPUStats.CPUUsage.TotalUsage >= stats.PreCPUStats.CPUUsage.TotalUsage &&
		stats.CPUStats.SystemUsage > stats.PreCPUStats.SystemUsage
}

func cpuPercent(stats container.StatsResponse) float64 {
	cpuCurrent, cpuPrevious := stats.CPUStats.CPUUsage.TotalUsage, stats.PreCPUStats.CPUUsage.TotalUsage
	systemCurrent, systemPrevious := stats.CPUStats.SystemUsage, stats.PreCPUStats.SystemUsage
	if cpuCurrent <= cpuPrevious || systemCurrent <= systemPrevious {
		return 0
	}
	cpuDelta := float64(cpuCurrent - cpuPrevious)
	systemDelta := float64(systemCurrent - systemPrevious)
	processors := float64(stats.CPUStats.OnlineCPUs)
	if processors == 0 {
		processors = float64(len(stats.CPUStats.CPUUsage.PercpuUsage))
	}
	if processors == 0 {
		processors = 1
	}
	return cpuDelta / systemDelta * processors * 100
}

func networkTotals(networks map[string]container.NetworkStats) (uint64, uint64) {
	var rx, tx uint64
	for _, network := range networks {
		rx += network.RxBytes
		tx += network.TxBytes
	}
	return rx, tx
}

func diskTotals(stats container.StatsResponse) (uint64, uint64, bool) {
	read, written := stats.StorageStats.ReadSizeBytes, stats.StorageStats.WriteSizeBytes
	available := read > 0 || written > 0
	for _, entry := range stats.BlkioStats.IoServiceBytesRecursive {
		switch entry.Op {
		case "Read", "read":
			read += entry.Value
		case "Write", "write":
			written += entry.Value
		}
		available = true
	}
	return read, written, available
}
