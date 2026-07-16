package host

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/mem"
)

func TestObserverKeepsPartialHostDataHonest(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	observer := &Observer{
		cpu: func(context.Context, time.Duration, bool) ([]float64, error) { return []float64{22.5}, nil },
		memory: func(context.Context) (*mem.VirtualMemoryStat, error) {
			return &mem.VirtualMemoryStat{Used: 12 << 30, Total: 32 << 30}, nil
		},
		docker: func(context.Context) (int64, int64, error) { return 0, 0, errors.New("offline") },
		now:    func() time.Time { return at },
	}
	got := observer.Observe(context.Background())
	if got.CPUPercent != 22.5 || got.MemoryUsedBytes != 12<<30 || got.Docker.Connected || got.Docker.Attribution != "unknown" {
		t.Fatalf("observation = %#v", got)
	}
	if len(got.Warnings) != 1 || got.ObservedAt != at {
		t.Fatalf("warnings/time = %#v %s", got.Warnings, got.ObservedAt)
	}
}

func TestObserverLabelsDockerTotalsAsShared(t *testing.T) {
	t.Parallel()
	observer := &Observer{
		cpu:    func(context.Context, time.Duration, bool) ([]float64, error) { return nil, errors.New("offline") },
		memory: func(context.Context) (*mem.VirtualMemoryStat, error) { return nil, errors.New("offline") },
		docker: func(context.Context) (int64, int64, error) { return 42, 7, nil },
		now:    time.Now,
	}
	got := observer.Observe(context.Background())
	if !got.Docker.Connected || got.Docker.StorageBytes == nil || *got.Docker.StorageBytes != 42 || got.Docker.Attribution != "shared" {
		t.Fatalf("docker = %#v", got.Docker)
	}
}

func TestObserverCoalescesHostInspectionWithinShortCacheWindow(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	calls := 0
	observer := &Observer{
		cpu: func(context.Context, time.Duration, bool) ([]float64, error) { return []float64{10}, nil },
		memory: func(context.Context) (*mem.VirtualMemoryStat, error) {
			return &mem.VirtualMemoryStat{Used: 1, Total: 2}, nil
		},
		docker: func(context.Context) (int64, int64, error) { calls++; return 3, 1, nil },
		now:    func() time.Time { return at },
	}
	first := observer.Observe(context.Background())
	first.Warnings = append(first.Warnings, "caller mutation")
	second := observer.Observe(context.Background())
	if calls != 1 || len(second.Warnings) != 0 {
		t.Fatalf("calls = %d, cached warnings = %#v", calls, second.Warnings)
	}
}
