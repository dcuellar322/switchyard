// Package httpclient is the typed local API adapter used by CLI clients.
package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"switchyard.dev/switchyard/internal/platform/localipc"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

// Client wraps the generated transport without leaking response mechanics.
type Client struct {
	generated *generated.ClientWithResponses
}

// APIError preserves a stable problem code and HTTP status for CLI exit mapping.
type APIError struct {
	Operation string
	Status    int
	Code      string
	Detail    string
}

func (e *APIError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s", e.Operation, e.Detail)
	}
	return fmt.Sprintf("%s: unexpected HTTP %d", e.Operation, e.Status)
}

// New creates a typed client for a daemon address.
func New(address string) (*Client, error) {
	baseURL := address
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}
	client, err := generated.NewClientWithResponses(strings.TrimRight(baseURL, "/") + "/api/v1")
	if err != nil {
		return nil, fmt.Errorf("create local API client: %w", err)
	}
	return &Client{generated: client}, nil
}

// NewIPC creates a typed client over privileged local IPC.
func NewIPC(address string) (*Client, error) {
	client, err := generated.NewClientWithResponses(
		"http://switchyard.local/api/v1",
		generated.WithHTTPClient(localipc.HTTPClient(address)),
	)
	if err != nil {
		return nil, fmt.Errorf("create local IPC client: %w", err)
	}
	return &Client{generated: client}, nil
}

