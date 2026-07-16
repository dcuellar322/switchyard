// Package domain defines plugin trust, health, and audit state without owning
// process or wire-protocol behavior.
package domain

import "time"

// TrustState makes executable identity changes visible before execution.
type TrustState string

const (
	// TrustUntrusted means no current fingerprint was approved.
	TrustUntrusted TrustState = "untrusted"
	// TrustTrusted means the current fingerprint exactly matches approval.
	TrustTrusted TrustState = "trusted"
	// TrustChanged means an approved package identity has changed.
	TrustChanged TrustState = "changed"
)

// HealthState is the last bounded process observation.
type HealthState string

const (
	// HealthUnknown means no current supervised result exists.
	HealthUnknown HealthState = "unknown"
	// HealthHealthy means the plugin passed its last check.
	HealthHealthy HealthState = "healthy"
	// HealthDegraded means the plugin is usable with a reported warning.
	HealthDegraded HealthState = "degraded"
	// HealthUnhealthy means process or protocol supervision failed.
	HealthUnhealthy HealthState = "unhealthy"
)

// Plugin is one discovered external process and its durable user decision.
type Plugin struct {
	ID                 string      `json:"id"`
	Name               string      `json:"name"`
	Version            string      `json:"version"`
	ProtocolVersion    string      `json:"protocolVersion"`
	ManifestPath       string      `json:"manifestPath"`
	Executable         string      `json:"-"`
	Arguments          []string    `json:"-"`
	Fingerprint        string      `json:"fingerprint"`
	TrustedFingerprint string      `json:"-"`
	Capabilities       []string    `json:"capabilities"`
	RequestedScopes    []string    `json:"requestedScopes"`
	GrantedScopes      []string    `json:"grantedScopes"`
	Available          bool        `json:"available"`
	Enabled            bool        `json:"enabled"`
	Trust              TrustState  `json:"trust"`
	Health             HealthState `json:"health"`
	HealthMessage      string      `json:"healthMessage,omitempty"`
	LastError          string      `json:"lastError,omitempty"`
	DiscoveredAt       time.Time   `json:"discoveredAt"`
	UpdatedAt          time.Time   `json:"updatedAt"`
}

// LogEntry is bounded host-captured plugin stderr or supervision evidence.
type LogEntry struct {
	ID       int64     `json:"id"`
	PluginID string    `json:"pluginId"`
	Level    string    `json:"level"`
	Message  string    `json:"message"`
	Created  time.Time `json:"createdAt"`
}
