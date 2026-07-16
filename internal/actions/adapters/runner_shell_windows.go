//go:build windows

package adapters

import (
	"context"
	"os/exec"
)

func platformShellCommand(ctx context.Context, script string) *exec.Cmd {
	return exec.CommandContext(ctx, "cmd.exe", "/D", "/S", "/C", script)
}
