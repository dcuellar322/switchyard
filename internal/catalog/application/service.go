// Package application coordinates project onboarding, trust, and effective manifests.
package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	catalog "switchyard.dev/switchyard/internal/catalog/domain"
	discovery "switchyard.dev/switchyard/internal/discovery/application"
	discoveryDomain "switchyard.dev/switchyard/internal/discovery/domain"
	"switchyard.dev/switchyard/internal/foundation/identifier"
	manifest "switchyard.dev/switchyard/internal/manifest/application"
	manifestDomain "switchyard.dev/switchyard/internal/manifest/domain"
)

var (
	// ErrNotFound identifies an unknown project or proposal.
	ErrNotFound = errors.New("catalog entity not found")
	// ErrInvalidProposal prevents trusting invalid or unresolved candidates.
	ErrInvalidProposal = errors.New("manifest proposal is invalid")
	// ErrAlreadyReviewed prevents a second trust transition.
	ErrAlreadyReviewed = errors.New("manifest proposal has already been reviewed")
)

// Repository persists projects, proposals, evidence, and accepted snapshots.
type Repository interface {
	CreateProposal(context.Context, catalog.Project, discoveryDomain.Proposal) error
	FindProposalByLocation(context.Context, string) (catalog.Project, discoveryDomain.Proposal, error)
	GetProposal(context.Context, string) (discoveryDomain.Proposal, error)
	LatestProposal(context.Context, string) (discoveryDomain.Proposal, error)
	AcceptProposal(context.Context, string, time.Time) (catalog.Project, discoveryDomain.Proposal, error)
	GetProject(context.Context, string) (catalog.Project, error)
	ListProjects(context.Context) ([]catalog.Project, error)
	AcceptedManifest(context.Context, string) (manifestDomain.Manifest, error)
	RemoveProject(context.Context, string, time.Time) error
}

// Service is the project onboarding use-case boundary.
type Service struct {
	repository Repository
	scanners   []discovery.Scanner
	now        func() time.Time
}

// NewService creates project onboarding use cases with explicit scanner adapters.
func NewService(repository Repository, scanners []discovery.Scanner) *Service {
	return &Service{repository: repository, scanners: slices.Clone(scanners), now: time.Now}
}

// Scan creates a pending project and reviewable proposal without executing repository code.
func (s *Service) Scan(ctx context.Context, path string) (catalog.Project, discoveryDomain.Proposal, error) {
	root, err := discovery.SelectRoot(path)
	if err != nil {
		return catalog.Project{}, discoveryDomain.Proposal{}, err
	}
	if project, proposal, findErr := s.repository.FindProposalByLocation(ctx, root.Path); findErr == nil {
		return project, proposal, nil
	} else if !errors.Is(findErr, ErrNotFound) {
		return catalog.Project{}, discoveryDomain.Proposal{}, findErr
	}
	projectID, err := identifier.New("project")
	if err != nil {
		return catalog.Project{}, discoveryDomain.Proposal{}, err
	}
	proposalID, err := identifier.New("proposal")
	if err != nil {
		return catalog.Project{}, discoveryDomain.Proposal{}, err
	}
	evidence, err := discovery.ScanAll(ctx, root, s.scanners)
	if err != nil {
		return catalog.Project{}, discoveryDomain.Proposal{}, err
	}
	proposal := discovery.BuildProposal(root, projectID, proposalID, evidence)
	validation := manifest.Validate(root.Path, proposal.Candidate)
	proposal.Validation = discoveryDomain.Validation(validation)
	proposal.CreatedAt = s.now().UTC()
	project := catalog.Project{
		ID: projectID, Slug: proposal.Candidate.Metadata.ID, DisplayName: proposal.Candidate.Metadata.Name,
		Description: proposal.Candidate.Metadata.Description, TrustState: catalog.TrustPending,
		PrimaryLocation: root.Path, Tags: proposal.Candidate.Metadata.Tags,
		CreatedAt: proposal.CreatedAt, UpdatedAt: proposal.CreatedAt,
	}
	if err := s.repository.CreateProposal(ctx, project, proposal); err != nil {
		return catalog.Project{}, discoveryDomain.Proposal{}, err
	}
	return project, proposal, nil
}

// GetProposal returns the persisted candidate and all source evidence.
func (s *Service) GetProposal(ctx context.Context, id string) (discoveryDomain.Proposal, error) {
	return s.repository.GetProposal(ctx, id)
}

