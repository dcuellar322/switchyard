// Package domain defines portable signed team configuration without depending
// on persistence, HTTP, encryption implementations, manifests, or plugins.
package domain

import (
	"encoding/json"
	"time"
)

const (
	// BundleSchemaVersion identifies canonical signed shared configuration.
	BundleSchemaVersion = "switchyard.bundle/v1"
	// SyncSchemaVersion identifies configuration-only encrypted sync documents.
	SyncSchemaVersion = "switchyard.sync/v1"
	// SignatureAlgorithm is the only bundle signature algorithm accepted by v1.
	SignatureAlgorithm = "Ed25519"
)

// BundleKind identifies a validated portable payload contract.
type BundleKind string

const (
	// KindProjectTemplate contains a portable parameterized project manifest.
	KindProjectTemplate BundleKind = "project-template"
	// KindPolicyPack contains restrictive optional-feature allowlists.
	KindPolicyPack BundleKind = "policy-pack"
	// KindPluginRegistry contains curated signed plugin metadata only.
	KindPluginRegistry BundleKind = "plugin-registry"
	// KindEnterpriseConfig contains restrictive policy and publisher requirements.
	KindEnterpriseConfig BundleKind = "enterprise-config"
)

// KnownBundleKinds is the complete signed bundle vocabulary.
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

// Signature records the publisher key, algorithm, and canonical-envelope signature.
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

// TemplateVariable defines one reviewed project-template substitution.
type TemplateVariable struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
}

// ProjectTemplate contains a portable manifest and bounded variable catalog.
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

// EnterpriseConfig combines restrictive policy with publisher requirements.
type EnterpriseConfig struct {
	Policy                     PolicyPack `json:"policy"`
	RequiredPublisherIDs       []string   `json:"requiredPublisherIds"`
	RequireSignedConfiguration bool       `json:"requireSignedConfiguration"`
}

// RegistryEntry is signed plugin metadata and never installation authority.
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

// PluginRegistry is a bounded collection of curated signed metadata.
type PluginRegistry struct {
	Entries []RegistryEntry `json:"entries"`
}

// EffectivePolicy is the restrictive intersection of installed policy bundles.
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

// SyncPreview lists verified import effects and replacement warnings.
type SyncPreview struct {
	PublisherCount int      `json:"publisherCount"`
	BundleCount    int      `json:"bundleCount"`
	BundleIDs      []string `json:"bundleIds"`
	Warnings       []string `json:"warnings"`
}

// AuditEvent records publisher trust and signed configuration mutations.
type AuditEvent struct {
	Type, ActorType, ActorID, SubjectID, Detail string
	OccurredAt                                  time.Time
}
