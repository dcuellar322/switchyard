// Package domain owns trusted action definitions and redaction-safe audit facts.
package domain

import "time"

// Risk classifies the authorization and confirmation sensitivity of an action.
type Risk string

// Supported action risk classifications.
const (
	RiskReadOnly    Risk = "read_only"
	RiskMutating    Risk = "mutating"
	RiskNetworked   Risk = "networked"
	RiskDestructive Risk = "destructive"
	RiskInteractive Risk = "interactive"
)

// Definition is an accepted, declarative project action.
type Definition struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Type             string            `json:"type"`
	Command          []string          `json:"command"`
	WorkingDirectory string            `json:"workingDirectory"`
	Shell            bool              `json:"shell"`
	CaptureOutput    bool              `json:"captureOutput"`
	Provider         string            `json:"provider,omitempty"`
	Target           string            `json:"target,omitempty"`
	Risk             Risk              `json:"risk"`
	TimeoutSeconds   int               `json:"timeoutSeconds"`
	Environment      map[string]string `json:"-"`
}

// ProjectActions contains definitions plus the trusted execution boundary.
type ProjectActions struct {
	ProjectID   string       `json:"projectId"`
	ProjectName string       `json:"projectName"`
	Root        string       `json:"-"`
	Actions     []Definition `json:"actions"`
}

// Execution is the fully resolved input supplied to an adapter.
type Execution struct {
	OperationID      string
	ProjectID        string
	Root             string
	WorkingDirectory string
	Action           Definition
}

// Audit records action identity and outcome without command output or environment values.
type Audit struct {
	ID               string     `json:"id"`
	OperationID      string     `json:"operationId"`
	ProjectID        string     `json:"projectId"`
	ActionID         string     `json:"actionId"`
	ActionType       string     `json:"actionType"`
	Risk             Risk       `json:"risk"`
	ActorType        string     `json:"actorType"`
	ActorID          string     `json:"actorId"`
	State            string     `json:"state"`
	WorkingDirectory string     `json:"workingDirectory"`
	StartedAt        time.Time  `json:"startedAt"`
	FinishedAt       *time.Time `json:"finishedAt,omitempty"`
	ErrorCode        string     `json:"errorCode,omitempty"`
}
