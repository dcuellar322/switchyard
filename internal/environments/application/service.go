// Package application coordinates registration of independently runnable
// project environments from trusted Git worktree observations.
package application

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"switchyard.dev/switchyard/internal/environments/domain"
	routingDomain "switchyard.dev/switchyard/internal/routing/domain"
)

var (
	// ErrNotFound identifies an unknown registered project environment.
	ErrNotFound = errors.New("project environment not found")
	// ErrProjectUntrusted prevents worktree registration before catalog trust.
	ErrProjectUntrusted = errors.New("project must be trusted before environment registration")
	// ErrNoWorktrees distinguishes an unavailable inventory from a successful
	// registration with fabricated locations.
	ErrNoWorktrees = errors.New("git returned no worktrees for the project")
	// ErrAllocationExhausted reports that no unique logical port offset remains.
	ErrAllocationExhausted = errors.New("environment port-offset namespace is exhausted")
	// ErrRuntimeConflict protects friendly hostnames and exact host-port leases.
	ErrRuntimeConflict = errors.New("environment runtime allocation conflicts with another environment")
)

// ProjectDescriptor is the bounded catalog information needed to register
// locations. Trust enforcement belongs to the ProjectSource adapter.
type ProjectDescriptor struct {
	ID              string
	Slug            string
	PrimaryLocation string
}

// WorktreeObservation is repository metadata only. It contains no commands or
// environment values and is safe to persist after explicit observation.
type WorktreeObservation struct {
	Path     string
	Head     string
	Branch   string
	Detached bool
	Bare     bool
	Locked   bool
}

// ProjectSource resolves one trusted project without exposing catalog storage.
type ProjectSource interface {
	Project(context.Context, string) (ProjectDescriptor, error)
}

// WorktreeSource reads Git administrative metadata without repository code.
type WorktreeSource interface {
	ListWorktrees(context.Context, string) ([]WorktreeObservation, error)
}

// EnvironmentRepository owns registered environments behind an explicit
// application port. Durable adapters may persist the record without exposing
// their tables to source control, routing, or runtime domains.
type EnvironmentRepository interface {
	List(context.Context) ([]domain.Environment, error)
	Get(context.Context, string) (domain.Environment, error)
	ReplaceProject(context.Context, string, []domain.Environment) error
	Update(context.Context, domain.Environment) error
}

// Registration summarizes one complete worktree reconciliation.
type Registration struct {
	ProjectID    string               `json:"projectId"`
	Environments []domain.Environment `json:"environments"`
	RemovedIDs   []string             `json:"removedIds"`
	ObservedAt   time.Time            `json:"observedAt"`
}

// Service turns explicit worktree observations into stable runtime identities.
type Service struct {
	projects   ProjectSource
	worktrees  WorktreeSource
	repository EnvironmentRepository
	now        func() time.Time
}

// NewService creates the project-environment registration boundary.
func NewService(projects ProjectSource, worktrees WorktreeSource, repository EnvironmentRepository) *Service {
	return &Service{projects: projects, worktrees: worktrees, repository: repository, now: time.Now}
}

// RuntimeConfiguration is the narrow mutation used after a runtime resolves
// exact ports and an optional HTTP endpoint.
type RuntimeConfiguration struct {
	State      domain.State
	Hostname   string
	Target     string
	PortLeases []domain.PortLease
}

// Get resolves one environment for runtime planning.
func (s *Service) Get(ctx context.Context, environmentID string) (domain.Environment, error) {
	return s.repository.Get(ctx, environmentID)
}

// List returns all registered environment records for routing reconciliation.
func (s *Service) List(ctx context.Context) ([]domain.Environment, error) {
	return s.repository.List(ctx)
}

