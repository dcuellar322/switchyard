//go:build !windows

// Package processgroup contains one-shot subprocess trees across platforms.
package processgroup

import (
	"errors"
	"os/exec"
	"syscall"
)

// Ownership retains the process-group identity created for one command.
type Ownership struct{ group int }

// Configure prepares a command to own a new process group.
func Configure(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// Own captures the process group after a configured command starts.
func Own(command *exec.Cmd) (*Ownership, error) {
	if command.Process == nil {
		return nil, errors.New("process has not started")
	}
	return &Ownership{group: command.Process.Pid}, nil
}

// Terminate kills the complete process group, including surviving descendants.
func (ownership *Ownership) Terminate() error {
	if ownership == nil || ownership.group <= 0 {
		return nil
	}
	err := syscall.Kill(-ownership.group, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

// Close releases platform ownership state.
func (*Ownership) Close() error { return nil }
