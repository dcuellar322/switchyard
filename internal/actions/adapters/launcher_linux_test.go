//go:build linux

package adapters

import (
	"context"
	"errors"
	"testing"
)

type linuxLaunchStub struct {
	executable string
	arguments  []string
}

func (s *linuxLaunchStub) Run(_ context.Context, executable string, arguments ...string) error {
	s.executable = executable
	s.arguments = append([]string(nil), arguments...)
	return nil
}

func TestLinuxLauncherUsesArgumentArrays(t *testing.T) {
	t.Parallel()
	executor := &linuxLaunchStub{}
	launcher := platformLauncher{executor: executor, lookPath: func(name string) (string, error) {
		if name == "gnome-terminal" {
			return "/usr/bin/gnome-terminal", nil
		}
		return "", errors.New("not found")
	}}
	if err := launcher.OpenTerminal(context.Background(), "/tmp/project with spaces", []string{"codex", "--resume"}, ""); err != nil {
		t.Fatal(err)
	}
	want := []string{"--working-directory=/tmp/project with spaces", "--", "codex", "--resume"}
	if executor.executable != "/usr/bin/gnome-terminal" || len(executor.arguments) != len(want) {
		t.Fatalf("launch = %q %#v", executor.executable, executor.arguments)
	}
	for index := range want {
		if executor.arguments[index] != want[index] {
			t.Fatalf("argument %d = %q, want %q", index, executor.arguments[index], want[index])
		}
	}
}

func TestLinuxLauncherRejectsNonHTTPBrowserTargets(t *testing.T) {
	t.Parallel()
	launcher := platformLauncher{executor: &linuxLaunchStub{}}
	if err := launcher.OpenBrowser(context.Background(), "file:///etc/passwd"); err == nil {
		t.Fatal("OpenBrowser() error = nil")
	}
}
