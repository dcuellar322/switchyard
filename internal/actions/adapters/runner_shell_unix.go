//go:build !windows

package adapters

import (
	"context"
	"os/exec"
)

func platformShellCommand(ctx context.Context, script string) *exec.Cmd {
	return exec.CommandContext(ctx, "/bin/sh", "-lc", script)
}
