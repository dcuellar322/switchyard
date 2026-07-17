//go:build darwin

package adapters

import (
	"context"
	"strings"
	"testing"
)

type launchExecutorStub struct {
	executable string
	arguments  []string
}

func (s *launchExecutorStub) Run(_ context.Context, executable string, arguments ...string) error {
	s.executable, s.arguments = executable, append([]string(nil), arguments...)
	return nil
}

func TestMacTerminalLaunchUsesExactWorkingDirectory(t *testing.T) {
	t.Parallel()
	executor := &launchExecutorStub{}
	launcher := platformLauncher{executor: executor}
	workingDirectory := "/tmp/project with spaces"
	if err := launcher.OpenTerminal(context.Background(), workingDirectory, nil); err != nil {
		t.Fatal(err)
	}
	if executor.executable != "osascript" || !strings.Contains(executor.arguments[1], "cd '/tmp/project with spaces'") {
		t.Fatalf("launch = %q %#v", executor.executable, executor.arguments)
	}
	if err := launcher.OpenTerminal(context.Background(), workingDirectory, []string{"codex"}); err != nil {
		t.Fatal(err)
	}
	if executor.executable != "osascript" || !strings.Contains(executor.arguments[1], "cd '/tmp/project with spaces' && exec 'codex'") {
		t.Fatalf("agent launch = %q %#v", executor.executable, executor.arguments)
	}
}
