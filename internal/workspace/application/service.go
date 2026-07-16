// Package application coordinates workspace persistence and lifecycle execution.
package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"switchyard.dev/switchyard/internal/foundation/identifier"
	"switchyard.dev/switchyard/internal/workspace/domain"
)

var (
	// ErrNotFound identifies an unknown workspace.
	ErrNotFound = errors.New("workspace not found")
	// ErrRevisionConflict protects concurrent workspace edits.
	ErrRevisionConflict = errors.New("workspace revision conflict")
	// ErrInvalidRequest identifies an unsupported workspace operation request.
	ErrInvalidRequest = errors.New("invalid workspace request")
)

// Repository owns durable workspace aggregates and execution snapshots.
type Repository interface {
	Create(context.Context, domain.Workspace) error
	Update(context.Context, domain.Workspace, int64) error
	Get(context.Context, string) (domain.Workspace, error)
	List(context.Context) ([]domain.Workspace, error)
	Delete(context.Context, string) error
	SaveExecution(context.Context, domain.ExecutionSummary) error
}

// ExecutionRecoverer reconciles snapshots interrupted by daemon restart.
type ExecutionRecoverer interface {
	RecoverWorkspaceExecutions(context.Context, time.Time) error
}

// ProjectOperator is the consumer-owned boundary to project lifecycle use cases.
type ProjectOperator interface {
	Start(context.Context, string) error
	Stop(context.Context, string, StopOptions) error
}

// HealthGate waits until a started project is safe for its dependents.
type HealthGate interface {
	WaitHealthy(context.Context, string, time.Duration) error
}

// MemberValidator verifies that a referenced project or environment is eligible for coordination.
type MemberValidator interface {
	ValidateWorkspaceMember(context.Context, string) error
}

// ProgressReporter exposes per-project transitions to a durable outer operation.
type ProgressReporter interface {
	ProjectProgress(context.Context, domain.ProjectResult) error
}

// StopOptions keeps ordinary workspace stop separate from destructive teardown.
type StopOptions struct {
	RemoveData bool
}

// SaveRequest is the editable portion of a workspace aggregate.
type SaveRequest struct {
	Name                 string
	Description          string
	DefaultFailurePolicy domain.FailurePolicy
	DefaultProfileID     string
	Members              []domain.Member
	Dependencies         []domain.Dependency
	Recipes              []domain.Recipe
	Profiles             []domain.Profile
	Revision             int64
}

// ExecuteRequest selects a synchronous coordinated lifecycle operation.
type ExecuteRequest struct {
	Kind       domain.ExecutionKind
	Policy     domain.FailurePolicy
	ProfileID  string
	RemoveData bool
}

// ExecutionError retains a fully persisted result for failed or partial runs.
type ExecutionError struct {
	Summary domain.ExecutionSummary
	cause   error
	partial bool
}

func (e *ExecutionError) Error() string {
	return e.cause.Error()
}

// Unwrap exposes the underlying cancellation or project error.
func (e *ExecutionError) Unwrap() error { return e.cause }

// Partial reports whether independent branches made useful progress.
func (e *ExecutionError) Partial() bool { return e.partial }

// Service is the workspace command/query and orchestration boundary.
type Service struct {
	repository Repository
	projects   ProjectOperator
	health     HealthGate
	members    MemberValidator
	now        func() time.Time
}

// NewService creates workspace use cases with explicit cross-domain consumers.
func NewService(repository Repository, projects ProjectOperator, health HealthGate, members MemberValidator) *Service {
	return &Service{repository: repository, projects: projects, health: health, members: members, now: time.Now}
}

// Recover marks interrupted workspace executions honestly before the outer
// durable operation kernel recovers queued work.
func (s *Service) Recover(ctx context.Context) error {
	recoverer, ok := s.repository.(ExecutionRecoverer)
	if !ok {
		return errors.New("workspace execution repository does not support recovery")
	}
	return recoverer.RecoverWorkspaceExecutions(ctx, s.now().UTC())
}

