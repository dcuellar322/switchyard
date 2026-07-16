//go:build darwin || linux || freebsd || openbsd || netbsd || dragonfly || solaris || aix

package adapters

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/terminal/application"
	"switchyard.dev/switchyard/internal/terminal/domain"
)

func TestUnixPTYSupportsResizeUnicodeColorAndAlternateScreen(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	process, err := NewPTY().Start(ctx, application.LaunchPlan{
		ProjectID: "project", DisplayName: "test", WorkingDirectory: t.TempDir(), Executable: "/bin/sh",
	}, domain.Size{Columns: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := process.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	if err := process.Resize(domain.Size{Columns: 132, Rows: 43}); err != nil {
		t.Fatal(err)
	}
	command := "printf '\\033[31mterminal-✓\\033[0m\\n'; printf '\\033[?1049hfullscreen\\033[?1049l\\n'; stty size; exit\n"
	if _, err := process.Write([]byte(command)); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(process)
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Wait(); err != nil {
		t.Fatal(err)
	}
	for _, expected := range [][]byte{[]byte("terminal-✓"), []byte("\x1b[31m"), []byte("\x1b[?1049h"), []byte("fullscreen"), []byte("43 132")} {
		if !bytes.Contains(output, expected) {
			t.Fatalf("PTY output does not contain %q: %q", expected, output)
		}
	}
}

func TestUnixPTYTerminateStopsProcessGroup(t *testing.T) {
	t.Parallel()
	process, err := NewPTY().Start(context.Background(), application.LaunchPlan{
		ProjectID: "project", DisplayName: "test", WorkingDirectory: t.TempDir(), Executable: "/bin/sh", Arguments: []string{"-c", "sleep 30"},
	}, domain.Size{Columns: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := process.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := process.Terminate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := process.Wait(); err == nil || !strings.Contains(err.Error(), "signal") {
		t.Fatalf("Wait() error = %v, want signal exit", err)
	}
}
