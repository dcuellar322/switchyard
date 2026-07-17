// Package application coordinates project onboarding, trust, and effective manifests.
package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
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
	// ErrHumanApprovalRequired prevents an AI or MCP agent from self-approving an assisted proposal.
	ErrHumanApprovalRequired = errors.New("assisted manifest proposals require human approval")
)

// Repository persists projects, proposals, evidence, and accepted snapshots.
type Repository interface {
	CreateProposal(context.Context, catalog.Project, discoveryDomain.Proposal, MutationActor) error
	ReplacePendingProposal(context.Context, string, catalog.Project, discoveryDomain.Proposal, MutationActor) error
	CreateRevision(context.Context, string, discoveryDomain.Proposal, MutationActor) error
	FindProposalByLocation(context.Context, string) (catalog.Project, discoveryDomain.Proposal, error)
	GetProposal(context.Context, string) (discoveryDomain.Proposal, error)
	LatestProposal(context.Context, string) (discoveryDomain.Proposal, error)
	AcceptProposal(context.Context, string, time.Time, MutationActor) (catalog.Project, discoveryDomain.Proposal, error)
	GetProject(context.Context, string) (catalog.Project, error)
	ListProjects(context.Context) ([]catalog.Project, error)
	AcceptedManifest(context.Context, string) (manifestDomain.Manifest, error)
	RemoveProject(context.Context, string, time.Time, MutationActor) error
}

// ProjectRootPolicy authorizes a canonical repository location without
// coupling catalog behavior to settings persistence.
type ProjectRootPolicy interface {
	AuthorizeProjectRoot(context.Context, string, bool) error
}

// MutationActor is the non-secret principal recorded with catalog trust changes.
type MutationActor struct {
	Type string
	ID   string
}

var systemActor = MutationActor{Type: "system", ID: "catalog-service"}

// Service is the project onboarding use-case boundary.
type Service struct {
	repository Repository
	scanners   []discovery.Scanner
	rootPolicy ProjectRootPolicy
	now        func() time.Time
}

// NewService creates project onboarding use cases with explicit scanner adapters.
func NewService(repository Repository, scanners []discovery.Scanner, policies ...ProjectRootPolicy) *Service {
	service := &Service{repository: repository, scanners: slices.Clone(scanners), now: time.Now}
	if len(policies) > 0 {
		service.rootPolicy = policies[0]
	}
	return service
}

// Scan creates a pending project and reviewable proposal without executing repository code.
func (s *Service) Scan(ctx context.Context, path string) (catalog.Project, discoveryDomain.Proposal, error) {
	return s.ScanAs(ctx, path, systemActor)
}

// ScanAs creates a proposal and records the initiating principal.
func (s *Service) ScanAs(ctx context.Context, path string, actor MutationActor) (catalog.Project, discoveryDomain.Proposal, error) {
	return s.ScanWithRootOverrideAs(ctx, path, false, actor)
}

// ScanWithRootOverrideAs requires an explicit request signal before discovery
// may inspect a repository outside the configured project roots.
func (s *Service) ScanWithRootOverrideAs(ctx context.Context, path string, allowOutsideRoots bool, actor MutationActor) (catalog.Project, discoveryDomain.Proposal, error) {
	root, err := discovery.SelectRoot(path)
	if err != nil {
		return catalog.Project{}, discoveryDomain.Proposal{}, err
	}
	if s.rootPolicy != nil {
		if err := s.rootPolicy.AuthorizeProjectRoot(ctx, root.Path, allowOutsideRoots); err != nil {
			return catalog.Project{}, discoveryDomain.Proposal{}, err
		}
	}
	if project, proposal, findErr := s.repository.FindProposalByLocation(ctx, root.Path); findErr == nil {
		if project.TrustState != catalog.TrustPending || proposal.Status != discoveryDomain.StatusProposed {
			return project, proposal, nil
		}
		refreshed, scanErr := s.buildProposal(ctx, root, project.ID)
		if scanErr != nil {
			return catalog.Project{}, discoveryDomain.Proposal{}, scanErr
		}
		project.Slug = refreshed.Candidate.Metadata.ID
		project.DisplayName = refreshed.Candidate.Metadata.Name
		project.Description = refreshed.Candidate.Metadata.Description
		project.Tags = slices.Clone(refreshed.Candidate.Metadata.Tags)
		project.UpdatedAt = refreshed.CreatedAt
		if replaceErr := s.repository.ReplacePendingProposal(ctx, proposal.ID, project, refreshed, normalizedActor(actor)); replaceErr != nil {
			return catalog.Project{}, discoveryDomain.Proposal{}, replaceErr
		}
		return project, refreshed, nil
	} else if !errors.Is(findErr, ErrNotFound) {
		return catalog.Project{}, discoveryDomain.Proposal{}, findErr
	}
	projectID, err := identifier.New("project")
	if err != nil {
		return catalog.Project{}, discoveryDomain.Proposal{}, err
	}
	proposal, err := s.buildProposal(ctx, root, projectID)
	if err != nil {
		return catalog.Project{}, discoveryDomain.Proposal{}, err
	}
	project := catalog.Project{
		ID: projectID, Slug: proposal.Candidate.Metadata.ID, DisplayName: proposal.Candidate.Metadata.Name,
		Description: proposal.Candidate.Metadata.Description, TrustState: catalog.TrustPending,
		PrimaryLocation: root.Path, Tags: proposal.Candidate.Metadata.Tags,
		CreatedAt: proposal.CreatedAt, UpdatedAt: proposal.CreatedAt,
	}
	if err := s.repository.CreateProposal(ctx, project, proposal, normalizedActor(actor)); err != nil {
		return catalog.Project{}, discoveryDomain.Proposal{}, err
	}
	return project, proposal, nil
}