// Validate reruns validation against the current selected root.
func (s *Service) Validate(ctx context.Context, id string) (discoveryDomain.Proposal, error) {
	proposal, err := s.repository.GetProposal(ctx, id)
	if err != nil {
		return discoveryDomain.Proposal{}, err
	}
	project, err := s.repository.GetProject(ctx, proposal.ProjectID)
	if err != nil {
		return discoveryDomain.Proposal{}, err
	}
	result := manifest.Validate(project.PrimaryLocation, proposal.Candidate)
	proposal.Validation = discoveryDomain.Validation(result)
	return proposal, nil
}

// Accept records a human trust decision and immutable manifest snapshot.
func (s *Service) Accept(ctx context.Context, id string) (catalog.Project, discoveryDomain.Proposal, error) {
	proposal, err := s.Validate(ctx, id)
	if err != nil {
		return catalog.Project{}, discoveryDomain.Proposal{}, err
	}
	if proposal.Status != discoveryDomain.StatusProposed {
		return catalog.Project{}, discoveryDomain.Proposal{}, ErrAlreadyReviewed
	}
	if !proposal.Validation.Valid || len(proposal.Unresolved) > 0 {
		return catalog.Project{}, discoveryDomain.Proposal{}, fmt.Errorf("%w: %v", ErrInvalidProposal, proposal.Validation.Errors)
	}
	return s.repository.AcceptProposal(ctx, id, s.now().UTC())
}

// ListProjects returns registered projects in stable display order.
func (s *Service) ListProjects(ctx context.Context) ([]catalog.Project, error) {
	return s.repository.ListProjects(ctx)
}

// GetProject returns one registered project.
func (s *Service) GetProject(ctx context.Context, id string) (catalog.Project, error) {
	return s.repository.GetProject(ctx, id)
}

// TrustProject validates and accepts the latest proposal for a project.
func (s *Service) TrustProject(ctx context.Context, projectID string) (catalog.Project, discoveryDomain.Proposal, error) {
	proposal, err := s.repository.LatestProposal(ctx, projectID)
	if err != nil {
		return catalog.Project{}, discoveryDomain.Proposal{}, err
	}
	return s.Accept(ctx, proposal.ID)
}

// RemoveProject removes catalog state without touching repository files.
func (s *Service) RemoveProject(ctx context.Context, projectID string) error {
	return s.repository.RemoveProject(ctx, projectID, s.now().UTC())
}

// EffectiveManifest resolves the accepted snapshot with portable and local files.
func (s *Service) EffectiveManifest(ctx context.Context, projectID string, runtimeOverride []byte) (manifest.EffectiveManifest, error) {
	project, err := s.repository.GetProject(ctx, projectID)
	if err != nil {
		return manifest.EffectiveManifest{}, err
	}
	accepted, err := s.repository.AcceptedManifest(ctx, projectID)
	if err != nil {
		return manifest.EffectiveManifest{}, err
	}
	return manifest.Resolve(project.PrimaryLocation, manifestDomain.Manifest{}, accepted, runtimeOverride)
}

// Diff returns canonical JSON source and effective documents for automation-friendly review.
func (s *Service) Diff(ctx context.Context, projectID string) (map[string]json.RawMessage, error) {
	effective, err := s.EffectiveManifest(ctx, projectID, nil)
	if err != nil {
		return nil, err
	}
	accepted, err := s.repository.AcceptedManifest(ctx, projectID)
	if err != nil {
		return nil, err
	}
	acceptedJSON, _ := json.MarshalIndent(accepted, "", "  ")
	effectiveJSON, _ := json.MarshalIndent(effective.Manifest, "", "  ")
	return map[string]json.RawMessage{"accepted": acceptedJSON, "effective": effectiveJSON}, nil
}

// ValidateProject checks the fully resolved manifest against the trusted root.
func (s *Service) ValidateProject(ctx context.Context, projectID string) (manifest.ValidationResult, error) {
	project, err := s.repository.GetProject(ctx, projectID)
	if err != nil {
		return manifest.ValidationResult{}, err
	}
	effective, err := s.EffectiveManifest(ctx, projectID, nil)
	if err != nil {
		return manifest.ValidationResult{}, err
	}
	return manifest.Validate(project.PrimaryLocation, effective.Manifest), nil
}
