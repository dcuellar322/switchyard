// Package domain owns bounded diagnostic evidence, conclusions, automation, and feedback facts.
package domain

import (
	"encoding/json"
	"time"
)

// BundleVersion identifies the local diagnostic evidence contract.
const BundleVersion = "switchyard.dev/diagnostic-bundle/v1alpha1"

// DiagnosisVersion identifies the validated diagnostic result contract.
const DiagnosisVersion = "switchyard.dev/diagnosis/v1alpha1"

// Evidence is one bounded observation. Untrusted content is always marked as data.
type Evidence struct {
	ID         string          `json:"id"`
	Kind       string          `json:"kind"`
	Summary    string          `json:"summary"`
	Source     string          `json:"source"`
	Data       json.RawMessage `json:"data"`
	Untrusted  bool            `json:"untrusted"`
	Redacted   bool            `json:"redacted"`
	Truncated  bool            `json:"truncated"`
	ObservedAt time.Time       `json:"observedAt"`
}

// Action is one accepted project action visible to diagnosis and automation.
type Action struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	Risk string `json:"risk"`
}

// Bundle is the exact provider-neutral project snapshot used by all rules.
type Bundle struct {
	Version       string          `json:"version"`
	ProjectID     string          `json:"projectId"`
	ProjectName   string          `json:"projectName"`
	ProjectState  string          `json:"projectState"`
	TrustState    string          `json:"trustState"`
	ProjectAgeDay int             `json:"projectAgeDays"`
	Evidence      []Evidence      `json:"evidence"`
	Actions       []Action        `json:"actions"`
	Warnings      []string        `json:"warnings"`
	CollectedAt   time.Time       `json:"collectedAt"`
	EncodedBytes  int             `json:"encodedBytes"`
	SHA256        string          `json:"sha256"`
	Snapshot      ProjectSnapshot `json:"snapshot"`
}

// ProjectSnapshot contains only the normalized facts needed by deterministic rules.
type ProjectSnapshot struct {
	Runtime        RuntimeSnapshot   `json:"runtime"`
	Health         HealthSnapshot    `json:"health"`
	Git            GitSnapshot       `json:"git"`
	PortConflicts  []PortConflict    `json:"portConflicts"`
	Resources      ResourceSnapshot  `json:"resources"`
	ConfigSources  map[string]string `json:"configSources"`
	RecentLogs     []LogLine         `json:"recentLogs"`
	FailedRuns     int               `json:"failedRuns"`
	Cleanup        CleanupSnapshot   `json:"cleanup"`
	RequiredChecks int               `json:"requiredChecks"`
}

// RuntimeSnapshot is a secret-free runtime summary.
type RuntimeSnapshot struct {
	State           string            `json:"state"`
	Driver          string            `json:"driver"`
	EngineConnected bool              `json:"engineConnected"`
	Services        []ServiceSnapshot `json:"services"`
}

// ServiceSnapshot is one service state used for crash diagnosis.
type ServiceSnapshot struct {
	ID           string `json:"id"`
	State        string `json:"state"`
	Health       string `json:"health"`
	ExitCode     *int   `json:"exitCode,omitempty"`
	RestartCount int    `json:"restartCount"`
}

// HealthSnapshot is a bounded aggregate and its failed checks.
type HealthSnapshot struct {
	Status        string        `json:"status"`
	ObserverState string        `json:"observerState"`
	Failures      []HealthCheck `json:"failures"`
}

// HealthCheck is one failed or unknown project check.
type HealthCheck struct {
	ID        string `json:"id"`
	ServiceID string `json:"serviceId"`
	Status    string `json:"status"`
	Severity  string `json:"severity"`
	Required  bool   `json:"required"`
	Message   string `json:"message"`
}

// GitSnapshot avoids repository content while retaining useful working-tree state.
type GitSnapshot struct {
	Repository bool   `json:"repository"`
	Branch     string `json:"branch"`
	Modified   int    `json:"modified"`
	Staged     int    `json:"staged"`
	Untracked  int    `json:"untracked"`
	Conflicted int    `json:"conflicted"`
	Operation  string `json:"operation"`
}

