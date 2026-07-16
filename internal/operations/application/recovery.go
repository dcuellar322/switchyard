package application

import (
	"context"
	"fmt"
	"time"

	"switchyard.dev/switchyard/internal/operations/domain"
)

// Recover marks interrupted work failed and resumes operations that never ran.
func (c *Coordinator) Recover(ctx context.Context) error {
	operations, err := c.repo.Recoverable(ctx)
	if err != nil {
		return fmt.Errorf("list recoverable operations: %w", err)
	}
	for _, operation := range operations {
		switch operation.State {
		case domain.StateRunning:
			c.finish(operation, domain.StateFailed, "DAEMON_RESTARTED", "daemon restarted during execution")
		case domain.StateQueued:
			if operation.CancellationRequested {
				c.finish(operation, domain.StateCancelled, "OPERATION_CANCELLED", "cancelled before daemon restart")
				continue
			}
			c.schedule(operation)
		case domain.StateSucceeded, domain.StateFailed, domain.StateCancelled, domain.StatePartiallySucceeded:
			continue
		}
	}
	return nil
}

// Wait blocks until the operation reaches a terminal state.
func (c *Coordinator) Wait(ctx context.Context, id string) (domain.Operation, error) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		operation, err := c.repo.Get(ctx, id)
		if err != nil {
			return domain.Operation{}, err
		}
		if operation.Terminal() {
			return operation, nil
		}
		select {
		case <-ctx.Done():
			return domain.Operation{}, ctx.Err()
		case <-ticker.C:
		}
	}
}
