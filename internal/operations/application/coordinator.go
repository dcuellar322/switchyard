package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"switchyard.dev/switchyard/internal/foundation/events"
	"switchyard.dev/switchyard/internal/foundation/identifier"
	"switchyard.dev/switchyard/internal/operations/domain"
)

// SubmitRequest describes a durable mutation before domain-specific execution.
type SubmitRequest struct {
	ProjectID      string
	WorkspaceID    string
	Kind           string
	IdempotencyKey string
	Input          []byte
	ActorType      string
	ActorID        string
}

// Observer receives a non-authoritative notification after a new durable
// operation is accepted. Implementations must discard identifying input.
type Observer interface {
	ObserveOperation(context.Context, string)
}

// Coordinator serializes operation execution by project and persists progress.
type Coordinator struct {
	ctx       context.Context
	repo      Repository
	journal   events.Journal
	executor  Executor
	now       func() time.Time
	gates     *keyedGate
	mu        sync.Mutex
	active    map[string]context.CancelFunc
	wg        sync.WaitGroup
	observers []Observer
}

// NewCoordinator constructs the durable operation coordinator.
func NewCoordinator(ctx context.Context, repo Repository, journal events.Journal, executor Executor, observers ...Observer) *Coordinator {
	return &Coordinator{
		ctx: ctx, repo: repo, journal: journal, executor: executor,
		now: time.Now, gates: newKeyedGate(), active: make(map[string]context.CancelFunc), observers: append([]Observer(nil), observers...),
	}
}

// Submit creates an idempotent queued operation and schedules its execution.
func (c *Coordinator) Submit(ctx context.Context, request SubmitRequest) (domain.Operation, error) {
	if request.ProjectID == "" || request.Kind == "" || request.IdempotencyKey == "" {
		return domain.Operation{}, ErrInvalidRequest
	}
	id, err := identifier.New("op")
	if err != nil {
		return domain.Operation{}, err
	}
	now := c.now().UTC()
	operation := domain.Operation{
		ID: id, ProjectID: request.ProjectID, WorkspaceID: request.WorkspaceID, Kind: request.Kind,
		State: domain.StateQueued, IdempotencyKey: request.IdempotencyKey,
		Input: append([]byte(nil), request.Input...), RequestedAt: now, UpdatedAt: now,
	}
	operation, created, err := c.repo.CreateOrGet(ctx, operation)
	if err != nil {
		return domain.Operation{}, fmt.Errorf("create operation: %w", err)
	}
	if !created {
		return operation, nil
	}
	auditErr := c.repo.RecordAudit(ctx, AuditEvent{
		Type: "operation.requested", ActorType: defaultValue(request.ActorType, "local"),
		ActorID: defaultValue(request.ActorID, "unknown"), ProjectID: operation.ProjectID,
		WorkspaceID: operation.WorkspaceID,
		OperationID: operation.ID, IdempotencyKey: operation.IdempotencyKey,
		Detail: []byte(`{}`), OccurredAt: now,
	})
	emitErr := c.emit(ctx, operation, "operation.queued", map[string]any{"kind": operation.Kind})
	for _, observer := range c.observers {
		observer.ObserveOperation(ctx, operation.Kind)
	}
	c.schedule(operation)
	if auditErr != nil || emitErr != nil {
		return operation, errors.Join(
			wrapIfPresent("record operation audit", auditErr),
			emitErr,
		)
	}
	return operation, nil
}

// Get returns one durable operation.
func (c *Coordinator) Get(ctx context.Context, id string) (domain.Operation, error) {
	return c.repo.Get(ctx, id)
}

// List returns recent durable operations, optionally filtered by project.
func (c *Coordinator) List(ctx context.Context, projectID string, limit int64) ([]domain.Operation, error) {
	if limit < 1 || limit > 500 {
		return nil, ErrInvalidRequest
	}
	return c.repo.List(ctx, projectID, limit)
}

// Cancel requests idempotent cancellation and signals a live executor.
func (c *Coordinator) Cancel(ctx context.Context, id, actorType, actorID, idempotencyKey string) (domain.Operation, error) {
	operation, err := c.repo.Get(ctx, id)
	if err != nil {
		return domain.Operation{}, err
	}
	if operation.Terminal() {
		return operation, nil
	}
	now := c.now().UTC()
	changed, err := c.repo.RequestCancellation(ctx, id, now)
	if err != nil {
		return domain.Operation{}, fmt.Errorf("request cancellation: %w", err)
	}
	var sideEffectErr error
	if changed {
		auditErr := c.repo.RecordAudit(ctx, AuditEvent{
			Type: "operation.cancellation_requested", ActorType: defaultValue(actorType, "local"),
			ActorID: defaultValue(actorID, "unknown"), ProjectID: operation.ProjectID,
			OperationID: id, IdempotencyKey: idempotencyKey, Detail: []byte(`{}`), OccurredAt: now,
		})
		emitErr := c.emit(ctx, operation, "operation.cancellation_requested", map[string]any{})
		sideEffectErr = errors.Join(wrapIfPresent("record cancellation audit", auditErr), emitErr)
	}
	c.mu.Lock()
	cancel := c.active[id]
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	current, getErr := c.repo.Get(ctx, id)
	return current, errors.Join(sideEffectErr, getErr)
}

