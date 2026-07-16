package mcpserver

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

const operationPollInterval = 250 * time.Millisecond

func (s *Server) projectHealthWait(ctx context.Context, request *mcp.CallToolRequest, input healthWaitInput) (*mcp.CallToolResult, healthWaitOutput, error) {
	if err := s.validateProjectRead(input.ProjectID); err != nil {
		return nil, healthWaitOutput{}, err
	}
	timeoutSeconds, err := bounded(input.TimeoutSeconds, 15, 30, "timeoutSeconds")
	if err != nil {
		return nil, healthWaitOutput{}, err
	}
	deadline := time.NewTimer(time.Duration(timeoutSeconds) * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	started := time.Now()
	for {
		health, readErr := s.backend.Health(ctx, input.ProjectID)
		if readErr != nil {
			return nil, healthWaitOutput{}, readErr
		}
		if health.Status == generated.ProjectHealthStatusHealthy {
			return nil, healthWaitOutput{SchemaVersion: schemaVersion, Health: health, Healthy: true}, nil
		}
		s.notifyWaitProgress(ctx, request, "project health is "+string(health.Status), started, time.Duration(timeoutSeconds)*time.Second)
		select {
		case <-ctx.Done():
			return nil, healthWaitOutput{}, ctx.Err()
		case <-deadline.C:
			return nil, healthWaitOutput{SchemaVersion: schemaVersion, Health: health, TimedOut: true}, nil
		case <-ticker.C:
		}
	}
}

func (s *Server) operationWait(ctx context.Context, request *mcp.CallToolRequest, input operationWaitInput) (*mcp.CallToolResult, operationWaitOutput, error) {
	if err := required(input.OperationID, "operationId"); err != nil {
		return nil, operationWaitOutput{}, err
	}
	timeoutSeconds, err := bounded(input.TimeoutSeconds, 10, 30, "timeoutSeconds")
	if err != nil {
		return nil, operationWaitOutput{}, err
	}

	deadline := time.NewTimer(time.Duration(timeoutSeconds) * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(operationPollInterval)
	defer ticker.Stop()

	started := time.Now()
	for {
		operation, readErr := s.backend.Operation(ctx, input.OperationID)
		if readErr != nil {
			return nil, operationWaitOutput{}, readErr
		}
		if readErr = s.scope.AuthorizeRead(operation.ProjectId); readErr != nil {
			return nil, operationWaitOutput{}, readErr
		}
		if operationTerminal(operation.State) {
			return nil, operationWaitOutput{SchemaVersion: schemaVersion, Operation: operation, Terminal: true}, nil
		}
		s.notifyProgress(ctx, request, operation, started, time.Duration(timeoutSeconds)*time.Second)

		select {
		case <-ctx.Done():
			return nil, operationWaitOutput{}, ctx.Err()
		case <-deadline.C:
			return nil, operationWaitOutput{SchemaVersion: schemaVersion, Operation: operation, TimedOut: true}, nil
		case <-ticker.C:
		}
	}
}

func (s *Server) notifyProgress(ctx context.Context, request *mcp.CallToolRequest, operation generated.Operation, started time.Time, timeout time.Duration) {
	s.notifyWaitProgress(ctx, request, "operation "+operation.Id+" is "+string(operation.State), started, timeout)
}

func (s *Server) notifyWaitProgress(ctx context.Context, request *mcp.CallToolRequest, message string, started time.Time, timeout time.Duration) {
	if request == nil || request.Session == nil || request.Params == nil {
		return
	}
	token := request.Params.GetProgressToken()
	if token == nil {
		return
	}
	progress := time.Since(started).Seconds()
	total := timeout.Seconds()
	if progress > total {
		progress = total
	}
	_ = request.Session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{
		ProgressToken: token,
		Progress:      progress,
		Total:         total,
		Message:       message,
	})
}

func operationTerminal(state generated.OperationState) bool {
	switch state {
	case generated.OperationStateSucceeded, generated.OperationStateFailed,
		generated.OperationStateCancelled, generated.OperationStatePartiallySucceeded:
		return true
	case generated.OperationStateQueued, generated.OperationStateRunning:
		return false
	}
	return false
}
