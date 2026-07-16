package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	catalogDomain "switchyard.dev/switchyard/internal/catalog/domain"
	discoveryDomain "switchyard.dev/switchyard/internal/discovery/domain"
	manifestDomain "switchyard.dev/switchyard/internal/manifest/domain"
)

const (
	// BundleVersion identifies the provider evidence contract.
	BundleVersion = "switchyard.dev/ai-evidence/v1alpha1"
	// OutputVersion identifies the schema-constrained provider response.
	OutputVersion        = "switchyard.dev/ai-proposal/v1alpha1"
	defaultEvidenceBytes = 64 << 10
	defaultOutputBytes   = 256 << 10
	defaultTimeout       = 90 * time.Second
)

var (
	// ErrProviderUnavailable identifies a configured provider that cannot currently run.
	ErrProviderUnavailable = errors.New("structured provider unavailable")
	// ErrProviderOutput identifies malformed or unsafe provider output.
	ErrProviderOutput = errors.New("proposal provider output rejected")
	// ErrRunNotFound identifies an unknown assisted-onboarding run.
	ErrRunNotFound = errors.New("assisted onboarding run not found")
	// ErrInvalidLimits identifies unsupported generation budgets.
	ErrInvalidLimits = errors.New("invalid provider generation limits")
)

// Limits are provider-neutral hard ceilings. Provider adapters may enforce stricter limits.
type Limits struct {
	EvidenceBytes   int64         `json:"evidenceBytes"`
	OutputBytes     int64         `json:"outputBytes"`
	Timeout         time.Duration `json:"-"`
	TimeoutSeconds  int           `json:"timeoutSeconds"`
	MaxTurns        int           `json:"maxTurns"`
	MaxOutputTokens int           `json:"maxOutputTokens"`
	MaxBudgetUSD    float64       `json:"maxBudgetUsd"`
}

// Normalize validates limits and fills conservative defaults.
func (l Limits) Normalize() (Limits, error) {
	if l.EvidenceBytes == 0 {
		l.EvidenceBytes = defaultEvidenceBytes
	}
	if l.OutputBytes == 0 {
		l.OutputBytes = defaultOutputBytes
	}
	if l.TimeoutSeconds == 0 && l.Timeout == 0 {
		l.Timeout = defaultTimeout
	} else if l.Timeout == 0 {
		l.Timeout = time.Duration(l.TimeoutSeconds) * time.Second
	}
	l.TimeoutSeconds = int(l.Timeout / time.Second)
	if l.MaxTurns == 0 {
		l.MaxTurns = 1
	}
	if l.MaxOutputTokens == 0 {
		l.MaxOutputTokens = 4_096
	}
	if l.EvidenceBytes < 4<<10 || l.EvidenceBytes > 256<<10 || l.OutputBytes < 4<<10 || l.OutputBytes > 1<<20 ||
		l.Timeout < 5*time.Second || l.Timeout > 10*time.Minute || l.MaxTurns < 1 || l.MaxTurns > 8 ||
		l.MaxOutputTokens < 256 || l.MaxOutputTokens > 32_768 || l.MaxBudgetUSD < 0 || l.MaxBudgetUSD > 25 {
		return Limits{}, ErrInvalidLimits
	}
	return l, nil
}

// ProviderDescriptor documents one proposal adapter and its enforceable budgets.
type ProviderDescriptor struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	Kind                 string   `json:"kind"`
	Model                string   `json:"model,omitempty"`
	Available            bool     `json:"available"`
	Reason               string   `json:"reason,omitempty"`
	SupportedBudgetKinds []string `json:"supportedBudgetKinds"`
}

// EvidenceItem is one sanitized provider-visible observation.
type EvidenceItem struct {
	ID         string                      `json:"id"`
	Kind       string                      `json:"kind"`
	SourcePath string                      `json:"sourcePath"`
	Location   discoveryDomain.SourceRange `json:"location"`
	Confidence float64                     `json:"confidence"`
	Data       json.RawMessage             `json:"data"`
	Warnings   []string                    `json:"warnings"`
	Excerpt    string                      `json:"excerpt,omitempty"`
	Truncated  bool                        `json:"truncated"`
}

