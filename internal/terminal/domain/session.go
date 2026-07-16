// Package domain owns provider-neutral interactive terminal session facts.
package domain

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	// PersistenceDetachUntilIdle states that browser disconnects detach without
	// terminating the PTY; the daemon expires an unattached session later.
	PersistenceDetachUntilIdle = "detach_until_idle_timeout"
	// CaptureUserVisibleOutput is the only output scope recorded for terminals
	// and agents. It explicitly excludes private reasoning or hidden state.
	CaptureUserVisibleOutput = "user_visible_terminal_output_only"
)

var identifierPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:/-]{0,127}$`)

// Kind identifies one reviewed interactive launch capability.
type Kind string

const (
	// KindShell starts the user's login shell at a trusted project root.
	KindShell Kind = "shell"
	// KindService starts a shell inside a declared Compose service.
	KindService Kind = "service"
	// KindDatabase starts a supported database client in a declared service.
	KindDatabase Kind = "database"
	// KindAgent starts a supported coding-agent CLI as a PTY process.
	KindAgent Kind = "agent"
	// KindAction starts a trusted manifest action classified as interactive.
	KindAction Kind = "action"
)

// Status is the durable lifecycle of one session record.
type Status string

const (
	// StatusStarting is persisted before the PTY is created.
	StatusStarting Status = "starting"
	// StatusActive has a live PTY owned by this daemon.
	StatusActive Status = "active"
	// StatusExited ended because the interactive command returned.
	StatusExited Status = "exited"
	// StatusTerminated ended after an explicit owner request.
	StatusTerminated Status = "terminated"
	// StatusExpired ended after the detached idle deadline.
	StatusExpired Status = "expired"
	// StatusInterrupted was active when the owning daemon stopped or restarted.
	StatusInterrupted Status = "interrupted"
	// StatusFailed could not start or encountered an unrecoverable PTY failure.
	StatusFailed Status = "failed"
)

// Owner is the authenticated local principal allowed to attach and terminate.
type Owner struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// Validate checks a non-secret authenticated owner identity.
func (o Owner) Validate() error {
	if !identifierPattern.MatchString(o.Type) || !identifierPattern.MatchString(o.ID) {
		return errors.New("session owner must contain bounded identifiers")
	}
	return nil
}

// Size is one validated terminal grid size.
type Size struct {
	Columns uint16 `json:"columns"`
	Rows    uint16 `json:"rows"`
}

// CreateRequest is a typed launch request. It intentionally exposes no raw
// command field; arbitrary commands require a trusted interactive action.
type CreateRequest struct {
	ProjectID      string `json:"projectId"`
	EnvironmentID  string `json:"environmentId,omitempty"`
	Kind           Kind   `json:"kind"`
	Provider       string `json:"provider,omitempty"`
	ServiceID      string `json:"serviceId,omitempty"`
	ActionID       string `json:"actionId,omitempty"`
	Shell          string `json:"shell,omitempty"`
	DatabaseClient string `json:"databaseClient,omitempty"`
	Columns        uint16 `json:"columns"`
	Rows           uint16 `json:"rows"`
}

// Session is durable metadata only. Terminal bytes remain in bounded memory
// while the daemon owns the PTY and are never stored in SQLite.
type Session struct {
	ID                string     `json:"id"`
	ProjectID         string     `json:"projectId"`
	EnvironmentID     string     `json:"environmentId,omitempty"`
	Kind              Kind       `json:"kind"`
	DisplayName       string     `json:"displayName"`
	Owner             Owner      `json:"owner"`
	Provider          string     `json:"provider,omitempty"`
	ServiceID         string     `json:"serviceId,omitempty"`
	ActionID          string     `json:"actionId,omitempty"`
	WorkingDirectory  string     `json:"workingDirectory"`
	Status            Status     `json:"status"`
	PersistencePolicy string     `json:"persistencePolicy"`
	CapturePolicy     string     `json:"capturePolicy"`
	OutputBytes       int64      `json:"outputBytes"`
	OutputTruncated   bool       `json:"outputTruncated"`
	LastOutputAt      *time.Time `json:"lastOutputAt,omitempty"`
	ExitCode          *int       `json:"exitCode,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	LastAttachedAt    *time.Time `json:"lastAttachedAt,omitempty"`
	DetachedAt        *time.Time `json:"detachedAt,omitempty"`
	FinishedAt        *time.Time `json:"finishedAt,omitempty"`
	ErrorCode         string     `json:"errorCode,omitempty"`
}

