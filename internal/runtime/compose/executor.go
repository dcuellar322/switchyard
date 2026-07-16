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
	var output limitedBuffer
	runErr := e.runner.Run(ctx, validated.invocation, &output, &output)
	if runErr != nil {
		return commandError("execute Docker Compose lifecycle", runErr, output.String())
	}
	if err := streamCommandProgress(ctx, bytes.NewReader(output.Bytes()), sink); err != nil {
		return err
	}
	e.managed.RecordAction(validated.config.ProjectName, plan.Action, plan.OperationID)
	return sink.Step(ctx, "compose.execute", "succeeded", "Docker Compose lifecycle command completed")
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
