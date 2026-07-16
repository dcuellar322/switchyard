package application

import (
	"context"
	"testing"
	"time"
)

type hostObserverStub struct{ observation HostObservation }

func (s hostObserverStub) Observe(context.Context) HostObservation { return s.observation }

func TestHostQueryReturnsAdapterObservation(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	query := NewHostQuery(hostObserverStub{observation: HostObservation{CPUPercent: 24, MemoryTotalBytes: 32 << 30, ObservedAt: at}})
	got := query.Get(context.Background())
	if got.CPUPercent != 24 || got.MemoryTotalBytes != 32<<30 || got.ObservedAt != at {
		t.Fatalf("Get() = %#v", got)
	}
}