// EvidenceBundle is the exact immutable JSON document sent to a provider.
type EvidenceBundle struct {
	Version           string                  `json:"version"`
	ProjectID         string                  `json:"projectId"`
	ProposalID        string                  `json:"proposalId"`
	Candidate         manifestDomain.Manifest `json:"deterministicCandidate"`
	ConfidenceByField map[string]float64      `json:"confidenceByField"`
	Unresolved        []string                `json:"unresolved"`
	Evidence          []EvidenceItem          `json:"evidence"`
	RedactionCount    int                     `json:"redactionCount"`
	Truncated         bool                    `json:"truncated"`
	EncodedBytes      int                     `json:"encodedBytes"`
}

// FieldClaim links one proposed JSON pointer to deterministic evidence.
type FieldClaim struct {
	Path        string   `json:"path" jsonschema:"required,pattern=^/"`
	EvidenceIDs []string `json:"evidenceIds" jsonschema:"required,minItems=1"`
	Rationale   string   `json:"rationale" jsonschema:"required,maxLength=500"`
}

// ProposalOutput is the only accepted provider response shape.
type ProposalOutput struct {
	Version   string                  `json:"version" jsonschema:"required,enum=switchyard.dev/ai-proposal/v1alpha1"`
	Candidate manifestDomain.Manifest `json:"candidate" jsonschema:"required"`
	Claims    []FieldClaim            `json:"claims" jsonschema:"required"`
	Warnings  []string                `json:"warnings" jsonschema:"required"`
}

// ProviderRequest carries sanitized bytes and schema only; it never includes a repository root.
type ProviderRequest struct {
	Bundle       json.RawMessage
	OutputSchema json.RawMessage
	Limits       Limits
}

// ProviderResult is untrusted until application validation succeeds.
type ProviderResult struct {
	Output json.RawMessage
	Model  string
	Usage  Usage
}

// Usage records non-secret provider accounting where available.
type Usage struct {
	InputTokens  int     `json:"inputTokens,omitempty"`
	OutputTokens int     `json:"outputTokens,omitempty"`
	CostUSD      float64 `json:"costUsd,omitempty"`
}

// ProposalProvider is the provider-neutral structured-generation boundary.
type ProposalProvider interface {
	Descriptor(context.Context) ProviderDescriptor
	ProposeManifest(context.Context, ProviderRequest) (ProviderResult, error)
	Diagnose(context.Context, ProviderRequest) (ProviderResult, error)
}

// ProposalCatalog is the explicit application boundary to catalog onboarding state.
type ProposalCatalog interface {
	GetProposal(context.Context, string) (discoveryDomain.Proposal, error)
	GetProject(context.Context, string) (catalogDomain.Project, error)
	CreateRevisionAs(context.Context, string, manifestDomain.Manifest, map[string]float64, []string, string, string) (discoveryDomain.Proposal, error)
}

// EvidenceReader reads bounded repository excerpts without executing repository code.
type EvidenceReader interface {
	ReadExcerpt(context.Context, string, string, discoveryDomain.SourceRange, int64) (string, bool, error)
}

// TextRedactor removes known credential forms before any provider boundary.
type TextRedactor interface {
	RedactText(string) (string, bool)
}

// CandidateValidation is a provider-neutral side-effect-free manifest validation result.
type CandidateValidation struct {
	Valid    bool
	Errors   []string
	Warnings []string
}

// CandidateValidator validates schema, domain, paths, tools, ports, and health without mutation.
type CandidateValidator interface {
	Validate(string, manifestDomain.Manifest) CandidateValidation
}

// RunRepository persists the exact sent bundle and review result.
type RunRepository interface {
	Start(context.Context, Run) error
	Finish(context.Context, Run) error
	Get(context.Context, string) (Run, error)
}

// RunState is the durable lifecycle for an assisted onboarding attempt.
type RunState string

// RunRunning through RunCancelled are the persisted assisted-onboarding states.
const (
	RunRunning   RunState = "running"
	RunSucceeded RunState = "succeeded"
	RunFailed    RunState = "failed"
	RunCancelled RunState = "cancelled"
)

