//go:build windows

package adapters

import (
	"context"
	"errors"

	"switchyard.dev/switchyard/internal/terminal/application"
	"switchyard.dev/switchyard/internal/terminal/domain"
)

// UnixPTY reports the deliberate Phase 14 Windows capability gap.
type UnixPTY struct{}

// NewPTY returns the unsupported Windows placeholder until the ConPTY phase.
func NewPTY() *UnixPTY { return &UnixPTY{} }

// Start never fabricates a terminal on Windows.
func (*UnixPTY) Start(context.Context, application.LaunchPlan, domain.Size) (application.Process, error) {
	return nil, errors.New("embedded PTY sessions require the future Windows ConPTY adapter")
}