// ConfigureRuntime records a validated loopback HTTP route and exact per-port
// leases. It performs no lifecycle action and never starts a process.
func (s *Service) ConfigureRuntime(ctx context.Context, environmentID string, configuration RuntimeConfiguration) (domain.Environment, error) {
	environment, err := s.repository.Get(ctx, environmentID)
	if err != nil {
		return domain.Environment{}, err
	}
	if configuration.Hostname == "" {
		configuration.Hostname = environment.Hostname
	}
	hostname, err := routingDomain.NormalizeHostname(configuration.Hostname)
	if err != nil {
		return domain.Environment{}, err
	}
	if configuration.Target != "" {
		if _, err := routingDomain.ValidateTarget(configuration.Target); err != nil {
			return domain.Environment{}, err
		}
	}
	environment.State = configuration.State
	environment.Hostname = hostname
	environment.Target = configuration.Target
	environment.Allocation.PortLeases = append([]domain.PortLease(nil), configuration.PortLeases...)
	environment.UpdatedAt = s.now().UTC()
	if err := environment.Validate(); err != nil {
		return domain.Environment{}, err
	}
	all, err := s.repository.List(ctx)
	if err != nil {
		return domain.Environment{}, err
	}
	if err := checkRuntimeConflicts(environment, all); err != nil {
		return domain.Environment{}, err
	}
	if err := s.repository.Update(ctx, environment); err != nil {
		return domain.Environment{}, fmt.Errorf("update environment runtime: %w", err)
	}
	return environment, nil
}

// RegisterWorktrees reconciles the current Git worktree inventory. It never
// executes repository-defined commands; the source adapter may only inspect
// Git administrative metadata after the catalog trust check.
func (s *Service) RegisterWorktrees(ctx context.Context, projectID string) (Registration, error) {
	project, err := s.projects.Project(ctx, projectID)
	if err != nil {
		return Registration{}, fmt.Errorf("resolve trusted project: %w", err)
	}
	observed, err := s.worktrees.ListWorktrees(ctx, projectID)
	if err != nil {
		return Registration{}, fmt.Errorf("inspect project worktrees: %w", err)
	}
	if len(observed) == 0 {
		return Registration{}, ErrNoWorktrees
	}
	existing, err := s.repository.List(ctx)
	if err != nil {
		return Registration{}, fmt.Errorf("list registered environments: %w", err)
	}
	observedAt := s.now().UTC()
	environments, err := buildEnvironments(ctx, project, observed, existing, observedAt)
	if err != nil {
		return Registration{}, err
	}
	if err := s.repository.ReplaceProject(ctx, project.ID, environments); err != nil {
		return Registration{}, fmt.Errorf("persist registered environments: %w", err)
	}
	return Registration{
		ProjectID: project.ID, Environments: environments,
		RemovedIDs: removedEnvironmentIDs(existing, project.ID, environments), ObservedAt: observedAt,
	}, nil
}

// ListProject returns registered environments in stable display order.
func (s *Service) ListProject(ctx context.Context, projectID string) ([]domain.Environment, error) {
	all, err := s.repository.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domain.Environment, 0)
	for _, environment := range all {
		if environment.ProjectID == projectID {
			result = append(result, environment)
		}
	}
	sortEnvironments(result)
	return result, nil
}

func buildEnvironments(
	ctx context.Context,
	project ProjectDescriptor,
	observed []WorktreeObservation,
	existing []domain.Environment,
	observedAt time.Time,
) ([]domain.Environment, error) {
	if project.ID == "" || project.Slug == "" || !filepath.IsAbs(project.PrimaryLocation) {
		return nil, errors.New("trusted project descriptor is invalid")
	}
	existingByID, usedOffsets := existingAllocations(existing, project.ID)
	candidates, err := worktreeCandidates(project, observed, observedAt)
	if err != nil {
		return nil, err
	}
	for index := range candidates {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		candidate := &candidates[index]
		if previous, ok := existingByID[candidate.ID]; ok {
			candidate.RegisteredAt = previous.RegisteredAt
			candidate.Allocation.ComposeProjectName = previous.Allocation.ComposeProjectName
			candidate.Allocation.PortLeaseNamespace = previous.Allocation.PortLeaseNamespace
			if candidate.Availability == domain.AvailabilityAvailable {
				candidate.State = previous.State
				candidate.Hostname = previous.Hostname
				candidate.Target = previous.Target
				candidate.Allocation.PortLeases = append([]domain.PortLease(nil), previous.Allocation.PortLeases...)
			}
			if reserveOffset(previous.Allocation.PortOffset, usedOffsets) {
				candidate.Allocation.PortOffset = previous.Allocation.PortOffset
				continue
			}
		}
		offset, err := allocateOffset(domain.PortOffsetSeed(candidate.ID), usedOffsets)
		if err != nil {
			return nil, err
		}
		candidate.Allocation.PortOffset = offset
	}
	if err := validateAllocations(candidates); err != nil {
		return nil, err
	}
	sortEnvironments(candidates)
	return candidates, nil
}

