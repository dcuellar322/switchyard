// Package domain defines redacted, portable support evidence.
package domain

import "time"

const (
	// PreviewSchema identifies the stable support preview document.
	PreviewSchema = "switchyard.dev/support-preview/v1"
	// BundleSchema identifies the stable archive manifest.
	BundleSchema = "switchyard.dev/support-bundle/v1"
)

// SystemIdentity is non-secret build and schema evidence from the daemon.
type SystemIdentity struct {
	Status         string `json:"status"`
	Version        string `json:"version"`
	Commit         string `json:"commit"`
	APIVersion     string `json:"apiVersion"`
	DatabaseSchema int    `json:"databaseSchema"`
}

// AdapterAvailability reports a local capability without exposing its path.
type AdapterAvailability struct {
	ID        string `json:"id"`
	Available bool   `json:"available"`
	Detail    string `json:"detail"`
}

// ProviderConfiguration discloses configuration presence, never credentials.
type ProviderConfiguration struct {
	ID                            string `json:"id"`
	Model                         string `json:"model,omitempty"`
	Configured                    bool   `json:"configured"`
	CredentialReferenceConfigured bool   `json:"credentialReferenceConfigured"`
}

// RetentionConfiguration records non-secret daemon bounds.
type RetentionConfiguration struct {
	LogAge             string `json:"logAge"`
	LogMaximumBytes    int64  `json:"logMaximumBytes"`
	MetricRaw          string `json:"metricRaw"`
	MetricMinute       string `json:"metricMinute"`
	MetricQuarterHour  string `json:"metricQuarterHour"`
	MaximumHistoryRows int    `json:"maximumHistoryRows"`
}

// SanitizedConfiguration is the allowlisted support-safe process configuration.
type SanitizedConfiguration struct {
	HTTPBinding         string                  `json:"httpBinding"`
	IPCMode             string                  `json:"ipcMode"`
	RoutingEnabled      bool                    `json:"routingEnabled"`
	RemoteAgentEnabled  bool                    `json:"remoteAgentEnabled"`
	SettingsRevision    int64                   `json:"settingsRevision,omitempty"`
	ProjectRootCount    int                     `json:"projectRootCount,omitempty"`
	PreferredPortRange  string                  `json:"preferredPortRange,omitempty"`
	ExcludedPortCount   int                     `json:"excludedPortCount,omitempty"`
	TerminalPreference  string                  `json:"terminalPreference,omitempty"`
	EditorPreference    string                  `json:"editorPreference,omitempty"`
	DefaultAgentProfile string                  `json:"defaultAgentProfile,omitempty"`
	Appearance          string                  `json:"appearance,omitempty"`
	Retention           RetentionConfiguration  `json:"retention"`
	Providers           []ProviderConfiguration `json:"providers"`
}

// InternalLogEntry is an allowlisted redacted daemon event. It never contains
// project application output or arbitrary structured attributes.
type InternalLogEntry struct {
	Timestamp     time.Time `json:"timestamp"`
	Level         string    `json:"level"`
	Message       string    `json:"message"`
	Component     string    `json:"component,omitempty"`
	ErrorCode     string    `json:"errorCode,omitempty"`
	Error         string    `json:"error,omitempty"`
	ProjectID     string    `json:"projectId,omitempty"`
	OperationID   string    `json:"operationId,omitempty"`
	CorrelationID string    `json:"correlationId,omitempty"`
}

// Preview is the exact review shown before a support archive is written.
type Preview struct {
	SchemaVersion  string                 `json:"schemaVersion"`
	GeneratedAt    time.Time              `json:"generatedAt"`
	System         SystemIdentity         `json:"system"`
	Adapters       []AdapterAvailability  `json:"adapters"`
	Configuration  SanitizedConfiguration `json:"configuration"`
	InternalErrors []InternalLogEntry     `json:"internalErrors"`
	Included       []string               `json:"included"`
	Excluded       []string               `json:"excluded"`
	Redaction      string                 `json:"redaction"`
}

// BundleManifest is the archive's canonical root document.
type BundleManifest struct {
	SchemaVersion string   `json:"schemaVersion"`
	Preview       Preview  `json:"preview"`
	Files         []string `json:"files"`
}

// BundleReceipt identifies one private archive without exposing its contents.
type BundleReceipt struct {
	Path      string  `json:"path"`
	SHA256    string  `json:"sha256"`
	SizeBytes int64   `json:"sizeBytes"`
	Preview   Preview `json:"preview"`
}
