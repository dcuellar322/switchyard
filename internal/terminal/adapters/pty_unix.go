//go:build darwin || linux || freebsd || openbsd || netbsd || dragonfly || solaris || aix

package adapters

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"

	"switchyard.dev/switchyard/internal/terminal/application"
	"switchyard.dev/switchyard/internal/terminal/domain"
)

// UnixPTY starts owned process groups attached to a real Unix pseudo-terminal.
type UnixPTY struct{}

// NewPTY returns the platform PTY adapter.
func NewPTY() *UnixPTY { return &UnixPTY{} }

// Start creates a PTY with explicit cwd, environment, arguments, and size.
func (*UnixPTY) Start(ctx context.Context, plan application.LaunchPlan, size domain.Size) (application.Process, error) {
	command := exec.CommandContext(ctx, plan.Executable, plan.Arguments...)
	command.Dir = plan.WorkingDirectory
	command.Env = terminalEnvironment(plan.Environment)
	file, err := pty.StartWithSize(command, &pty.Winsize{Cols: size.Columns, Rows: size.Rows})
	if err != nil {
		return nil, err
	}
	process := &unixProcess{command: command, file: file, done: make(chan struct{})}
	go func() {
		process.waitErr = command.Wait()
		close(process.done)
	}()
	return process, nil
}

type unixProcess struct {
	command *exec.Cmd
	file    *os.File
	done    chan struct{}
	waitErr error
	close   sync.Once
}

func (p *unixProcess) Read(value []byte) (int, error) {
	count, err := p.file.Read(value)
	if errors.Is(err, syscall.EIO) {
		err = io.EOF
	}
	return count, err
}

func (p *unixProcess) Write(value []byte) (int, error) { return p.file.Write(value) }

func (p *unixProcess) Resize(size domain.Size) error {
	return pty.Setsize(p.file, &pty.Winsize{Cols: size.Columns, Rows: size.Rows})
}

func (p *unixProcess) Terminate(ctx context.Context) error {
	select {
	case <-p.done:
		return nil
	default:
	}
	pid := p.command.Process.Pid
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		return ctx.Err()
	case <-timer.C:
		if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			return err
		}
		return nil
	}
}

func (p *unixProcess) Wait() error { <-p.done; return p.waitErr }
func (p *unixProcess) PID() int    { return p.command.Process.Pid }
func (p *unixProcess) Close() error {
	var err error
	p.close.Do(func() { err = p.file.Close() })
	return err
}
