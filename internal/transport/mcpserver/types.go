package mcpserver

import "switchyard.dev/switchyard/internal/transport/contract/generated"

const schemaVersion = "switchyard.mcp/v1"

type emptyInput struct{}

type projectInput struct {
	ProjectID string `json:"projectId" jsonschema:"opaque Switchyard project identifier"`
}

type systemOutput struct {
	SchemaVersion string               `json:"schemaVersion"`
	System        generated.SystemInfo `json:"system"`
}

type projectsInput struct {
	Limit int `json:"limit,omitempty" jsonschema:"maximum projects to return, from 1 to 100"`
}

type projectsOutput struct {
	SchemaVersion string              `json:"schemaVersion"`
	Projects      []generated.Project `json:"projects"`
	Truncated     bool                `json:"truncated"`
}

type projectOutput struct {
	SchemaVersion string            `json:"schemaVersion"`
	Project       generated.Project `json:"project"`
}

type statusOutput struct {
	SchemaVersion string                       `json:"schemaVersion"`
	Project       generated.Project            `json:"project"`
	Runtime       generated.RuntimeObservation `json:"runtime"`
	Health        generated.ProjectHealth      `json:"health"`
}

type servicesOutput struct {
	SchemaVersion string                                `json:"schemaVersion"`
	ProjectID     string                                `json:"projectId"`
	Services      []generated.RuntimeServiceObservation `json:"services"`
	Truncated     bool                                  `json:"truncated"`
}

type logsInput struct {
	ProjectID   string `json:"projectId" jsonschema:"opaque Switchyard project identifier"`
	ServiceID   string `json:"serviceId,omitempty" jsonschema:"optional declared service identifier"`
	Since       string `json:"since,omitempty" jsonschema:"optional RFC3339 timestamp or bounded duration such as 10m"`
	RunID       string `json:"runId,omitempty" jsonschema:"optional native process run identifier"`
	OperationID string `json:"operationId,omitempty" jsonschema:"optional durable operation identifier"`
	Tail        int    `json:"tail,omitempty" jsonschema:"maximum redacted entries to return, from 1 to 500"`
}

type logsOutput struct {
	SchemaVersion string                      `json:"schemaVersion"`
	Entries       []generated.RuntimeLogEntry `json:"entries"`
	Truncated     bool                        `json:"truncated"`
}

type healthOutput struct {
	SchemaVersion string                  `json:"schemaVersion"`
	Health        generated.ProjectHealth `json:"health"`
}

type healthWaitInput struct {
	ProjectID      string `json:"projectId" jsonschema:"opaque Switchyard project identifier"`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty" jsonschema:"bounded wait from 1 to 30 seconds"`
}

type healthWaitOutput struct {
	SchemaVersion string                  `json:"schemaVersion"`
	Health        generated.ProjectHealth `json:"health"`
	Healthy       bool                    `json:"healthy"`
	TimedOut      bool                    `json:"timedOut"`
}

type gitOutput struct {
	SchemaVersion string             `json:"schemaVersion"`
	Git           generated.GitState `json:"git"`
}

type portsInput struct {
	ProjectID string `json:"projectId,omitempty" jsonschema:"optional project filter"`
	Limit     int    `json:"limit,omitempty" jsonschema:"maximum port facts to return, from 1 to 500"`
}

type portsOutput struct {
	SchemaVersion string                   `json:"schemaVersion"`
	Facts         []generated.PortFact     `json:"facts"`
	Conflicts     []generated.PortConflict `json:"conflicts"`
	Warnings      []string                 `json:"warnings"`
	Truncated     bool                     `json:"truncated"`
}

type portSuggestionInput struct {
	RangeStart int    `json:"rangeStart" jsonschema:"first candidate port, from 1 to 65535"`
	RangeEnd   int    `json:"rangeEnd" jsonschema:"last candidate port, from 1 to 65535"`
	Protocol   string `json:"protocol" jsonschema:"tcp or udp"`
	ProjectID  string `json:"projectId,omitempty" jsonschema:"optional project owning the declaration"`
	Excluded   []int  `json:"excluded,omitempty" jsonschema:"ports to skip"`
	RequestID  string `json:"requestId" jsonschema:"stable opaque idempotency key, 8 to 128 characters"`
}

