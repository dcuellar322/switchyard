package compose

import (
	"context"
	"errors"
	"io"
	"testing"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

func TestExecutorUsesDurableProgressStates(t *testing.T) {
	t.Parallel()
	managed := newManagedContainers()
	pendingAtRun := false
	runner := scriptedRunner{run: func(domain.Command, io.Writer, io.Writer) error {
		managed.mu.RLock()
		_, pendingAtRun = managed.pending["fixture"]
		managed.mu.RUnlock()
		return nil
	}}
	project := domain.ProjectRuntime{ProjectID: "project-1"}
	command := domain.Command{Executable: "docker"}
	plan := domain.Plan{
		ProjectID: "project-1", OperationID: "op-1", Driver: domain.KindCompose, Action: domain.ActionStart,
		Summary: "start", Commands: []domain.Command{command},
		DriverData: executionPlan{project: project, config: normalizedConfig{ProjectName: "fixture"}, invocation: command},
	}
	sink := &progressRecorder{}
	if err := (executor{runner: runner, managed: managed}).Execute(context.Background(), plan, sink); err != nil {
		t.Fatal(err)
	}
	if len(sink.states) != 2 || sink.states[0] != "succeeded" || sink.states[1] != "succeeded" {
		t.Fatalf("states = %#v", sink.states)
	}
	if managed.Operation("fixture") != "op-1" {
		t.Fatalf("operation = %q", managed.Operation("fixture"))
	}
	if token := managed.OwnershipToken("fixture"); !token.ready {
		t.Fatal("successful Compose command did not make ownership ready for final reconciliation")
	}
	if !pendingAtRun {
		t.Fatal("ownership intent was not visible while the Compose command ran")
	}
}

func TestExecutorDiscardsOwnershipIntentWhenComposeCommandFails(t *testing.T) {
	t.Parallel()
	managed := newManagedContainers()
	runner := scriptedRunner{run: func(domain.Command, io.Writer, io.Writer) error { return errors.New("compose failed") }}
	command := domain.Command{Executable: "docker"}
	plan := domain.Plan{
		ProjectID: "project-1", OperationID: "op-1", Driver: domain.KindCompose, Action: domain.ActionStart,
		Summary: "start", Commands: []domain.Command{command},
		DriverData: executionPlan{config: normalizedConfig{ProjectName: "fixture"}, invocation: command},
	}
	if err := (executor{runner: runner, managed: managed}).Execute(context.Background(), plan, &progressRecorder{}); err == nil {
		t.Fatal("expected Compose command failure")
	}
	managed.mu.RLock()
	_, pending := managed.pending["fixture"]
	managed.mu.RUnlock()
	if pending || managed.Operation("fixture") != "" {
		t.Fatal("failed Compose command retained ownership intent")
	}
}

type progressRecorder struct{ states []string }

func (s *progressRecorder) Step(_ context.Context, _, state, _ string) error {
	s.states = append(s.states, state)
	return nil
}
