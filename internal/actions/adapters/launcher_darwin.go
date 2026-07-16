//go:build darwin

package adapters

import (
	"context"
	"errors"
	"strings"
)

type platformLauncher struct{ executor launchExecutor }

// NewLauncher creates the native macOS terminal, editor, and browser adapter.
func NewLauncher() Launcher { return platformLauncher{executor: installedLauncher{}} }

func (l platformLauncher) OpenTerminal(ctx context.Context, workingDirectory string, command []string) error {
	if len(command) == 0 {
		return l.executor.Run(ctx, "open", "-a", "Terminal", workingDirectory)
	}
	shellCommand := "cd " + shellQuote(workingDirectory) + " && exec " + shellJoin(command)
	script := `tell application "Terminal" to do script "` + appleScriptQuote(shellCommand) + `"` + "\n" + `tell application "Terminal" to activate`
	return l.executor.Run(ctx, "osascript", "-e", script)
}

func (l platformLauncher) OpenEditor(ctx context.Context, workingDirectory, provider string) error {
	if provider != "" && provider != "vscode" {
		return errors.New("unsupported editor provider")
	}
	return l.executor.Run(ctx, "open", "-a", "Visual Studio Code", workingDirectory)
}

func (l platformLauncher) OpenBrowser(ctx context.Context, target string) error {
	if err := validateBrowserTarget(target); err != nil {
		return err
	}
	return l.executor.Run(ctx, "open", target)
}

func shellJoin(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, shellQuote(value))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'" }

func appleScriptQuote(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), `"`, `\"`)
}
