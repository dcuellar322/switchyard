// Package application coordinates durable operations through explicit ports.
package application

import (
	"context"
	"errors"
	"time"

	"switchyard.dev/switchyard/internal/operations/domain"
)

var (
	// ErrNotFound identifies an unknown operation.
	ErrNotFound = errors.New("operation not found")
	// ErrInvalidRequest identifies an incomplete operation request.
	ErrInvalidRequest = errors.New("invalid operation request")
)

// Repository persists operation lifecycle records, steps, and audit evidence.
type Repository interface {
	CreateOrGet(ctx context.Context, operation domain.Operation) (domain.Operation, bool, error)
	Get(ctx context.Context, id string) (domain.Operation, error)
	List(ctx context.Context, projectID string, limit int64) ([]domain.Operation, error)
	Transition(ctx context.Context, current domain.Operation, next domain.Operation) error
	RequestCancellation(ctx context.Context, id string, at time.Time) (bool, error)
	AddStep(ctx context.Context, step Step) error
	Recoverable(ctx context.Context) ([]domain.Operation, error)
	RecordAudit(ctx context.Context, event AuditEvent) error
}

// Step is an append-only operation progress record.
type Step struct {
	OperationID string
	Name        string
	State       string
	Message     string
	OccurredAt  time.Time
}

// AuditEvent records who requested a mutation and its durable target.
type AuditEvent struct {
	Type           string
	ActorType      string
	ActorID        string
	ProjectID      string
	OperationID    string
	IdempotencyKey string
	Detail         []byte
	OccurredAt     time.Time
}

// Progress reports structured executor steps.
type Progress interface {
	Step(ctx context.Context, name, state, message string) error
}

// Executor performs the operation-specific work registered by later domains.
type Executor interface {
	Execute(ctx context.Context, operation domain.Operation, progress Progress) error
}

// ExecutorFunc adapts a function into an Executor.
type ExecutorFunc func(context.Context, domain.Operation, Progress) error

// Execute implements Executor.
func (f ExecutorFunc) Execute(ctx context.Context, operation domain.Operation, progress Progress) error {
	return f(ctx, operation, progress)
}
