package adapters

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	actionsDomain "switchyard.dev/switchyard/internal/actions/domain"
	manifestDomain "switchyard.dev/switchyard/internal/manifest/domain"
	"switchyard.dev/switchyard/internal/terminal/application"
)

func TestComposeExecUsesDeclaredServiceAndArgumentArray(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	resolver := &Resolver{}
	plan, err := resolver.composeExec(application.LaunchPlan{ProjectID: "project", WorkingDirectory: root}, manifestDomain.Manifest{
		Runtime:  manifestDomain.Runtime{Driver: "compose", Compose: &manifestDomain.ComposeConfig{Files: []string{"compose.yaml"}, ProjectName: "declared", Context: "desktop-linux"}},
		Services: []manifestDomain.Service{{ID: "database", Source: manifestDomain.ServiceSource{ComposeService: "postgres"}}},
	}, "worktree_name", "database", "psql", "psql")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--context", "desktop-linux", "compose", "--project-directory", root, "--file", filepath.Join(root, "compose.yaml"), "--project-name", "worktree_name", "exec", "postgres", "psql"}
	if plan.Executable != "docker" || !reflect.DeepEqual(plan.Arguments, want) {
		t.Fatalf("plan = %#v, want args %#v", plan, want)
	}
}

func TestComposeExecRejectsUndeclaredServiceAndEscapingFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	resolver := &Resolver{}
	base := manifestDomain.Manifest{Runtime: manifestDomain.Runtime{Driver: "compose", Compose: &manifestDomain.ComposeConfig{Files: []string{"compose.yaml"}}}}
	if _, err := resolver.composeExec(application.LaunchPlan{WorkingDirectory: root}, base, "", "unknown", "sh", "shell"); err == nil {
		t.Fatal("undeclared service was accepted")
	}
	base.Services = []manifestDomain.Service{{ID: "api", Source: manifestDomain.ServiceSource{ComposeService: "api"}}}
	base.Runtime.Compose.Files = []string{"../compose.yaml"}
	if _, err := resolver.composeExec(application.LaunchPlan{WorkingDirectory: root}, base, "", "api", "sh", "shell"); err == nil {
		t.Fatal("escaping Compose file was accepted")
	}
}

func TestInteractiveActionRequiresRiskAndRejectsPrivilegeEscalation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	tests := []struct {
		name   string
		action actionsDomain.Definition
		err    error
	}{
		{name: "ordinary action", action: actionsDomain.Definition{ID: "console", Name: "Console", Type: "command", Risk: actionsDomain.RiskMutating, Command: []string{"bin/console"}}, err: ErrInteractiveActionRequired},
		{name: "direct sudo", action: actionsDomain.Definition{ID: "console", Name: "Console", Type: "command", Risk: actionsDomain.RiskInteractive, Command: []string{"sudo", "bin/console"}}, err: errors.New("escalation")},
		{name: "shell sudo", action: actionsDomain.Definition{ID: "console", Name: "Console", Type: "command", Risk: actionsDomain.RiskInteractive, Shell: true, Command: []string{"echo safe; sudo sh"}}, err: errors.New("escalation")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolver := &Resolver{actions: staticActions{actions: []actionsDomain.Definition{test.action}}}
			_, err := resolver.interactiveAction(context.Background(), application.LaunchPlan{ProjectID: "project", WorkingDirectory: root}, "console")
			if err == nil {
				t.Fatal("interactive action was accepted")
			}
			if errors.Is(test.err, ErrInteractiveActionRequired) && !errors.Is(err, ErrInteractiveActionRequired) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestInteractiveActionPreservesArgumentArrayAndEnvironment(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "tools"), 0o755); err != nil {
		t.Fatal(err)
	}
	action := actionsDomain.Definition{ID: "console", Name: "Console", Type: "command", Risk: actionsDomain.RiskInteractive, Command: []string{"bin/console", "--color"}, WorkingDirectory: "tools", Environment: map[string]string{"MODE": "dev"}}
	resolver := &Resolver{actions: staticActions{actions: []actionsDomain.Definition{action}}}
	plan, err := resolver.interactiveAction(context.Background(), application.LaunchPlan{ProjectID: "project", WorkingDirectory: root}, "console")
	if err != nil {
		t.Fatal(err)
	}
	expectedRoot, err := filepath.EvalSymlinks(filepath.Join(root, "tools"))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Executable != "bin/console" || !reflect.DeepEqual(plan.Arguments, []string{"--color"}) || plan.WorkingDirectory != expectedRoot || plan.Environment["MODE"] != "dev" {
		t.Fatalf("plan = %#v", plan)
	}
}

type staticActions struct{ actions []actionsDomain.Definition }

func (s staticActions) ResolveActions(context.Context, string) (actionsDomain.ProjectActions, error) {
	return actionsDomain.ProjectActions{Actions: s.actions}, nil
}
