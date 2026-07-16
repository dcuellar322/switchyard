// Package domain defines portable signed team configuration without depending
// on persistence, HTTP, encryption implementations, manifests, or plugins.
package domain

import (
	"encoding/json"
	"time"
)

const (
	BundleSchemaVersion = "switchyard.bundle/v1"
	SyncSchemaVersion   = "switchyard.sync/v1"
	SignatureAlgorithm  = "Ed25519"
)

type BundleKind string

const (
	KindProjectTemplate  BundleKind = "project-template"
	KindPolicyPack       BundleKind = "policy-pack"
	KindPluginRegistry   BundleKind = "plugin-registry"
	KindEnterpriseConfig BundleKind = "enterprise-config"
)

var KnownBundleKinds = []BundleKind{KindProjectTemplate, KindPolicyPack, KindPluginRegistry, KindEnterpriseConfig}

// BundleMetadata is publisher-controlled portable identity.
type BundleMetadata struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Version     string     `json:"version"`
	PublisherID string     `json:"publisherId"`
	CreatedAt   time.Time  `json:"createdAt"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
}

type Signature struct {
	KeyID     string `json:"keyId"`
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
}

// Bundle signs a canonical envelope and normalized JSON payload. Payloads may
// never contain private keys, access tokens, environment values, or host paths.
type Bundle struct {
	SchemaVersion string          `json:"schemaVersion"`
	Kind          BundleKind      `json:"kind"`
	Metadata      BundleMetadata  `json:"metadata"`
	Payload       json.RawMessage `json:"payload"`
	Signature     Signature       `json:"signature"`
	InstalledAt   *time.Time      `json:"installedAt,omitempty"`
}

// Publisher is an explicitly trusted public signing identity.
type Publisher struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	PublicKey string    `json:"publicKey"`
	TrustedAt time.Time `json:"trustedAt"`
}

type TemplateVariable struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
}

type ProjectTemplate struct {
	Manifest  json.RawMessage    `json:"manifest"`
	Variables []TemplateVariable `json:"variables"`
}

// PolicyPack is intentionally allowlist-based. Empty arrays deny the
// corresponding optional capability when a pack is installed.
type PolicyPack struct {
	AllowedRemoteCapabilities []string `json:"allowedRemoteCapabilities"`
	AllowedRemoteActions      []string `json:"allowedRemoteActions"`
	AllowedPluginPublishers   []string `json:"allowedPluginPublishers"`
	TelemetryAllowed          bool     `json:"telemetryAllowed"`
}

type EnterpriseConfig struct {
	Policy                     PolicyPack `json:"policy"`
	RequiredPublisherIDs       []string   `json:"requiredPublisherIds"`
	RequireSignedConfiguration bool       `json:"requireSignedConfiguration"`
}

type RegistryEntry struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Version       string   `json:"version"`
	Summary       string   `json:"summary"`
	Publisher     string   `json:"publisher"`
	DownloadURL   string   `json:"downloadUrl"`
	SHA256        string   `json:"sha256"`
	Platforms     []string `json:"platforms"`
	Capabilities  []string `json:"capabilities"`
	Documentation string   `json:"documentation,omitempty"`
}

type PluginRegistry struct {
	Entries []RegistryEntry `json:"entries"`
}

type EffectivePolicy struct {
	SourceBundleIDs            []string `json:"sourceBundleIds"`
	AllowedRemoteCapabilities  []string `json:"allowedRemoteCapabilities"`
	AllowedRemoteActions       []string `json:"allowedRemoteActions"`
	AllowedPluginPublishers    []string `json:"allowedPluginPublishers"`
	TelemetryAllowed           bool     `json:"telemetryAllowed"`
	RequireSignedConfiguration bool     `json:"requireSignedConfiguration"`
}

// SyncDocument contains configuration only. Fleet registrations, certificate
// references, projects, operations, repositories, logs, and secrets are absent.
type SyncDocument struct {
	SchemaVersion string      `json:"schemaVersion"`
	Publishers    []Publisher `json:"publishers"`
	Bundles       []Bundle    `json:"bundles"`
	ExportedAt    time.Time   `json:"exportedAt"`
}

type SyncPreview struct {
	PublisherCount int      `json:"publisherCount"`
	BundleCount    int      `json:"bundleCount"`
	BundleIDs      []string `json:"bundleIds"`
	Warnings       []string `json:"warnings"`
}

type AuditEvent struct {
	Type, ActorType, ActorID, SubjectID, Detail string
	OccurredAt                                  time.Time
}
