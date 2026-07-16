package adapters

import (
	"context"
	"errors"
	"net/url"
	"os/exec"
)

type launchExecutor interface {
	Run(context.Context, string, ...string) error
}

type installedLauncher struct{}

func (installedLauncher) Run(ctx context.Context, executable string, arguments ...string) error {
	return exec.CommandContext(ctx, executable, arguments...).Run()
}

func validateBrowserTarget(target string) error {
	parsed, err := url.Parse(target)
	if err != nil || parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Host == "" {
		return errors.New("browser action requires an absolute HTTP or HTTPS URL")
	}
	return nil
}
