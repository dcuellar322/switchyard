package application

import (
	"context"
	"time"
)

// HostObservation is one current, explicitly partial view of host capacity.
type HostObservation struct {
	CPUPercent       float64           `json:"cpuPercent"`
	MemoryUsedBytes  uint64            `json:"memoryUsedBytes"`
	MemoryTotalBytes uint64            `json:"memoryTotalBytes"`
	Docker           DockerObservation `json:"docker"`
	ObservedAt       time.Time         `json:"observedAt"`
	Warnings         []string          `json:"warnings"`
}

// DockerObservation reports aggregate shared Engine storage without project attribution.
type DockerObservation struct {
	Connected        bool   `json:"connected"`
	StorageBytes     *int64 `json:"storageBytes,omitempty"`
	ReclaimableBytes *int64 `json:"reclaimableBytes,omitempty"`
	Attribution      string `json:"attribution"`
}

// HostObserver reads operating-system and Docker capacity without product policy.
type HostObserver interface {
	Observe(context.Context) HostObservation
}

// HostQuery exposes current host capacity to local clients.
type HostQuery struct{ observer HostObserver }

// NewHostQuery creates the host observation use case.
func NewHostQuery(observer HostObserver) *HostQuery { return &HostQuery{observer: observer} }

// Get returns a fresh host observation.
func (q *HostQuery) Get(ctx context.Context) HostObservation { return q.observer.Observe(ctx) }
