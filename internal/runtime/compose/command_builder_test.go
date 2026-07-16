package compose

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

func TestCommandBuilderDistinguishesStopAndTeardown(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	project := domain.ProjectRuntime{ProjectID: "project-1", Root: root, Compose: &domain.ComposeRuntime{Files: []string{"compose.yaml"}}}
	config := normalizedConfig{ProjectName: "fixture", Connection: dockerConnection{ContextName: "desktop-linux"}}

	stop, err := (commandBuilder{}).Build(domain.PlanRequest{Project: project, Action: domain.ActionStop}, config)
	if err != nil {
		t.Fatal(err)
	}
	if stop.Risk != domain.RiskCaution || !reflect.DeepEqual(lastArguments(stop.Commands[0], 1), []string{"stop"}) {
		t.Fatalf("stop plan = %#v", stop)
	}
	if !containsText(stop.Effects, "preserve volumes") {
		t.Fatalf("stop effects = %#v", stop.Effects)
	}

	teardown, err := (commandBuilder{}).Build(domain.PlanRequest{Project: project, Action: domain.ActionTeardown, RemoveVolumes: true}, config)
	if err != nil {
		t.Fatal(err)
	}
	if teardown.Risk != domain.RiskDestructive || !reflect.DeepEqual(lastArguments(teardown.Commands[0], 2), []string{"down", "--volumes"}) {
		t.Fatalf("teardown plan = %#v", teardown)
	}
	if !containsText(teardown.Effects, "remove named") {
		t.Fatalf("teardown effects = %#v", teardown.Effects)
	}
}

func TestComposeArgumentsRejectFileOutsideTrustedRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	project := domain.ProjectRuntime{Root: root, Compose: &domain.ComposeRuntime{Files: []string{"../compose.yaml"}}}
	if _, err := composeBaseArguments(project, dockerConnection{}, "fixture"); err == nil {
		t.Fatal("compose file outside root accepted")
	}
}

func TestCommandBuilderTargetsDeclaredServiceWithoutShell(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	project := domain.ProjectRuntime{
		ProjectID: "project-1", Root: root, Compose: &domain.ComposeRuntime{Files: []string{"compose.yaml"}},
		Services: []domain.ServiceDeclaration{{ID: "api", RuntimeName: "backend"}, {ID: "worker", RuntimeName: "worker"}},
	}
	plan, err := (commandBuilder{}).Build(domain.PlanRequest{
		Project: project, Action: domain.ActionRestart, Services: []string{"api"},
	}, normalizedConfig{ProjectName: "fixture", Services: []string{"backend", "worker"}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(lastArguments(plan.Commands[0], 2), []string{"restart", "backend"}) {
		t.Fatalf("arguments = %#v", plan.Commands[0].Arguments)
	}
	if !reflect.DeepEqual(plan.Services, []string{"api"}) {
		t.Fatalf("services = %#v", plan.Services)
	}
	if _, err := (commandBuilder{}).Build(domain.PlanRequest{
		Project: project, Action: domain.ActionRestart, Services: []string{"missing"},
	}, normalizedConfig{ProjectName: "fixture", Services: []string{"backend", "worker"}}); err == nil {
		t.Fatal("unknown service target accepted")
	}
}

func TestConfigReaderNormalizesNameAndServices(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	runner := scriptedRunner{run: func(command domain.Command, stdout, _ io.Writer) error {
		joined := strings.Join(command.Arguments, " ")
		switch {
		case strings.Contains(joined, "context inspect selected"):
			_, _ = io.WriteString(stdout, `[{"Name":"selected","Endpoints":{"docker":{"Host":"unix:///tmp/docker.sock"}},"Storage":{}}]`)
		case strings.Contains(joined, "compose") && strings.Contains(joined, "config --format json"):
			_, _ = io.WriteString(stdout, `{"name":"normalized","services":{"worker":{},"api":{}}}`)
		default:
			return errors.New("unexpected command: " + joined)
		}
		return nil
	}}
	reader := configReader{runner: runner, contexts: contextResolver{runner: runner}}
	project := domain.ProjectRuntime{Root: root, Compose: &domain.ComposeRuntime{Files: []string{"compose.yaml"}, Context: "selected"}}
	config, err := reader.Normalize(context.Background(), project)
	if err != nil {
		t.Fatal(err)
	}
	if config.ProjectName != "normalized" || !reflect.DeepEqual(config.Services, []string{"api", "worker"}) {
		t.Fatalf("config = %#v", config)
	}
	if config.Connection.ContextName != "selected" {
		t.Fatalf("connection = %#v", config.Connection)
	}
}

type scriptedRunner struct {
	run func(domain.Command, io.Writer, io.Writer) error
}

func (r scriptedRunner) Run(_ context.Context, command domain.Command, stdout, stderr io.Writer) error {
	return r.run(command, stdout, stderr)
}

func lastArguments(command domain.Command, count int) []string {
	return command.Arguments[len(command.Arguments)-count:]
}

func containsText(values []string, wanted string) bool {
	for _, value := range values {
		if strings.Contains(value, wanted) {
			return true
		}
	}
	return false
}