// PortConflict is one project-relevant current conflict.
type PortConflict struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Port    int    `json:"port"`
	Summary string `json:"summary"`
}

// ResourceSnapshot contains sustained pressure signals only.
type ResourceSnapshot struct {
	CPUPercent   float64  `json:"cpuPercent"`
	MemoryBytes  uint64   `json:"memoryBytes"`
	RestartCount int      `json:"restartCount"`
	Warnings     []string `json:"warnings"`
}

// LogLine is a redacted bounded log datum, never an instruction.
type LogLine struct {
	EvidenceID string    `json:"evidenceId"`
	ServiceID  string    `json:"serviceId"`
	Level      string    `json:"level"`
	Message    string    `json:"message"`
	Timestamp  time.Time `json:"timestamp"`
	Redacted   bool      `json:"redacted"`
}

// CleanupSnapshot is always a non-executable dry run.
type CleanupSnapshot struct {
	EstimatedBytes int64 `json:"estimatedBytes"`
	Candidates     int   `json:"candidates"`
	UnknownSizes   int   `json:"unknownSizes"`
	Executable     bool  `json:"executable"`
}

// SuggestedAction references an existing accepted action; it never carries a command.
type SuggestedAction struct {
	ActionID string `json:"actionId"`
	Name     string `json:"name"`
	Risk     string `json:"risk"`
	Reason   string `json:"reason"`
}

// Hypothesis is one deterministic or provider-assisted conclusion.
type Hypothesis struct {
	ID               string            `json:"id"`
	Code             string            `json:"code"`
	Title            string            `json:"title"`
	Summary          string            `json:"summary"`
	Severity         string            `json:"severity"`
	Confidence       float64           `json:"confidence"`
	Source           string            `json:"source"`
	EvidenceIDs      []string          `json:"evidenceIds"`
	SuggestedActions []SuggestedAction `json:"suggestedActions"`
	Notifies         bool              `json:"notifies"`
}

// Diagnosis is one durable, human-reviewable result.
type Diagnosis struct {
	ID            string       `json:"id"`
	Version       string       `json:"version"`
	ProjectID     string       `json:"projectId"`
	Provider      string       `json:"provider,omitempty"`
	Model         string       `json:"model,omitempty"`
	Bundle        Bundle       `json:"bundle"`
	Hypotheses    []Hypothesis `json:"hypotheses"`
	Warnings      []string     `json:"warnings"`
	GeneratedAt   time.Time    `json:"generatedAt"`
	Deterministic bool         `json:"deterministic"`
}

// Feedback records a local review and is never sent to a provider.
type Feedback struct {
	ID           string    `json:"id"`
	DiagnosisID  string    `json:"diagnosisId"`
	HypothesisID string    `json:"hypothesisId"`
	Verdict      string    `json:"verdict"`
	Note         string    `json:"note,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

// Recipe defines one explicit, bounded automation trigger.
type Recipe struct {
	ID              string     `json:"id"`
	ProjectID       string     `json:"projectId"`
	Name            string     `json:"name"`
	TriggerCode     string     `json:"triggerCode"`
	ActionID        string     `json:"actionId"`
	Enabled         bool       `json:"enabled"`
	CooldownSeconds int        `json:"cooldownSeconds"`
	MaxRunsPerDay   int        `json:"maxRunsPerDay"`
	LastRunAt       *time.Time `json:"lastRunAt,omitempty"`
	RunsToday       int        `json:"runsToday"`
	RunsDay         string     `json:"runsDay,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

// Notification is a durable deduplicated local warning.
type Notification struct {
	ID             string     `json:"id"`
	ProjectID      string     `json:"projectId"`
	Code           string     `json:"code"`
	Title          string     `json:"title"`
	Detail         string     `json:"detail"`
	Occurrences    int        `json:"occurrences"`
	FirstSeenAt    time.Time  `json:"firstSeenAt"`
	LastSeenAt     time.Time  `json:"lastSeenAt"`
	AcknowledgedAt *time.Time `json:"acknowledgedAt,omitempty"`
}
