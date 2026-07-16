// Package application coordinates trusted source-control observations.
package application

import (
	"context"
	"errors"

	catalog "switchyard.dev/switchyard/internal/catalog/application"
	catalogDomain "switchyard.dev/switchyard/internal/catalog/domain"
	"switchyard.dev/switchyard/internal/sourcecontrol/domain"
)

// ErrProjectUntrusted prevents repository access before manifest approval.
var ErrProjectUntrusted = errors.New("project must be trusted before repository observation")

// ErrWorktreeObservationUnsupported reports an observer that cannot provide an
// explicit worktree inventory. Production Git observers implement this seam;
// keeping it separate avoids pretending an empty list is a successful probe.
var ErrWorktreeObservationUnsupported = errors.New("source-control observer does not support worktree inventory")

// ProjectSource resolves the trusted repository root.
type ProjectSource interface {
	Root(context.Context, string) (string, error)
}

// Observer reads Git without mutating the repository.
type Observer interface {
	Observe(context.Context, string, string) (domain.State, error)
}

// WorktreeObserver reads only Git's administrative worktree metadata. It must
// never invoke repository-defined commands or hooks.
type WorktreeObserver interface {
	ObserveWorktrees(context.Context, string) ([]domain.Worktree, error)
}

// Service resolves a trusted root before delegating read-only Git observation.
type Service struct {
	projects ProjectSource
	observer Observer
}

// NewService creates the source-control application boundary.
func NewService(projects ProjectSource, observer Observer) *Service {
	return &Service{projects: projects, observer: observer}
}

// Get returns current source-control state for a trusted project.
func (s *Service) Get(ctx context.Context, projectID string) (domain.State, error) {
	root, err := s.projects.Root(ctx, projectID)
	if err != nil {
		return domain.State{}, err
	}
	return s.observer.Observe(ctx, projectID, root)
}

// ListWorktrees returns the trusted repository's current worktree inventory.
// This is an explicit observation operation, not part of deterministic project
// discovery, and executes no repository-defined command.
func (s *Service) ListWorktrees(ctx context.Context, projectID string) ([]domain.Worktree, error) {
	root, err := s.projects.Root(ctx, projectID)
	if err != nil {
		return nil, err
	}
	observer, ok := s.observer.(WorktreeObserver)
	if !ok {
		return nil, ErrWorktreeObservationUnsupported
	}
	return observer.ObserveWorktrees(ctx, root)
}

// CatalogSource prevents pending repository paths from reaching Git.
type CatalogSource struct{ catalog *catalog.Service }

// NewCatalogSource adapts accepted catalog projects to trusted roots.
func NewCatalogSource(service *catalog.Service) *CatalogSource {
	return &CatalogSource{catalog: service}
}

// Root returns a canonical path only for a trusted project.
func (s *CatalogSource) Root(ctx context.Context, projectID string) (string, error) {
	project, err := s.catalog.GetProject(ctx, projectID)
	if err != nil {
		return "", err
	}
	if project.TrustState != catalogDomain.TrustTrusted {
		return "", ErrProjectUntrusted
	}
	return project.PrimaryLocation, nil
}
