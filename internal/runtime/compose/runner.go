package compose

import (
	"context"
	"io"
	"os/exec"
	"time"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

type commandRunner interface {
	Run(context.Context, domain.Command, io.Writer, io.Writer) error
}

type osCommandRunner struct{}

func (osCommandRunner) Run(ctx context.Context, command domain.Command, stdout, stderr io.Writer) error {
	process := exec.CommandContext(ctx, command.Executable, command.Arguments...)
	process.Dir = command.WorkingDirectory
	process.Stdout = stdout
	process.Stderr = stderr
	process.WaitDelay = 2 * time.Second
	return process.Run()
}
