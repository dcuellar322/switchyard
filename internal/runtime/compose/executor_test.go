package compose

import (
	"context"
	"io"
	"testing"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

func TestExecutorUsesDurableProgressStates(t *testing.T) {
	t.Parallel()
	runner := scriptedRunner{run: func(domain.Command, io.Writer, io.Writer) error { return nil }}
	project := domain.ProjectRuntime{ProjectID: "project-1"}
	command := domain.Command{Executable: "docker"}
	plan := domain.Plan{
		ProjectID: "project-1", OperationID: "op-1", Driver: domain.KindCompose, Action: domain.ActionStart,
		Summary: "start", Commands: []domain.Command{command},
		DriverData: executionPlan{project: project, config: normalizedConfig{ProjectName: "fixture"}, invocation: command},
	}
	sink := &progressRecorder{}
	managed := newManagedContainers()
	if err := (executor{runner: runner, managed: managed}).Execute(context.Background(), plan, sink); err != nil {
		t.Fatal(err)
	}
	if len(sink.states) != 2 || sink.states[0] != "succeeded" || sink.states[1] != "succeeded" {
		t.Fatalf("states = %#v", sink.states)
	}
	if managed.Operation("fixture") != "op-1" {
		t.Fatalf("operation = %q", managed.Operation("fixture"))
	}
}

type progressRecorder struct{ states []string }

func (s *progressRecorder) Step(_ context.Context, _, state, _ string) error {
	s.states = append(s.states, state)
	return nil
}
