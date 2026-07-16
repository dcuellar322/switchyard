//go:build windows

package adapters

import (
	"context"
	"errors"
)

type platformLauncher struct{ executor launchExecutor }

// NewLauncher creates the Windows Terminal, VS Code, and browser adapter.
func NewLauncher() Launcher { return platformLauncher{executor: installedLauncher{}} }

func (l platformLauncher) OpenTerminal(ctx context.Context, workingDirectory string, command []string) error {
	arguments := []string{"-d", workingDirectory}
	arguments = append(arguments, command...)
	return l.executor.Run(ctx, "wt.exe", arguments...)
}

func (l platformLauncher) OpenEditor(ctx context.Context, workingDirectory, provider string) error {
	if provider != "" && provider != "vscode" {
		return errors.New("unsupported editor provider")
	}
	return l.executor.Run(ctx, "code.cmd", "--new-window", workingDirectory)
}

func (l platformLauncher) OpenBrowser(ctx context.Context, target string) error {
	if err := validateBrowserTarget(target); err != nil {
		return err
	}
	return l.executor.Run(ctx, "rundll32.exe", "url.dll,FileProtocolHandler", target)
}
