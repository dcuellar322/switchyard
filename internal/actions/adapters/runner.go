// Package adapters executes reviewed actions through narrow operating-system capabilities.
package adapters

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"switchyard.dev/switchyard/internal/actions/domain"
)

// ErrPrivilegeEscalation rejects action definitions that invoke sudo or doas.
var ErrPrivilegeEscalation = errors.New("actions may not invoke privilege escalation")

// Launcher exposes reviewed native desktop capabilities to the action runner.
type Launcher interface {
	OpenTerminal(context.Context, string, []string) error
	OpenEditor(context.Context, string, string) error
	OpenBrowser(context.Context, string) error
}

// Runner dispatches typed launch actions and shell-free commands.
type Runner struct{ launcher Launcher }

// NewRunner creates a dispatcher for typed actions and bounded commands.
func NewRunner(launcher Launcher) *Runner { return &Runner{launcher: launcher} }

// Run executes one fully authorized and resolved action.
func (r *Runner) Run(ctx context.Context, execution domain.Execution) error {
	timeout := time.Duration(execution.Action.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	action := execution.Action
	switch action.Type {
	case "terminal.open":
		return r.launcher.OpenTerminal(ctx, execution.WorkingDirectory, nil)
	case "editor.open":
		return r.launcher.OpenEditor(ctx, execution.WorkingDirectory, action.Provider)
	case "browser.open":
		return r.launcher.OpenBrowser(ctx, action.Target)
	case "agent.start":
		if action.Provider != "codex" && action.Provider != "claude" {
			return fmt.Errorf("unsupported agent provider %q", action.Provider)
		}
		return r.launcher.OpenTerminal(ctx, execution.WorkingDirectory, []string{action.Provider})
	case "git.fetch", "git.pull", "git.push":
		command := action.Command
		if len(command) == 0 {
			command = []string{"git", strings.TrimPrefix(action.Type, "git.")}
		}
		return runCommand(ctx, execution.WorkingDirectory, action, command)
	case "command", "command.run", "tests.run", "migration.run":
		return runCommand(ctx, execution.WorkingDirectory, action, action.Command)
	default:
		return fmt.Errorf("unsupported action type %q", action.Type)
	}
}

func runCommand(ctx context.Context, workingDirectory string, action domain.Definition, arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("action command is empty")
	}
	var command *exec.Cmd
	if action.Shell {
		if len(arguments) != 1 {
			return errors.New("explicit shell action requires one command string")
		}
		if containsEscalation(arguments[0]) {
			return ErrPrivilegeEscalation
		}
		command = platformShellCommand(ctx, arguments[0])
	} else {
		if arguments[0] == "sudo" || arguments[0] == "doas" {
			return ErrPrivilegeEscalation
		}
		command = exec.CommandContext(ctx, arguments[0], arguments[1:]...)
	}
	command.Dir = workingDirectory
	command.Env = actionEnvironment(action.Environment)
	if action.CaptureOutput {
		var output cappedBuffer
		command.Stdout, command.Stderr = &output, &output
	} else {
		command.Stdout, command.Stderr = io.Discard, io.Discard
	}
	if err := command.Run(); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return ctx.Err()
		}
		return errors.New("action command failed")
	}
	return nil
}

var escalationPattern = regexp.MustCompile(`(^|[;&|[:space:]])(sudo|doas)([[:space:]]|$)`)

func containsEscalation(command string) bool { return escalationPattern.MatchString(command) }

func actionEnvironment(overlay map[string]string) []string {
	allowed := map[string]bool{
		"PATH": true, "HOME": true, "TMPDIR": true, "USER": true, "SHELL": true, "LANG": true, "LC_ALL": true, "TERM": true,
		"SystemRoot": true, "ComSpec": true, "USERPROFILE": true, "PATHEXT": true, "TEMP": true, "TMP": true, "LOCALAPPDATA": true,
	}
	values := make(map[string]string, len(allowed)+len(overlay))
	for _, entry := range os.Environ() {
		key, value, found := strings.Cut(entry, "=")
		if found && allowed[key] {
			values[key] = value
		}
	}
	for key, value := range overlay {
		values[key] = value
	}
	result := make([]string, 0, len(values))
	for key, value := range values {
		result = append(result, key+"="+value)
	}
	return result
}

type cappedBuffer struct{ bytes.Buffer }

func (b *cappedBuffer) Write(value []byte) (int, error) {
	const maximum = 1 << 20
	original := len(value)
	remaining := maximum - b.Len()
	if remaining > 0 {
		_, _ = b.Buffer.Write(value[:min(remaining, len(value))])
	}
	return original, nil
}
