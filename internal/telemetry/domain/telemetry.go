// Package domain defines the opt-in anonymous metrics contract.
package domain

import "time"

// SchemaVersion identifies the complete anonymous metrics payload contract.
const SchemaVersion = "switchyard.telemetry/v1"

// Settings records explicit local consent and the user-supplied destination.
type Settings struct {
	Enabled        bool      `json:"enabled"`
	Endpoint       string    `json:"endpoint,omitempty"`
	InstallationID string    `json:"installationId,omitempty"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// Counter is one aggregate from the fixed non-identifying vocabulary.
type Counter struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
}

// Status contains consent, pending counters, exact preview, and delivery state.
type Status struct {
	Settings   Settings   `json:"settings"`
	Counters   []Counter  `json:"counters"`
	Preview    *Payload   `json:"preview,omitempty"`
	LastSentAt *time.Time `json:"lastSentAt,omitempty"`
	LastError  string     `json:"lastError,omitempty"`
}

// Payload is the complete bounded document sent after explicit opt-in.
type Payload struct {
	SchemaVersion  string    `json:"schemaVersion"`
	InstallationID string    `json:"installationId"`
	Version        string    `json:"version"`
	OS             string    `json:"os"`
	Architecture   string    `json:"architecture"`
	Counters       []Counter `json:"counters"`
	GeneratedAt    time.Time `json:"generatedAt"`
}

// AuditEvent records local consent changes without payload or project data.
type AuditEvent struct {
	Type, ActorType, ActorID, Detail string
	OccurredAt                       time.Time
}
