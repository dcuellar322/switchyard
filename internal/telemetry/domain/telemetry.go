// Package domain defines the opt-in anonymous metrics contract.
package domain

import "time"

const SchemaVersion = "switchyard.telemetry/v1"

type Settings struct {
	Enabled        bool      `json:"enabled"`
	Endpoint       string    `json:"endpoint,omitempty"`
	InstallationID string    `json:"installationId,omitempty"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type Counter struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
}

type Status struct {
	Settings   Settings   `json:"settings"`
	Counters   []Counter  `json:"counters"`
	Preview    *Payload   `json:"preview,omitempty"`
	LastSentAt *time.Time `json:"lastSentAt,omitempty"`
	LastError  string     `json:"lastError,omitempty"`
}

type Payload struct {
	SchemaVersion  string    `json:"schemaVersion"`
	InstallationID string    `json:"installationId"`
	Version        string    `json:"version"`
	OS             string    `json:"os"`
	Architecture   string    `json:"architecture"`
	Counters       []Counter `json:"counters"`
	GeneratedAt    time.Time `json:"generatedAt"`
}

type AuditEvent struct {
	Type, ActorType, ActorID, Detail string
	OccurredAt                       time.Time
}
