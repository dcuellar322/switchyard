// Package application coordinates owned, durable interactive PTY sessions.
package application

import (
	"context"
	"io"
	"time"

	"switchyard.dev/switchyard/internal/terminal/domain"
)

// LaunchPlan is a trusted, fully resolved argument-array PTY command. Command
// and environment values are deliberately excluded from durable Session facts.
type LaunchPlan struct {
	ProjectID        string
	EnvironmentID    string
	DisplayName      string
	WorkingDirectory string
	Executable       string
	Arguments        []string
	Environment      map[string]string
	Provider         string
	ServiceID        string
	ActionID         string
}

// LaunchResolver maps one typed request through accepted project metadata.
type LaunchResolver interface {
	Resolve(context.Context, domain.CreateRequest) (LaunchPlan, error)
}

// Process is one daemon-owned PTY and process group.
type Process interface {
	io.Reader
	Write([]byte) (int, error)
	Resize(domain.Size) error
	Terminate(context.Context) error
	Wait() error
	PID() int
	Close() error
}

// Spawner starts a PTY without a shell unless the reviewed plan explicitly
// selected one as its executable.
type Spawner interface {
	Start(context.Context, LaunchPlan, domain.Size) (Process, error)
}

// Repository owns durable metadata and audits. It never stores terminal bytes.
type Repository interface {
	Create(context.Context, domain.Session) error
	Update(context.Context, domain.Session) error
	Get(context.Context, string) (domain.Session, error)
	List(context.Context, string) ([]domain.Session, error)
	InterruptActive(context.Context, time.Time) error
	AppendAudit(context.Context, domain.Audit) error
}

// Config bounds in-memory output, detached lifetime, and stream fanout.
type Config struct {
	ScrollbackBytes int
	IdleTimeout     time.Duration
	SubscriberQueue int
}

// DefaultConfig preserves a detached PTY for thirty minutes while bounding
// reconnect scrollback to one MiB per live session.
func DefaultConfig() Config {
	return Config{ScrollbackBytes: 1 << 20, IdleTimeout: 30 * time.Minute, SubscriberQueue: 64}
}