// System returns the generated system contract.
func (c *Client) System(ctx context.Context) (generated.SystemInfo, error) {
	response, err := c.generated.GetSystemWithResponse(ctx)
	if err != nil {
		return generated.SystemInfo{}, fmt.Errorf("request system status: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.SystemInfo{}, fmt.Errorf("request system status: unexpected HTTP %d", response.StatusCode())
	}
	return *response.JSON200, nil
}

// BrowserBootstrap requests a one-time browser credential over local IPC.
func (c *Client) BrowserBootstrap(ctx context.Context) (generated.BrowserBootstrap, error) {
	response, err := c.generated.CreateBrowserBootstrapTokenWithResponse(ctx)
	if err != nil {
		return generated.BrowserBootstrap{}, fmt.Errorf("request browser bootstrap: %w", err)
	}
	if response.StatusCode() != http.StatusCreated || response.JSON201 == nil {
		return generated.BrowserBootstrap{}, fmt.Errorf("request browser bootstrap: unexpected HTTP %d", response.StatusCode())
	}
	return *response.JSON201, nil
}

// CreateManifestProposal scans a repository through the privileged local API.
func (c *Client) CreateManifestProposal(ctx context.Context, path, idempotencyKey string) (generated.ManifestProposal, error) {
	response, err := c.generated.CreateManifestProposalWithResponse(ctx,
		&generated.CreateManifestProposalParams{IdempotencyKey: idempotencyKey},
		generated.CreateManifestProposalRequest{Path: path},
	)
	if err != nil {
		return generated.ManifestProposal{}, fmt.Errorf("create manifest proposal: %w", err)
	}
	if response.StatusCode() != http.StatusCreated || response.JSON201 == nil {
		return generated.ManifestProposal{}, unexpected("create manifest proposal", response.StatusCode())
	}
	return *response.JSON201, nil
}

// ValidateManifestProposal reruns proposal validation through the local API.
func (c *Client) ValidateManifestProposal(ctx context.Context, proposalID, idempotencyKey string) (generated.ManifestProposal, error) {
	response, err := c.generated.ValidateManifestProposalWithResponse(ctx, proposalID, &generated.ValidateManifestProposalParams{IdempotencyKey: idempotencyKey})
	if err != nil {
		return generated.ManifestProposal{}, fmt.Errorf("validate manifest proposal: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.ManifestProposal{}, unexpected("validate manifest proposal", response.StatusCode())
	}
	return *response.JSON200, nil
}

// AcceptManifestProposal records a trust decision through the local API.
func (c *Client) AcceptManifestProposal(ctx context.Context, proposalID, idempotencyKey string) (generated.AcceptedManifestProposal, error) {
	response, err := c.generated.AcceptManifestProposalWithResponse(ctx, proposalID, &generated.AcceptManifestProposalParams{IdempotencyKey: idempotencyKey})
	if err != nil {
		return generated.AcceptedManifestProposal{}, fmt.Errorf("accept manifest proposal: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.AcceptedManifestProposal{}, unexpected("accept manifest proposal", response.StatusCode())
	}
	return *response.JSON200, nil
}

// Projects lists registered projects through the local API.
func (c *Client) Projects(ctx context.Context) ([]generated.Project, error) {
	response, err := c.generated.ListProjectsWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return nil, unexpected("list projects", response.StatusCode())
	}
	return *response.JSON200, nil
}

// Project reads one registered project by opaque ID.
func (c *Client) Project(ctx context.Context, projectID string) (generated.Project, error) {
	response, err := c.generated.GetProjectWithResponse(ctx, projectID)
	if err != nil {
		return generated.Project{}, fmt.Errorf("get project: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.Project{}, apiError("get project", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// TrustProject accepts the latest valid proposal for one project.
func (c *Client) TrustProject(ctx context.Context, projectID, idempotencyKey string) (generated.AcceptedManifestProposal, error) {
	response, err := c.generated.TrustProjectWithResponse(ctx, projectID, &generated.TrustProjectParams{IdempotencyKey: idempotencyKey})
	if err != nil {
		return generated.AcceptedManifestProposal{}, fmt.Errorf("trust project: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.AcceptedManifestProposal{}, apiError("trust project", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// RemoveProject removes catalog data without touching the repository.
func (c *Client) RemoveProject(ctx context.Context, projectID, idempotencyKey string) error {
	response, err := c.generated.RemoveProjectWithResponse(ctx, projectID, &generated.RemoveProjectParams{IdempotencyKey: idempotencyKey})
	if err != nil {
		return fmt.Errorf("remove project: %w", err)
	}
	if response.StatusCode() != http.StatusNoContent {
		return apiError("remove project", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return nil
}

// Operations lists recent durable operations with an optional project filter.
func (c *Client) Operations(ctx context.Context, projectID string, limit int64) ([]generated.Operation, error) {
	params := &generated.ListOperationsParams{Limit: &limit}
	if projectID != "" {
		params.ProjectId = &projectID
	}
	response, err := c.generated.ListOperationsWithResponse(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list operations: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return nil, apiError("list operations", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// Operation reads one durable operation.
func (c *Client) Operation(ctx context.Context, operationID string) (generated.Operation, error) {
	response, err := c.generated.GetOperationWithResponse(ctx, operationID)
	if err != nil {
		return generated.Operation{}, fmt.Errorf("get operation: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.Operation{}, apiError("get operation", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// CancelOperation requests durable idempotent cancellation.
func (c *Client) CancelOperation(ctx context.Context, operationID, idempotencyKey string) (generated.Operation, error) {
	response, err := c.generated.CancelOperationWithResponse(ctx, operationID, &generated.CancelOperationParams{IdempotencyKey: idempotencyKey})
	if err != nil {
		return generated.Operation{}, fmt.Errorf("cancel operation: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.Operation{}, apiError("cancel operation", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// ExplainManifest returns effective fields and provenance.
func (c *Client) ExplainManifest(ctx context.Context, projectID string) (generated.EffectiveManifest, error) {
	response, err := c.generated.ExplainProjectManifestWithResponse(ctx, projectID)
	if err != nil {
		return generated.EffectiveManifest{}, fmt.Errorf("explain project manifest: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.EffectiveManifest{}, unexpected("explain project manifest", response.StatusCode())
	}
	return *response.JSON200, nil
}

// DiffManifest compares the accepted and effective manifests.
func (c *Client) DiffManifest(ctx context.Context, projectID string) (generated.ManifestDiff, error) {
	response, err := c.generated.DiffProjectManifestWithResponse(ctx, projectID)
	if err != nil {
		return generated.ManifestDiff{}, fmt.Errorf("diff project manifest: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.ManifestDiff{}, unexpected("diff project manifest", response.StatusCode())
	}
	return *response.JSON200, nil
}

// ValidateProjectManifest validates the fully resolved project manifest.
func (c *Client) ValidateProjectManifest(ctx context.Context, projectID string) (generated.ManifestValidation, error) {
	response, err := c.generated.ValidateProjectManifestWithResponse(ctx, projectID)
	if err != nil {
		return generated.ManifestValidation{}, fmt.Errorf("validate project manifest: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.ManifestValidation{}, unexpected("validate project manifest", response.StatusCode())
	}
	return *response.JSON200, nil
}

func unexpected(operation string, status int) error {
	return fmt.Errorf("%s: unexpected HTTP %d", operation, status)
}

func apiError(operation string, status int, problem *generated.Problem) error {
	result := &APIError{Operation: operation, Status: status}
	if problem != nil {
		result.Code = problem.Code
		if problem.Detail != nil {
			result.Detail = *problem.Detail
		}
	}
	return result
}
