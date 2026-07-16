package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	operations "switchyard.dev/switchyard/internal/operations/application"
	"switchyard.dev/switchyard/internal/operations/domain"
	"switchyard.dev/switchyard/internal/platform/sqlite/generated"
)

// OperationRepository persists the operations kernel in SQLite.
type OperationRepository struct {
	queries *generated.Queries
}

// NewOperationRepository creates a repository over database.
func NewOperationRepository(database *Database) *OperationRepository {
	return &OperationRepository{queries: database.queries}
}

// CreateOrGet atomically deduplicates an operation by project and idempotency key.
func (r *OperationRepository) CreateOrGet(ctx context.Context, operation domain.Operation) (domain.Operation, bool, error) {
	rows, err := r.queries.CreateOperation(ctx, generated.CreateOperationParams{
		ID: operation.ID, ProjectID: operation.ProjectID, Kind: operation.Kind,
		State: string(operation.State), IdempotencyKey: operation.IdempotencyKey,
		InputJson: string(defaultJSON(operation.Input)), RequestedAt: formatTime(operation.RequestedAt),
		UpdatedAt: formatTime(operation.UpdatedAt),
	})
	if err != nil {
		return domain.Operation{}, false, fmt.Errorf("insert operation: %w", err)
	}
	if rows == 0 {
		existing, err := r.queries.GetOperationByIdempotency(ctx, generated.GetOperationByIdempotencyParams{
			ProjectID: operation.ProjectID, IdempotencyKey: operation.IdempotencyKey,
		})
		if err != nil {
			return domain.Operation{}, false, fmt.Errorf("read idempotent operation: %w", err)
		}
		mapped, err := mapOperation(existing)
		return mapped, false, err
	}
	return operation, true, nil
}

// Get returns an operation by ID.
func (r *OperationRepository) Get(ctx context.Context, id string) (domain.Operation, error) {
	record, err := r.queries.GetOperation(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Operation{}, operations.ErrNotFound
	}
	if err != nil {
		return domain.Operation{}, fmt.Errorf("read operation: %w", err)
	}
	return mapOperation(record)
}

// Transition applies a compare-and-swap lifecycle change.
func (r *OperationRepository) Transition(ctx context.Context, current, next domain.Operation) error {
	rows, err := r.queries.UpdateOperationState(ctx, generated.UpdateOperationStateParams{
		State: string(next.State), ErrorCode: nullable(next.ErrorCode), ErrorMessage: nullable(next.ErrorMessage),
		StartedAt: nullableTime(next.StartedAt), FinishedAt: nullableTime(next.FinishedAt),
		UpdatedAt: formatTime(next.UpdatedAt), ID: next.ID, State_2: string(current.State),
	})
	if err != nil {
		return fmt.Errorf("update operation state: %w", err)
	}
	if rows != 1 {
		return fmt.Errorf("%w: operation state changed concurrently", domain.ErrInvalidTransition)
	}
	return nil
}

// RequestCancellation records a durable cancellation request.
func (r *OperationRepository) RequestCancellation(ctx context.Context, id string, at time.Time) (bool, error) {
	rows, err := r.queries.RequestOperationCancellation(ctx, generated.RequestOperationCancellationParams{
		UpdatedAt: formatTime(at), ID: id,
	})
	if err != nil {
		return false, fmt.Errorf("update cancellation request: %w", err)
	}
	return rows == 1, nil
}

// AddStep appends structured progress.
func (r *OperationRepository) AddStep(ctx context.Context, step operations.Step) error {
	if err := r.queries.CreateOperationStep(ctx, generated.CreateOperationStepParams{
		OperationID: step.OperationID, Name: step.Name, State: step.State,
		Message: step.Message, OccurredAt: formatTime(step.OccurredAt),
	}); err != nil {
		return fmt.Errorf("insert operation step: %w", err)
	}
	return nil
}

// Recoverable returns queued and interrupted operations in request order.
func (r *OperationRepository) Recoverable(ctx context.Context) ([]domain.Operation, error) {
	records, err := r.queries.ListRecoverableOperations(ctx)
	if err != nil {
		return nil, fmt.Errorf("list recoverable operations: %w", err)
	}
	result := make([]domain.Operation, 0, len(records))
	for _, record := range records {
		operation, err := mapOperation(record)
		if err != nil {
			return nil, err
		}
		result = append(result, operation)
	}
	return result, nil
}

// RecordAudit appends mutation evidence.
func (r *OperationRepository) RecordAudit(ctx context.Context, event operations.AuditEvent) error {
	if err := r.queries.CreateAuditEvent(ctx, generated.CreateAuditEventParams{
		EventType: event.Type, ActorType: event.ActorType, ActorID: event.ActorID,
		ProjectID: nullable(event.ProjectID), OperationID: nullable(event.OperationID),
		IdempotencyKey: nullable(event.IdempotencyKey), DetailJson: string(defaultJSON(event.Detail)),
		OccurredAt: formatTime(event.OccurredAt),
	}); err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}
	return nil
}

func mapOperation(record generated.Operation) (domain.Operation, error) {
	requestedAt, err := parseTime(record.RequestedAt)
	if err != nil {
		return domain.Operation{}, err
	}
	updatedAt, err := parseTime(record.UpdatedAt)
	if err != nil {
		return domain.Operation{}, err
	}
	startedAt, err := parseNullableTime(record.StartedAt)
	if err != nil {
		return domain.Operation{}, err
	}
	finishedAt, err := parseNullableTime(record.FinishedAt)
	if err != nil {
		return domain.Operation{}, err
	}
	return domain.Operation{
		ID: record.ID, ProjectID: record.ProjectID, Kind: record.Kind,
		State: domain.State(record.State), IdempotencyKey: record.IdempotencyKey,
		Input: []byte(record.InputJson), ErrorCode: record.ErrorCode.String,
		ErrorMessage: record.ErrorMessage.String, CancellationRequested: record.CancellationRequested == 1,
		RequestedAt: requestedAt, StartedAt: startedAt, FinishedAt: finishedAt, UpdatedAt: updatedAt,
	}, nil
}

func formatTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func parseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse persisted timestamp: %w", err)
	}
	return parsed, nil
}

func parseNullableTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid {
		return nil, nil
	}
	parsed, err := parseTime(value.String)
	return &parsed, err
}

func nullable(value string) sql.NullString { return sql.NullString{String: value, Valid: value != ""} }

func nullableTime(value *time.Time) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return nullable(formatTime(*value))
}

func defaultJSON(value []byte) []byte {
	if len(value) == 0 {
		return []byte(`{}`)
	}
	return value
}
