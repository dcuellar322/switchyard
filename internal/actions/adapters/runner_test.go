package adapters

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"switchyard.dev/switchyard/internal/actions/domain"
)

type launcherStub struct {
	cwd      string
	command  []string
	provider string
}

func (l *launcherStub) OpenTerminal(_ context.Context, cwd string, command []string, provider string) error {
	l.cwd, l.command, l.provider = cwd, command, provider
	return nil
}
func (*launcherStub) OpenEditor(context.Context, string, string) error { return nil }
func (*launcherStub) OpenBrowser(context.Context, string) error        { return nil }

func TestTerminalAndAgentLaunchUseResolvedWorkingDirectory(t *testing.T) {
	t.Parallel()
	launcher := &launcherStub{}
	runner := NewRunner(launcher)
	if err := runner.Run(context.Background(), domain.Execution{WorkingDirectory: "/trusted/project", Action: domain.Definition{Type: "agent.start", Provider: "codex"}}); err != nil {
		t.Fatal(err)
	}
	if launcher.cwd != "/trusted/project" || len(launcher.command) != 1 || launcher.command[0] != "codex" || launcher.provider != "" {
		t.Fatalf("launcher = %#v", launcher)
	}
}

func TestTerminalLaunchUsesSelectedProvider(t *testing.T) {
	t.Parallel()
	launcher := &launcherStub{}
	runner := NewRunner(launcher)
	if err := runner.Run(context.Background(), domain.Execution{WorkingDirectory: "/trusted/project", Action: domain.Definition{Type: "terminal.open", Provider: "iterm"}}); err != nil {
		t.Fatal(err)
	}
	if launcher.cwd != "/trusted/project" || len(launcher.command) != 0 || launcher.provider != "iterm" {
		t.Fatalf("launcher = %#v", launcher)
	}
}

func TestCommandRejectsPrivilegeEscalation(t *testing.T) {
	t.Parallel()
	err := runCommand(context.Background(), t.TempDir(), domain.Definition{}, []string{"sudo", "true"})
	if !errors.Is(err, ErrPrivilegeEscalation) {
		t.Fatalf("error = %v", err)
	}
}

func TestCommandHonorsCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := runCommand(ctx, t.TempDir(), domain.Definition{}, []string{"sh", "-c", "sleep 10"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}

func TestActionEnvironmentUsesAnExplicitAllowlist(t *testing.T) {
	const secret = "SWITCHYARD_ACTION_SECRET_CANARY"
	t.Setenv(secret, "must-not-leak")
	values := actionEnvironment(map[string]string{"SWITCHYARD_EXPLICIT_VALUE": "allowed"})
	joined := strings.Join(values, "\n")
	if strings.Contains(joined, secret+"=") {
		t.Fatalf("ambient environment leaked into action: %s", joined)
	}
	if !strings.Contains(joined, "SWITCHYARD_EXPLICIT_VALUE=allowed") {
		t.Fatalf("explicit action environment missing: %s", joined)
	}
	if _, present := os.LookupEnv(secret); !present {
		t.Fatal("test precondition failed")
	}
}