func (s *Service) buildProposal(ctx context.Context, root discovery.Root, projectID string) (discoveryDomain.Proposal, error) {
	proposalID, err := identifier.New("proposal")
	if err != nil {
		return discoveryDomain.Proposal{}, err
	}
	evidence, err := discovery.ScanAll(ctx, root, s.scanners)
	if err != nil {
		return discoveryDomain.Proposal{}, err
	}
	proposal := discovery.BuildProposal(root, projectID, proposalID, evidence)
	proposal.Validation = discoveryDomain.Validation(manifest.Validate(root.Path, proposal.Candidate))
	proposal.CreatedAt = s.now().UTC()
	return proposal, nil
}

// GetProposal returns the persisted candidate and all source evidence.
func (s *Service) GetProposal(ctx context.Context, id string) (discoveryDomain.Proposal, error) {
	return s.repository.GetProposal(ctx, id)
}

// CreateRevisionAs stores an immutable assisted candidate without trusting it.
func (s *Service) CreateRevisionAs(
	ctx context.Context,
	sourceID string,
	candidate manifestDomain.Manifest,
	confidence map[string]float64,
	unresolved []string,
	actorType, actorID string,
) (discoveryDomain.Proposal, error) {
	source, err := s.repository.GetProposal(ctx, sourceID)
	if err != nil {
		return discoveryDomain.Proposal{}, err
	}
	if source.Status != discoveryDomain.StatusProposed {
		return discoveryDomain.Proposal{}, ErrAlreadyReviewed
	}
	project, err := s.repository.GetProject(ctx, source.ProjectID)
	if err != nil {
		return discoveryDomain.Proposal{}, err
	}
	proposalID, err := identifier.New("proposal")
	if err != nil {
		return discoveryDomain.Proposal{}, err
	}
	evidence := make([]discoveryDomain.Evidence, 0, len(source.Evidence))
	for _, item := range source.Evidence {
		item.ID, err = identifier.New("evidence")
		if err != nil {
			return discoveryDomain.Proposal{}, err
		}
		evidence = append(evidence, item)
	}
	createdAt := s.now().UTC()
	validation := manifest.Validate(project.PrimaryLocation, candidate)
	proposal := discoveryDomain.Proposal{
		ID: proposalID, ProjectID: source.ProjectID, ScannerVersion: source.ScannerVersion + "+ai/" + actorID,
		SchemaVersion: manifestDomain.SchemaVersion, Candidate: candidate, Evidence: evidence,
		ConfidenceByField: confidence, Unresolved: append([]string(nil), unresolved...),
		Validation: discoveryDomain.Validation(validation), Status: discoveryDomain.StatusProposed, CreatedAt: createdAt,
	}
	actor := normalizedActor(MutationActor{Type: actorType, ID: actorID})
	if err := s.repository.CreateRevision(ctx, sourceID, proposal, actor); err != nil {
		return discoveryDomain.Proposal{}, err
	}
	return proposal, nil
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
	return s.AcceptAs(ctx, id, systemActor)
}

// AcceptAs records an explicit trust decision and its principal.
func (s *Service) AcceptAs(ctx context.Context, id string, actor MutationActor) (catalog.Project, discoveryDomain.Proposal, error) {
	proposal, err := s.Validate(ctx, id)
	if err != nil {
		return catalog.Project{}, discoveryDomain.Proposal{}, err
	}
	if proposal.Status != discoveryDomain.StatusProposed {
		return catalog.Project{}, discoveryDomain.Proposal{}, ErrAlreadyReviewed
	}
	if strings.Contains(proposal.ScannerVersion, "+ai/") && actor.Type == "agent" {
		return catalog.Project{}, discoveryDomain.Proposal{}, ErrHumanApprovalRequired
	}
	if !proposal.Validation.Valid || len(proposal.Unresolved) > 0 {
		return catalog.Project{}, discoveryDomain.Proposal{}, invalidProposalError(proposal)
	}
	return s.repository.AcceptProposal(ctx, id, s.now().UTC(), normalizedActor(actor))
}

func invalidProposalError(proposal discoveryDomain.Proposal) error {
	reasons := make([]string, 0, 2)
	if len(proposal.Validation.Errors) > 0 {
		reasons = append(reasons, "validation errors: "+strings.Join(proposal.Validation.Errors, "; "))
	}
	if len(proposal.Unresolved) > 0 {
		reasons = append(reasons, "unresolved fields: "+strings.Join(proposal.Unresolved, ", "))
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "validation failed without details")
	}
	return fmt.Errorf("%w: %s", ErrInvalidProposal, strings.Join(reasons, "; "))
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
	return s.TrustProjectAs(ctx, projectID, systemActor)
}

// TrustProjectAs accepts the latest valid proposal for a principal.
func (s *Service) TrustProjectAs(ctx context.Context, projectID string, actor MutationActor) (catalog.Project, discoveryDomain.Proposal, error) {
	proposal, err := s.repository.LatestProposal(ctx, projectID)
	if err != nil {
		return catalog.Project{}, discoveryDomain.Proposal{}, err
	}
	return s.AcceptAs(ctx, proposal.ID, actor)
}

// RemoveProject removes catalog state without touching repository files.
func (s *Service) RemoveProject(ctx context.Context, projectID string) error {
	return s.RemoveProjectAs(ctx, projectID, systemActor)
}

// RemoveProjectAs removes catalog state and records the initiating principal.
func (s *Service) RemoveProjectAs(ctx context.Context, projectID string, actor MutationActor) error {
	return s.repository.RemoveProject(ctx, projectID, s.now().UTC(), normalizedActor(actor))
}

func normalizedActor(actor MutationActor) MutationActor {
	if actor.Type == "" || actor.ID == "" {
		return systemActor
	}
	return actor
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
