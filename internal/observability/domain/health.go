// Package domain owns provider-neutral health observations and project diagnostics.
package domain

import "time"

// HealthStatus is the result of one bounded check.
type HealthStatus string

const (
	// StatusHealthy means the bounded probe passed.
	StatusHealthy HealthStatus = "healthy"
	// StatusUnhealthy means the bounded probe completed and failed.
	StatusUnhealthy HealthStatus = "unhealthy"
	// StatusUnknown means no current conclusion is safe.
	StatusUnknown HealthStatus = "unknown"
)

// ObserverState distinguishes a current observation from stale or disconnected data.
type ObserverState string

const (
	// ObserverConnected means runtime evidence is current.
	ObserverConnected ObserverState = "connected"
	// ObserverStale means the last result exceeded its freshness window.
	ObserverStale ObserverState = "stale"
	// ObserverDisconnected means the underlying runtime cannot be observed.
	ObserverDisconnected ObserverState = "disconnected"
)

// HealthResult is one immutable, redaction-safe check sample.
type HealthResult struct {
	ProjectID  string       `json:"projectId"`
	ServiceID  string       `json:"serviceId"`
	CheckID    string       `json:"checkId"`
	Type       string       `json:"type"`
	Status     HealthStatus `json:"status"`
	Severity   string       `json:"severity"`
	Required   bool         `json:"required"`
	LatencyMS  int64        `json:"latencyMs"`
	Message    string       `json:"message"`
	ObservedAt time.Time    `json:"observedAt"`
}

// ProjectHealth is the current aggregate diagnostic view.
type ProjectHealth struct {
	ProjectID     string         `json:"projectId"`
	Status        HealthStatus   `json:"status"`
	ObserverState ObserverState  `json:"observerState"`
	Results       []HealthResult `json:"results"`
	ObservedAt    time.Time      `json:"observedAt"`
}

// LogQuery selects a bounded, project-scoped persisted stream.
type LogQuery struct {
	ProjectID   string
	ServiceID   string
	RunID       string
	OperationID string
	Since       time.Time
	After       int64
	Limit       int
}