func (c *Coordinator) schedule(operation domain.Operation) {
	//nolint:gosec // G118: cancel is retained for external cancellation and deferred by the operation goroutine.
	operationCtx, cancel := context.WithCancel(c.ctx)
	c.mu.Lock()
	c.active[operation.ID] = cancel
	c.mu.Unlock()
	c.wg.Add(1)
	go func() {
		defer cancel()
		c.run(operationCtx, operation)
	}()
}

func (c *Coordinator) run(ctx context.Context, operation domain.Operation) {
	defer func() {
		c.wg.Done()
		c.mu.Lock()
		delete(c.active, operation.ID)
		c.mu.Unlock()
	}()
	release, err := c.gates.acquire(ctx, operation.ProjectID)
	if err != nil {
		c.finish(operation, domain.StateCancelled, "OPERATION_CANCELLED", "cancelled before execution")
		return
	}
	defer release()
	current, err := c.repo.Get(ctx, operation.ID)
	if err != nil {
		return
	}
	if current.CancellationRequested || ctx.Err() != nil {
		c.finish(current, domain.StateCancelled, "OPERATION_CANCELLED", "cancelled before execution")
		return
	}
	running, err := current.Transition(domain.StateRunning, c.now(), "", "")
	if err != nil || c.repo.Transition(ctx, current, running) != nil {
		return
	}
	_ = c.emit(ctx, running, "operation.running", map[string]any{"kind": running.Kind})
	err = c.execute(ctx, running)
	if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		c.finish(running, domain.StateCancelled, "OPERATION_CANCELLED", "operation cancelled")
		return
	}
	var partial *PartialSuccessError
	if errors.As(err, &partial) {
		c.finish(running, domain.StatePartiallySucceeded, "OPERATION_PARTIAL", partial.Error())
		return
	}
	if err != nil {
		c.finish(running, domain.StateFailed, "OPERATION_FAILED", err.Error())
		return
	}
	c.finish(running, domain.StateSucceeded, "", "")
}

func (c *Coordinator) execute(ctx context.Context, operation domain.Operation) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("operation executor panic: %v", recovered)
		}
	}()
	return c.executor.Execute(ctx, operation, progressReporter{coordinator: c, operation: operation})
}

// Shutdown cancels active work and waits for durable terminal transitions.
func (c *Coordinator) Shutdown(ctx context.Context) error {
	c.mu.Lock()
	for _, cancel := range c.active {
		cancel()
	}
	c.mu.Unlock()
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.wg.Wait()
	}()
	select {
	case <-ctx.Done():
		return fmt.Errorf("wait for operations shutdown: %w", ctx.Err())
	case <-done:
		return nil
	}
}

func (c *Coordinator) finish(current domain.Operation, state domain.State, code, message string) {
	next, err := current.Transition(state, c.now(), code, message)
	if err != nil {
		return
	}
	if err := c.repo.Transition(context.WithoutCancel(c.ctx), current, next); err != nil {
		return
	}
	_ = c.emit(context.WithoutCancel(c.ctx), next, "operation."+string(state), map[string]any{"errorCode": code})
}

func (c *Coordinator) emit(ctx context.Context, operation domain.Operation, eventType string, payload any) error {
	id, err := identifier.New("evt")
	if err != nil {
		return err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode operation event: %w", err)
	}
	_, err = c.journal.Publish(ctx, events.Envelope{
		ID: id, Type: eventType, OccurredAt: c.now().UTC(),
		ProjectID: operation.ProjectID, OperationID: operation.ID, Payload: body,
	})
	if err != nil {
		return fmt.Errorf("publish operation event: %w", err)
	}
	return nil
}

func defaultValue(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func wrapIfPresent(message string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

type progressReporter struct {
	coordinator *Coordinator
	operation   domain.Operation
}

func (r progressReporter) Step(ctx context.Context, name, state, message string) error {
	now := r.coordinator.now().UTC()
	if err := r.coordinator.repo.AddStep(ctx, Step{
		OperationID: r.operation.ID, Name: name, State: state,
		Message: message, OccurredAt: now,
	}); err != nil {
		return fmt.Errorf("record operation step: %w", err)
	}
	return r.coordinator.emit(ctx, r.operation, "operation.step", map[string]any{
		"name": name, "state": state, "message": message,
	})
}
