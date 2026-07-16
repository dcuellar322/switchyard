package compose

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

type executor struct {
	runner  commandRunner
	managed *managedContainers
}

func (e executor) Execute(ctx context.Context, plan domain.Plan, sink domain.ProgressSink) error {
	validated, err := unpackExecutionPlan(plan)
	if err != nil {
		return err
	}
	if err := sink.Step(ctx, "compose.preview", "succeeded", plan.Summary); err != nil {
		return err
	}
	if claimsComposeOwnership(plan.Action) {
		e.managed.RecordAction(validated.config.ProjectName, plan.Action, plan.OperationID)
	}
	var output limitedBuffer
	runErr := e.runner.Run(ctx, validated.invocation, &output, &output)
	if runErr != nil {
		if claimsComposeOwnership(plan.Action) {
			e.managed.DiscardPending(validated.config.ProjectName, plan.OperationID)
		}
		return commandError("execute Docker Compose lifecycle", runErr, output.String())
	}
	if claimsComposeOwnership(plan.Action) {
		e.managed.CompletePending(validated.config.ProjectName, plan.OperationID)
	} else {
		e.managed.RecordAction(validated.config.ProjectName, plan.Action, plan.OperationID)
	}
	if err := streamCommandProgress(ctx, bytes.NewReader(output.Bytes()), sink); err != nil {
		return err
	}
	return sink.Step(ctx, "compose.execute", "succeeded", "Docker Compose lifecycle command completed")
}

func claimsComposeOwnership(action domain.Action) bool {
	switch action {
	case domain.ActionStart, domain.ActionRestart, domain.ActionPause, domain.ActionUnpause, domain.ActionRebuild:
		return true
	case domain.ActionStop, domain.ActionTeardown:
		return false
	}
	return false
}

func streamCommandProgress(ctx context.Context, reader io.Reader, sink domain.ProgressSink) error {
	scanner := bufio.NewScanner(reader)
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, 256*1024)
	for scanner.Scan() {
		message := strings.TrimSpace(scanner.Text())
		if message == "" {
			continue
		}
		if err := sink.Step(ctx, "compose.output", "running", message); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read Docker Compose progress: %w", err)
	}
	return nil
}