// Create validates and persists a new workspace.
func (s *Service) Create(ctx context.Context, request SaveRequest) (domain.Workspace, error) {
	id, err := identifier.New("workspace")
	if err != nil {
		return domain.Workspace{}, err
	}
	now := s.now().UTC()
	workspace := request.workspace(id, 1, now, now)
	if err := workspace.Validate(); err != nil {
		return domain.Workspace{}, err
	}
	if err := s.validateMembers(ctx, workspace.Members); err != nil {
		return domain.Workspace{}, err
	}
	if err := s.repository.Create(ctx, workspace); err != nil {
		return domain.Workspace{}, fmt.Errorf("create workspace: %w", err)
	}
	return workspace, nil
}

// Update applies an optimistic revision-checked workspace replacement.
func (s *Service) Update(ctx context.Context, id string, request SaveRequest) (domain.Workspace, error) {
	if id == "" || request.Revision < 1 {
		return domain.Workspace{}, ErrInvalidRequest
	}
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return domain.Workspace{}, err
	}
	workspace := request.workspace(id, request.Revision+1, current.CreatedAt, s.now().UTC())
	workspace.LastRun = current.LastRun
	if err := workspace.Validate(); err != nil {
		return domain.Workspace{}, err
	}
	if err := s.validateMembers(ctx, workspace.Members); err != nil {
		return domain.Workspace{}, err
	}
	if err := s.repository.Update(ctx, workspace, request.Revision); err != nil {
		return domain.Workspace{}, fmt.Errorf("update workspace: %w", err)
	}
	return workspace, nil
}

// Get returns one durable workspace including its latest execution status.
func (s *Service) Get(ctx context.Context, id string) (domain.Workspace, error) {
	if strings.TrimSpace(id) == "" {
		return domain.Workspace{}, ErrInvalidRequest
	}
	return s.repository.Get(ctx, id)
}

// List returns durable workspaces in stable display order.
func (s *Service) List(ctx context.Context) ([]domain.Workspace, error) {
	return s.repository.List(ctx)
}

// Delete removes workspace coordination metadata, never project or runtime data.
func (s *Service) Delete(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrInvalidRequest
	}
	if err := s.repository.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete workspace: %w", err)
	}
	return nil
}

func (r SaveRequest) workspace(id string, revision int64, createdAt, updatedAt time.Time) domain.Workspace {
	policy := r.DefaultFailurePolicy
	if policy == "" {
		policy = domain.FailurePolicyRollback
	}
	return domain.Workspace{
		ID: id, Name: strings.TrimSpace(r.Name), Description: strings.TrimSpace(r.Description),
		DefaultFailurePolicy: policy, DefaultProfileID: r.DefaultProfileID,
		Members: slices.Clone(r.Members), Dependencies: slices.Clone(r.Dependencies),
		Recipes: cloneRecipes(r.Recipes), Profiles: cloneProfiles(r.Profiles),
		Revision: revision, CreatedAt: createdAt, UpdatedAt: updatedAt,
	}
}

func cloneProfiles(source []domain.Profile) []domain.Profile {
	result := slices.Clone(source)
	for index := range result {
		result[index].ProjectIDs = slices.Clone(result[index].ProjectIDs)
	}
	return result
}

func cloneRecipes(source []domain.Recipe) []domain.Recipe {
	result := slices.Clone(source)
	for index := range result {
		result[index].Arguments = slices.Clone(result[index].Arguments)
	}
	return result
}

func (s *Service) validateMembers(ctx context.Context, members []domain.Member) error {
	if s.members == nil {
		return fmt.Errorf("%w: workspace member validator is unavailable", ErrInvalidRequest)
	}
	for _, member := range members {
		if err := s.members.ValidateWorkspaceMember(ctx, member.ProjectID); err != nil {
			return fmt.Errorf("validate workspace member %s: %w", member.ProjectID, err)
		}
	}
	return nil
}
