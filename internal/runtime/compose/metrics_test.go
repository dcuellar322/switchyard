package compose

import (
	"testing"

	"github.com/moby/moby/api/types/container"
)

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