// Conflict records a provider disagreement and the deterministic-safe resolution.
type Conflict struct {
	Path               string          `json:"path"`
	DeterministicValue json.RawMessage `json:"deterministicValue"`
	ProposedValue      json.RawMessage `json:"proposedValue"`
	Resolution         string          `json:"resolution"`
}

// FieldReview is server-computed provenance for one changed or contested field.
type FieldReview struct {
	Path        string   `json:"path"`
	Source      string   `json:"source"`
	Confidence  float64  `json:"confidence"`
	EvidenceIDs []string `json:"evidenceIds"`
	Rationale   string   `json:"rationale"`
	Warnings    []string `json:"warnings"`
}

// DryRun captures side-effect-free acceptance checks.
type DryRun struct {
	Valid          bool     `json:"valid"`
	SchemaValid    bool     `json:"schemaValid"`
	EvidenceBacked bool     `json:"evidenceBacked"`
	RepositorySafe bool     `json:"repositorySafe"`
	Errors         []string `json:"errors"`
	Warnings       []string `json:"warnings"`
}

// Run is an immutable receipt plus terminal review data for one provider call.
type Run struct {
	OperationID      string          `json:"operationId"`
	ProjectID        string          `json:"projectId"`
	SourceProposalID string          `json:"sourceProposalId"`
	ResultProposalID string          `json:"resultProposalId,omitempty"`
	Provider         string          `json:"provider"`
	Model            string          `json:"model,omitempty"`
	State            RunState        `json:"state"`
	Bundle           json.RawMessage `json:"bundle"`
	BundleSHA256     string          `json:"bundleSha256"`
	Limits           Limits          `json:"limits"`
	Fields           []FieldReview   `json:"fields"`
	Conflicts        []Conflict      `json:"conflicts"`
	Warnings         []string        `json:"warnings"`
	DryRun           DryRun          `json:"dryRun"`
	Usage            Usage           `json:"usage"`
	ErrorCode        string          `json:"errorCode,omitempty"`
	ErrorMessage     string          `json:"errorMessage,omitempty"`
	StartedAt        time.Time       `json:"startedAt"`
	FinishedAt       *time.Time      `json:"finishedAt,omitempty"`
}

// Registry exposes stable provider selection without a service locator.
type Registry struct{ providers map[string]ProposalProvider }

// NewRegistry validates and indexes explicitly composed providers.
func NewRegistry(values ...ProposalProvider) (*Registry, error) {
	registry := &Registry{providers: make(map[string]ProposalProvider, len(values))}
	for _, provider := range values {
		if provider == nil {
			continue
		}
		descriptor := provider.Descriptor(context.Background())
		id := strings.TrimSpace(descriptor.ID)
		if id == "" {
			return nil, errors.New("proposal provider identifier is required")
		}
		if _, exists := registry.providers[id]; exists {
			return nil, fmt.Errorf("duplicate proposal provider %q", id)
		}
		registry.providers[id] = provider
	}
	return registry, nil
}

// List returns descriptors in stable provider order.
func (r *Registry) List(ctx context.Context) []ProviderDescriptor {
	result := make([]ProviderDescriptor, 0, len(r.providers))
	for _, provider := range r.providers {
		result = append(result, provider.Descriptor(ctx))
	}
	slices.SortFunc(result, func(left, right ProviderDescriptor) int { return strings.Compare(left.ID, right.ID) })
	return result
}

func (r *Registry) provider(ctx context.Context, id string) (ProposalProvider, ProviderDescriptor, error) {
	provider, ok := r.providers[id]
	if !ok {
		return nil, ProviderDescriptor{}, fmt.Errorf("%w: %s", ErrProviderUnavailable, id)
	}
	descriptor := provider.Descriptor(ctx)
	if !descriptor.Available {
		return nil, descriptor, fmt.Errorf("%w: %s", ErrProviderUnavailable, descriptor.Reason)
	}
	return provider, descriptor, nil
}

// Diagnose invokes one configured provider through the same bounded structured-output contract.
func (r *Registry) Diagnose(ctx context.Context, id string, request ProviderRequest) (ProviderResult, error) {
	provider, _, err := r.provider(ctx, id)
	if err != nil {
		return ProviderResult{}, err
	}
	return provider.Diagnose(ctx, request)
}
