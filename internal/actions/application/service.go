// Package application resolves, authorizes, executes, and audits project actions.
package application

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"switchyard.dev/switchyard/internal/actions/domain"
)

var (
	// ErrActionNotFound indicates that no trusted definition has the requested ID.
	ErrActionNotFound = errors.New("project action not found")
	// ErrConfirmationRequired prevents unconfirmed destructive execution.
	ErrConfirmationRequired = errors.New("destructive action requires explicit confirmation")
	// ErrWorkingDirEscape prevents implicit access outside the trusted root.
	ErrWorkingDirEscape = errors.New("action working directory escapes the trusted project root")
	// ErrProjectUntrusted prevents pending repository content from becoming executable.
	ErrProjectUntrusted = errors.New("project must be trusted before actions can run")
)

// ProjectSource resolves accepted actions plus their trusted repository root.
type ProjectSource interface {
	ResolveActions(context.Context, string) (domain.ProjectActions, error)
}

// Runner executes a fully resolved action through narrow platform adapters.
type Runner interface {
	Run(context.Context, domain.Execution) error
}

// AuditRepository persists redaction-safe action identity and outcome facts.
type AuditRepository interface {
	Begin(context.Context, domain.Audit) error
	Finish(context.Context, string, string, string, time.Time) error
}

// Service authorizes, resolves, audits, and executes trusted project actions.
type Service struct {
	projects ProjectSource
	runner   Runner
	audits   AuditRepository
	now      func() time.Time
}

// NewService creates the trusted-action application boundary.
func NewService(projects ProjectSource, runner Runner, audits AuditRepository) *Service {
	return &Service{projects: projects, runner: runner, audits: audits, now: time.Now}
}

// List returns accepted project actions and safe built-in quick actions.
func (s *Service) List(ctx context.Context, projectID string) (domain.ProjectActions, error) {
	return s.projects.ResolveActions(ctx, projectID)
}

// Execute is called by the durable operation coordinator and honors cancellation through ctx.
func (s *Service) Execute(ctx context.Context, operationID, projectID, actionID, actorType, actorID string, confirm, allowOutsideRoot bool) error {
	project, err := s.projects.ResolveActions(ctx, projectID)
	if err != nil {
		return err
	}
	action, ok := findAction(project.Actions, actionID)
	if !ok {
		return ErrActionNotFound
	}
	if action.Risk == domain.RiskDestructive && !confirm {
		return ErrConfirmationRequired
	}
	workingDirectory, err := ResolveWorkingDirectory(project.Root, action.WorkingDirectory, allowOutsideRoot)
	if err != nil {
		return err
	}
	auditID := "actionaudit_" + operationID
	audit := domain.Audit{
		ID: auditID, OperationID: operationID, ProjectID: projectID, ActionID: action.ID, ActionType: action.Type,
		Risk: action.Risk, ActorType: actorType, ActorID: actorID, State: "running", WorkingDirectory: workingDirectory, StartedAt: s.now().UTC(),
	}
	if err := s.audits.Begin(ctx, audit); err != nil {
		return fmt.Errorf("begin action audit: %w", err)
	}
	runErr := s.runner.Run(ctx, domain.Execution{OperationID: operationID, ProjectID: projectID, Root: project.Root, WorkingDirectory: workingDirectory, Action: action})
	state, code := "succeeded", ""
	if runErr != nil {
		state, code = "failed", "ACTION_FAILED"
		if errors.Is(runErr, context.Canceled) {
			state, code = "cancelled", "ACTION_CANCELLED"
		}
	}
	if auditErr := s.audits.Finish(context.WithoutCancel(ctx), auditID, state, code, s.now().UTC()); auditErr != nil && runErr == nil {
		return fmt.Errorf("finish action audit: %w", auditErr)
	}
	return runErr
}

// ResolveWorkingDirectory enforces the symlink-aware trusted-root boundary.
func ResolveWorkingDirectory(root, configured string, allowOutside bool) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	if configured == "" {
		configured = "."
	}
	candidate := configured
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(resolvedRoot, candidate)
	}
	candidate, err = filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("action working directory does not exist: %w", err)
		}
		return "", err
	}
	relative, err := filepath.Rel(resolvedRoot, resolved)
	if err != nil {
		return "", err
	}
	if !allowOutside && (relative == ".." || filepath.IsAbs(relative) || len(relative) > 3 && relative[:3] == ".."+string(filepath.Separator)) {
		return "", ErrWorkingDirEscape
	}
	return resolved, nil
}

func findAction(actions []domain.Definition, id string) (domain.Definition, bool) {
	for _, action := range actions {
		if action.ID == id {
			return action, true
		}
	}
	return domain.Definition{}, false
}