type portSuggestionOutput struct {
	SchemaVersion string                   `json:"schemaVersion"`
	Suggestion    generated.PortSuggestion `json:"suggestion"`
}

type actionsOutput struct {
	SchemaVersion string                   `json:"schemaVersion"`
	Actions       generated.ProjectActions `json:"actions"`
	Truncated     bool                     `json:"truncated"`
}

type operationInput struct {
	OperationID string `json:"operationId" jsonschema:"opaque durable operation identifier"`
}

type operationOutput struct {
	SchemaVersion string              `json:"schemaVersion"`
	Operation     generated.Operation `json:"operation"`
}

type operationWaitInput struct {
	OperationID    string `json:"operationId" jsonschema:"opaque durable operation identifier"`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty" jsonschema:"bounded wait from 1 to 30 seconds"`
}

type operationWaitOutput struct {
	SchemaVersion string              `json:"schemaVersion"`
	Operation     generated.Operation `json:"operation"`
	Terminal      bool                `json:"terminal"`
	TimedOut      bool                `json:"timedOut"`
}

type manifestOutput struct {
	SchemaVersion string                      `json:"schemaVersion"`
	Manifest      generated.EffectiveManifest `json:"manifest"`
}

type lifecycleInput struct {
	ProjectID  string   `json:"projectId" jsonschema:"opaque Switchyard project identifier"`
	ServiceIDs []string `json:"serviceIds,omitempty" jsonschema:"optional declared service identifiers; omit for the whole project"`
	RequestID  string   `json:"requestId" jsonschema:"stable opaque idempotency key, 8 to 128 characters"`
}

type teardownInput struct {
	ProjectID     string `json:"projectId" jsonschema:"opaque Switchyard project identifier"`
	RemoveVolumes bool   `json:"removeVolumes" jsonschema:"explicitly remove named and anonymous Compose volumes"`
	RequestID     string `json:"requestId" jsonschema:"stable opaque idempotency key, 8 to 128 characters"`
}

type mutationOutput struct {
	SchemaVersion string              `json:"schemaVersion"`
	Operation     generated.Operation `json:"operation"`
}

type actionInput struct {
	ProjectID        string `json:"projectId" jsonschema:"opaque Switchyard project identifier"`
	ActionID         string `json:"actionId" jsonschema:"identifier returned by switchyard_actions_list"`
	ConfirmRisk      bool   `json:"confirmRisk,omitempty" jsonschema:"explicit confirmation for a risk-bearing action"`
	AllowOutsideRoot bool   `json:"allowOutsideRoot,omitempty" jsonschema:"admin-only approval for a declared action working directory outside the project root"`
	RequestID        string `json:"requestId" jsonschema:"stable opaque idempotency key, 8 to 128 characters"`
}

type cancelInput struct {
	OperationID string `json:"operationId" jsonschema:"opaque durable operation identifier"`
	RequestID   string `json:"requestId" jsonschema:"stable opaque idempotency key, 8 to 128 characters"`
}

type proposalCreateInput struct {
	Path      string `json:"path" jsonschema:"repository directory to scan without executing repository code"`
	RequestID string `json:"requestId" jsonschema:"stable opaque idempotency key, 8 to 128 characters"`
}

type proposalOutput struct {
	SchemaVersion string                     `json:"schemaVersion"`
	Proposal      generated.ManifestProposal `json:"proposal"`
}

type proposalAcceptInput struct {
	ProposalID string `json:"proposalId" jsonschema:"validated proposal identifier reviewed by the user"`
	RequestID  string `json:"requestId" jsonschema:"stable opaque idempotency key, 8 to 128 characters"`
}

type proposalAcceptOutput struct {
	SchemaVersion string                             `json:"schemaVersion"`
	Accepted      generated.AcceptedManifestProposal `json:"accepted"`
}
