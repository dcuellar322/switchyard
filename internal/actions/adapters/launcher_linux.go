//go:build linux

package adapters

import (
	"context"
	"errors"
	"os"
	"os/exec"
)

type platformLauncher struct {
	executor launchExecutor
	lookPath func(string) (string, error)
}

// NewLauncher creates the Linux terminal, editor, and browser adapter.
func NewLauncher() Launcher {
	return platformLauncher{executor: installedLauncher{}, lookPath: exec.LookPath}
}

func (l platformLauncher) OpenTerminal(ctx context.Context, workingDirectory string, command []string) error {
	lookup := l.lookPath
	if lookup == nil {
		lookup = exec.LookPath
	}
	type terminal struct {
		name string
		args func() []string
	}
	terminals := []terminal{
		{name: os.Getenv("TERMINAL"), args: func() []string { return append([]string{"--working-directory", workingDirectory, "-e"}, command...) }},
		{name: "x-terminal-emulator", args: func() []string { return append([]string{"--working-directory", workingDirectory, "-e"}, command...) }},
		{name: "gnome-terminal", args: func() []string { return append([]string{"--working-directory=" + workingDirectory, "--"}, command...) }},
		{name: "konsole", args: func() []string { return append([]string{"--workdir", workingDirectory, "-e"}, command...) }},
		{name: "kitty", args: func() []string { return append([]string{"--directory", workingDirectory}, command...) }},
	}
	for _, candidate := range terminals {
		if candidate.name == "" {
			continue
		}
		path, err := lookup(candidate.name)
		if err == nil {
			arguments := candidate.args()
			if len(command) == 0 && len(arguments) > 0 && (arguments[len(arguments)-1] == "-e" || arguments[len(arguments)-1] == "--") {
				arguments = arguments[:len(arguments)-1]
			}
			return l.executor.Run(ctx, path, arguments...)
		}
	}
	return errors.New("no supported terminal emulator was found; set TERMINAL or install x-terminal-emulator, GNOME Terminal, Konsole, or kitty")
}

func (l platformLauncher) OpenEditor(ctx context.Context, workingDirectory, provider string) error {
	if provider != "" && provider != "vscode" {
		return errors.New("unsupported editor provider")
	}
	return l.executor.Run(ctx, "code", "--new-window", workingDirectory)
}

func (l platformLauncher) OpenBrowser(ctx context.Context, target string) error {
	if err := validateBrowserTarget(target); err != nil {
		return err
	}
	return l.executor.Run(ctx, "xdg-open", target)
}
