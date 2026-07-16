//go:build !darwin && !linux && !windows

package adapters

import (
	"context"
	"errors"
)

type platformLauncher struct{}

// NewLauncher returns an explicit unsupported-platform launcher.
func NewLauncher() Launcher { return platformLauncher{} }

func (platformLauncher) OpenTerminal(context.Context, string, []string) error {
	return errors.New("external terminal launch is not yet supported on this platform")
}
func (platformLauncher) OpenEditor(context.Context, string, string) error {
	return errors.New("external editor launch is not yet supported on this platform")
}
func (platformLauncher) OpenBrowser(context.Context, string) error {
	return errors.New("external browser launch is not yet supported on this platform")
}