func worktreeCandidates(project ProjectDescriptor, observed []WorktreeObservation, at time.Time) ([]domain.Environment, error) {
	projectRelativePath, err := projectPathRelativeToWorktree(project.PrimaryLocation, observed)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(observed))
	result := make([]domain.Environment, 0, len(observed))
	for _, worktree := range observed {
		worktreePath := filepath.Clean(strings.TrimSpace(worktree.Path))
		if !filepath.IsAbs(worktreePath) {
			return nil, fmt.Errorf("worktree path must be absolute: %q", worktree.Path)
		}
		projectPath := worktreePath
		if projectRelativePath != "." {
			projectPath = filepath.Join(worktreePath, projectRelativePath)
		}
		id, err := domain.StableID(project.ID, projectPath)
		if err != nil {
			return nil, err
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		environment, err := newEnvironment(project, worktree, id, worktreePath, projectPath, at)
		if err != nil {
			return nil, err
		}
		result = append(result, environment)
	}
	return result, nil
}

func projectPathRelativeToWorktree(primaryLocation string, observed []WorktreeObservation) (string, error) {
	primaryLocation = filepath.Clean(primaryLocation)
	for _, worktree := range observed {
		worktreePath := filepath.Clean(strings.TrimSpace(worktree.Path))
		if !filepath.IsAbs(worktreePath) {
			continue
		}
		relative, err := filepath.Rel(worktreePath, primaryLocation)
		if err != nil {
			continue
		}
		if relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
			return relative, nil
		}
	}
	return "", errors.New("trusted project location is not contained by an observed Git worktree")
}

func newEnvironment(project ProjectDescriptor, worktree WorktreeObservation, id, worktreePath, projectPath string, at time.Time) (domain.Environment, error) {
	branch := strings.TrimSpace(worktree.Branch)
	name := branch
	if name == "" {
		name = filepath.Base(worktreePath)
	}
	composeName, err := domain.ComposeProjectName(project.Slug, name, id)
	if err != nil {
		return domain.Environment{}, err
	}
	namespace, err := domain.PortLeaseNamespace(id)
	if err != nil {
		return domain.Environment{}, err
	}
	hostname, err := domain.LocalhostName(project.Slug, id)
	if err != nil {
		return domain.Environment{}, err
	}
	environment := domain.Environment{
		ID: id, ProjectID: project.ID, Name: name, Path: projectPath, Head: worktree.Head, Branch: branch,
		Detached: worktree.Detached, Bare: worktree.Bare, Locked: worktree.Locked,
		Primary:      filepath.Clean(project.PrimaryLocation) == projectPath,
		Availability: domain.AvailabilityAvailable, State: domain.StateRegistered, Hostname: hostname,
		Allocation:   domain.RuntimeAllocation{ComposeProjectName: composeName, PortLeaseNamespace: namespace, PortLeases: []domain.PortLease{}},
		RegisteredAt: at, LastObservedAt: at, UpdatedAt: at,
	}
	if worktree.Bare {
		environment.Availability = domain.AvailabilityUnavailable
		environment.State = domain.StateUnavailable
		environment.UnavailableReason = "bare Git worktrees cannot run project services"
	}
	return environment, nil
}

func removedEnvironmentIDs(existing []domain.Environment, projectID string, current []domain.Environment) []string {
	remaining := make(map[string]struct{}, len(current))
	for _, environment := range current {
		remaining[environment.ID] = struct{}{}
	}
	var removed []string
	for _, environment := range existing {
		if environment.ProjectID == projectID {
			if _, exists := remaining[environment.ID]; !exists {
				removed = append(removed, environment.ID)
			}
		}
	}
	sort.Strings(removed)
	return removed
}

func sortEnvironments(environments []domain.Environment) {
	sort.Slice(environments, func(left, right int) bool {
		if environments[left].Primary != environments[right].Primary {
			return environments[left].Primary
		}
		if environments[left].Name != environments[right].Name {
			return environments[left].Name < environments[right].Name
		}
		return environments[left].ID < environments[right].ID
	})
}
