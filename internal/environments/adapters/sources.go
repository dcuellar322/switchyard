package adapters

import (
	"context"

	catalogApplication "switchyard.dev/switchyard/internal/catalog/application"
	catalogDomain "switchyard.dev/switchyard/internal/catalog/domain"
	"switchyard.dev/switchyard/internal/environments/application"
	sourcecontrolApplication "switchyard.dev/switchyard/internal/sourcecontrol/application"
)

// CatalogSource exposes only trusted project identity and locations.
type CatalogSource struct{ catalog *catalogApplication.Service }

// NewCatalogSource creates the environment-facing catalog adapter.
func NewCatalogSource(catalog *catalogApplication.Service) *CatalogSource {
	return &CatalogSource{catalog: catalog}
}

// Project returns a trusted descriptor without exposing catalog persistence.
func (s *CatalogSource) Project(ctx context.Context, projectID string) (application.ProjectDescriptor, error) {
	project, err := s.catalog.GetProject(ctx, projectID)
	if err != nil {
		return application.ProjectDescriptor{}, err
	}
	if project.TrustState != catalogDomain.TrustTrusted {
		return application.ProjectDescriptor{}, application.ErrProjectUntrusted
	}
	return application.ProjectDescriptor{ID: project.ID, Slug: project.Slug, PrimaryLocation: project.PrimaryLocation}, nil
}

// SourceControlSource maps the source-control context's bounded worktree DTOs.
type SourceControlSource struct {
	sourcecontrol *sourcecontrolApplication.Service
}

// NewSourceControlSource creates the environment-facing source-control adapter.
func NewSourceControlSource(sourcecontrol *sourcecontrolApplication.Service) *SourceControlSource {
	return &SourceControlSource{sourcecontrol: sourcecontrol}
}

// ListWorktrees explicitly observes Git administrative metadata.
func (s *SourceControlSource) ListWorktrees(ctx context.Context, projectID string) ([]application.WorktreeObservation, error) {
	worktrees, err := s.sourcecontrol.ListWorktrees(ctx, projectID)
	if err != nil {
		return nil, err
	}
	result := make([]application.WorktreeObservation, 0, len(worktrees))
	for _, worktree := range worktrees {
		result = append(result, application.WorktreeObservation{
			Path: worktree.Path, Head: worktree.Head, Branch: worktree.Branch,
			Detached: worktree.Detached, Bare: worktree.Bare, Locked: worktree.Locked,
		})
	}
	return result, nil
}