// Audit is a metadata-only session security event. It never contains terminal
// input, output, environment values, or command arguments.
type Audit struct {
	ID         string         `json:"id"`
	SessionID  string         `json:"sessionId"`
	Event      string         `json:"event"`
	Actor      Owner          `json:"actor"`
	Detail     map[string]any `json:"detail"`
	OccurredAt time.Time      `json:"occurredAt"`
}

// Validate enforces the typed launch surface before repository or PTY use.
func (r CreateRequest) Validate() error {
	var problems []error
	if !identifierPattern.MatchString(r.ProjectID) {
		problems = append(problems, errors.New("project ID must be a bounded identifier"))
	}
	if r.EnvironmentID != "" && !identifierPattern.MatchString(r.EnvironmentID) {
		problems = append(problems, errors.New("environment ID must be a bounded identifier"))
	}
	if err := r.Size().Validate(); err != nil {
		problems = append(problems, err)
	}
	problems = append(problems, r.validateKind()...)
	return errors.Join(problems...)
}

func (r CreateRequest) validateKind() []error {
	switch r.Kind {
	case KindShell:
		return r.validateShell()
	case KindService:
		return r.validateService()
	case KindDatabase:
		return r.validateDatabase()
	case KindAgent:
		if r.Provider != "codex" && r.Provider != "claude" {
			return []error{errors.New("agent provider must be codex or claude")}
		}
	case KindAction:
		if !identifierPattern.MatchString(r.ActionID) {
			return []error{errors.New("action sessions require a bounded action ID")}
		}
	default:
		return []error{fmt.Errorf("unsupported terminal session kind %q", r.Kind)}
	}
	return nil
}

func (r CreateRequest) validateShell() []error {
	var problems []error
	if r.Provider != "" || r.ServiceID != "" || r.ActionID != "" || r.DatabaseClient != "" {
		problems = append(problems, errors.New("shell sessions do not accept provider, service, action, or database fields"))
	}
	if !supportedShell(r.Shell) {
		problems = append(problems, errors.New("shell must be sh, bash, or zsh"))
	}
	return problems
}

func (r CreateRequest) validateService() []error {
	var problems []error
	if !identifierPattern.MatchString(r.ServiceID) {
		problems = append(problems, errors.New("service sessions require a bounded service ID"))
	}
	if !supportedShell(r.Shell) {
		problems = append(problems, errors.New("service shell must be sh, bash, or zsh"))
	}
	return problems
}

func (r CreateRequest) validateDatabase() []error {
	var problems []error
	if !identifierPattern.MatchString(r.ServiceID) {
		problems = append(problems, errors.New("database sessions require a bounded service ID"))
	}
	switch r.DatabaseClient {
	case "psql", "mysql", "redis-cli", "mongosh", "sqlite3":
	default:
		problems = append(problems, errors.New("database client must be psql, mysql, redis-cli, mongosh, or sqlite3"))
	}
	return problems
}

func supportedShell(value string) bool {
	return value == "" || value == "sh" || value == "bash" || value == "zsh"
}

// Size returns the requested terminal grid.
func (r CreateRequest) Size() Size { return Size{Columns: r.Columns, Rows: r.Rows} }

// Validate checks durable invariants without requiring a live PTY.
func (s Session) Validate() error {
	var problems []error
	if !identifierPattern.MatchString(s.ID) || !identifierPattern.MatchString(s.ProjectID) {
		problems = append(problems, errors.New("session and project IDs must be bounded identifiers"))
	}
	if err := s.Owner.Validate(); err != nil {
		problems = append(problems, err)
	}
	if !filepath.IsAbs(s.WorkingDirectory) {
		problems = append(problems, errors.New("session working directory must be absolute"))
	}
	if strings.TrimSpace(s.DisplayName) == "" || len(s.DisplayName) > 160 {
		problems = append(problems, errors.New("session display name is required and limited to 160 characters"))
	}
	if s.PersistencePolicy != PersistenceDetachUntilIdle || s.CapturePolicy != CaptureUserVisibleOutput {
		problems = append(problems, errors.New("session persistence or capture policy is invalid"))
	}
	switch s.Status {
	case StatusStarting, StatusActive, StatusExited, StatusTerminated, StatusExpired, StatusInterrupted, StatusFailed:
	default:
		problems = append(problems, errors.New("session status is invalid"))
	}
	return errors.Join(problems...)
}

// Validate enforces a practical terminal grid and prevents overflow-sized PTYs.
func (s Size) Validate() error {
	if s.Columns < 2 || s.Columns > 500 || s.Rows < 2 || s.Rows > 300 {
		return errors.New("terminal size must be between 2x2 and 500x300")
	}
	return nil
}

// Active reports whether the record should have a daemon-owned PTY.
func (s Session) Active() bool { return s.Status == StatusStarting || s.Status == StatusActive }
