package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	manifestDomain "switchyard.dev/switchyard/internal/manifest/domain"
)

// EnhancementService orchestrates evidence consent, provider execution, merge, and durable review.
type EnhancementService struct {
	catalog   ProposalCatalog
	runs      RunRepository
	reader    EvidenceReader
	redactor  TextRedactor
	validator CandidateValidator
	providers *Registry
	now       func() time.Time
}

// NewEnhancementService constructs assisted onboarding with explicit ports.
func NewEnhancementService(catalog ProposalCatalog, runs RunRepository, reader EvidenceReader, redactor TextRedactor, validator CandidateValidator, providers *Registry) (*EnhancementService, error) {
	if catalog == nil || runs == nil || reader == nil || redactor == nil || validator == nil || providers == nil {
		return nil, errors.New("assisted onboarding dependencies are required")
	}
	return &EnhancementService{catalog: catalog, runs: runs, reader: reader, redactor: redactor, validator: validator, providers: providers, now: time.Now}, nil
}

// Providers reports current capability without invoking a model.
func (s *EnhancementService) Providers(ctx context.Context) []ProviderDescriptor {
	return s.providers.List(ctx)
}

// Preview builds the exact sanitized payload that would cross the provider boundary.
func (s *EnhancementService) Preview(ctx context.Context, proposalID string, limits Limits) (BundlePreview, error) {
	normalized, err := limits.Normalize()
	if err != nil {
		return BundlePreview{}, err
	}
	return buildBundle(ctx, s.catalog, s.reader, s.redactor, proposalID, normalized)
}

// GetRun returns one durable assisted-onboarding receipt.
func (s *EnhancementService) GetRun(ctx context.Context, operationID string) (Run, error) {
	return s.runs.Get(ctx, operationID)
}

// Execute performs one cancellable provider operation and creates an untrusted proposal revision.
func (s *EnhancementService) Execute(ctx context.Context, operationID, proposalID, providerID string, limits Limits) (err error) {
	if existing, getErr := s.runs.Get(ctx, operationID); getErr == nil && existing.State == RunSucceeded {
		return nil
	}
	normalized, err := limits.Normalize()
	if err != nil {
		return err
	}
	provider, descriptor, err := s.providers.provider(ctx, providerID)
	if err != nil {
		return err
	}
	preview, err := buildBundle(ctx, s.catalog, s.reader, s.redactor, proposalID, normalized)
	if err != nil {
		return err
	}
	baseline, err := s.catalog.GetProposal(ctx, proposalID)
	if err != nil {
		return err
	}
	project, err := s.catalog.GetProject(ctx, baseline.ProjectID)
	if err != nil {
		return err
	}
	run := Run{
		OperationID: operationID, ProjectID: baseline.ProjectID, SourceProposalID: proposalID,
		Provider: providerID, Model: descriptor.Model, State: RunRunning, Bundle: preview.Encoded,
		BundleSHA256: preview.SHA256, Limits: normalized, Fields: []FieldReview{}, Conflicts: []Conflict{},
		Warnings: []string{}, DryRun: DryRun{Errors: []string{}, Warnings: []string{}}, StartedAt: s.now().UTC(),
	}
	if err := s.runs.Start(ctx, run); err != nil {
		return err
	}
	defer func() {
		if err == nil {
			return
		}
		finished := s.now().UTC()
		run.FinishedAt = &finished
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			run.State = RunCancelled
			run.ErrorCode = "PROVIDER_CANCELLED"
		} else {
			run.State = RunFailed
			run.ErrorCode = "PROVIDER_FAILED"
		}
		message, _ := s.redactor.RedactText(err.Error())
		run.ErrorMessage = truncate(message, 2<<10)
		_ = s.runs.Finish(context.WithoutCancel(ctx), run)
	}()

	outputSchema, err := ProposalOutputSchema()
	if err != nil {
		return err
	}
	providerCtx, cancel := context.WithTimeout(ctx, normalized.Timeout)
	defer cancel()
	providerResult, err := provider.ProposeManifest(providerCtx, ProviderRequest{Bundle: preview.Encoded, OutputSchema: outputSchema, Limits: normalized})
	if err != nil {
		return fmt.Errorf("provider %s: %w", providerID, err)
	}
	output, err := decodeProviderOutput(providerResult.Output, normalized.OutputBytes)
	if err != nil {
		return err
	}
	merged, err := mergeProposal(project.PrimaryLocation, baseline, output, s.validator)
	if err != nil {
		return err
	}
	unresolved := unresolvedCandidate(merged.Candidate)
	resultProposal, err := s.catalog.CreateRevisionAs(ctx, proposalID, merged.Candidate, merged.Confidence, unresolved, "ai-provider", providerID)
	if err != nil {
		return err
	}
	finished := s.now().UTC()
	run.ResultProposalID = resultProposal.ID
	run.Model = firstNonEmpty(providerResult.Model, descriptor.Model)
	run.State = RunSucceeded
	run.Fields = merged.Fields
	run.Conflicts = merged.Conflicts
	run.Warnings = merged.Warnings
	run.DryRun = merged.DryRun
	run.Usage = providerResult.Usage
	run.FinishedAt = &finished
	if err := s.runs.Finish(context.WithoutCancel(ctx), run); err != nil {
		return err
	}
	return nil
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func unresolvedCandidate(candidate manifestDomain.Manifest) []string {
	result := []string{}
	if candidate.Runtime.Driver == "" {
		result = append(result, "/runtime/driver")
	}
	if len(candidate.Services) == 0 {
		result = append(result, "/services")
	}
	return result
}
