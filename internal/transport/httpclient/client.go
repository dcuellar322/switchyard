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

const (
	actorTypeHeader = "X-Switchyard-Actor-Type"
	actorIDHeader   = "X-Switchyard-Actor-ID"
)

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
	return newIPC(address, "", "")
}

// NewIPCForAgent creates a privileged local client whose mutations retain agent identity in durable audits.
func NewIPCForAgent(address, actorID string) (*Client, error) {
	if actorID == "" {
		return nil, fmt.Errorf("create agent IPC client: actor identity is required")
	}
	return newIPC(address, "agent", actorID)
}

func newIPC(address, actorType, actorID string) (*Client, error) {
	options := []generated.ClientOption{generated.WithHTTPClient(localipc.HTTPClient(address))}
	if actorType != "" {
		options = append(options, generated.WithRequestEditorFn(func(_ context.Context, request *http.Request) error {
			request.Header.Set(actorTypeHeader, actorType)
			request.Header.Set(actorIDHeader, actorID)
			return nil
		}))
	}
	client, err := generated.NewClientWithResponses(
		"http://switchyard.local/api/v1",
		options...,
	)
	if err != nil {
		return nil, fmt.Errorf("create local IPC client: %w", err)
	}
	return &Client{generated: client}, nil
}

// Health returns the latest persisted and currently evaluated project health.
func (c *Client) Health(ctx context.Context, projectID string) (generated.ProjectHealth, error) {
	response, err := c.generated.GetProjectHealthWithResponse(ctx, projectID)
	if err != nil {
		return generated.ProjectHealth{}, fmt.Errorf("read project health: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.ProjectHealth{}, apiError("read project health", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
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

// Host returns the bounded aggregate host observation used by native adapters.
func (c *Client) Host(ctx context.Context) (generated.HostObservation, error) {
	response, err := c.generated.GetHostWithResponse(ctx)
	if err != nil {
		return generated.HostObservation{}, fmt.Errorf("request host observation: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.HostObservation{}, apiError("request host observation", response.StatusCode(), response.ApplicationproblemJSONDefault)
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

// ManifestProposal reads one deterministic proposal and its owning project ID.
func (c *Client) ManifestProposal(ctx context.Context, proposalID string) (generated.ManifestProposal, error) {
	response, err := c.generated.GetManifestProposalWithResponse(ctx, proposalID)
	if err != nil {
		return generated.ManifestProposal{}, fmt.Errorf("read manifest proposal: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.ManifestProposal{}, apiError("read manifest proposal", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
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

// Runtime observes current project services through the configured driver.
func (c *Client) Runtime(ctx context.Context, projectID string) (generated.RuntimeObservation, error) {
	response, err := c.generated.GetProjectRuntimeWithResponse(ctx, projectID)
	if err != nil {
		return generated.RuntimeObservation{}, fmt.Errorf("observe project runtime: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.RuntimeObservation{}, apiError("observe project runtime", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// PlanRuntime previews one lifecycle action without executing it.
func (c *Client) PlanRuntime(ctx context.Context, projectID string, action generated.RuntimeAction, removeVolumes bool) (generated.RuntimePlan, error) {
	return c.PlanRuntimeServices(ctx, projectID, action, removeVolumes, nil)
}

// PlanRuntimeServices previews one lifecycle action for selected declared services.
func (c *Client) PlanRuntimeServices(ctx context.Context, projectID string, action generated.RuntimeAction, removeVolumes bool, services []string) (generated.RuntimePlan, error) {
	request := generated.RuntimeActionRequest{Action: action, RemoveVolumes: &removeVolumes}
	if len(services) > 0 {
		request.Services = &services
	}
	response, err := c.generated.PlanProjectRuntimeWithResponse(ctx, projectID, generated.RuntimeActionRequest{
		Action: request.Action, RemoveVolumes: request.RemoveVolumes, Services: request.Services,
	})
	if err != nil {
		return generated.RuntimePlan{}, fmt.Errorf("plan project runtime: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.RuntimePlan{}, apiError("plan project runtime", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// CreateRuntimeOperation queues a durable lifecycle mutation.
func (c *Client) CreateRuntimeOperation(
	ctx context.Context,
	projectID string,
	action generated.RuntimeAction,
	removeVolumes bool,
	idempotencyKey string,
) (generated.Operation, error) {
	return c.CreateRuntimeOperationForServices(ctx, projectID, action, removeVolumes, nil, idempotencyKey)
}

// CreateRuntimeOperationForServices queues a lifecycle mutation for selected declared services.
func (c *Client) CreateRuntimeOperationForServices(
	ctx context.Context,
	projectID string,
	action generated.RuntimeAction,
	removeVolumes bool,
	services []string,
	idempotencyKey string,
) (generated.Operation, error) {
	request := generated.RuntimeActionRequest{Action: action, RemoveVolumes: &removeVolumes}
	if len(services) > 0 {
		request.Services = &services
	}
	response, err := c.generated.CreateProjectOperationWithResponse(
		ctx, projectID, &generated.CreateProjectOperationParams{IdempotencyKey: idempotencyKey},
		request,
	)
	if err != nil {
		return generated.Operation{}, fmt.Errorf("create runtime operation: %w", err)
	}
	if response.StatusCode() != http.StatusAccepted || response.JSON202 == nil {
		return generated.Operation{}, apiError("create runtime operation", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON202, nil
}

// RuntimeLogs reads a bounded Docker log snapshot.
func (c *Client) RuntimeLogs(ctx context.Context, projectID, service, since, runID, operationID string, tail int) ([]generated.RuntimeLogEntry, error) {
	params := &generated.GetProjectLogsParams{Tail: &tail}
	if service != "" {
		params.Service = &service
	}
	if since != "" {
		params.Since = &since
	}
	if runID != "" {
		params.RunId = &runID
	}
	if operationID != "" {
		params.OperationId = &operationID
	}
	response, err := c.generated.GetProjectLogsWithResponse(ctx, projectID, params)
	if err != nil {
		return nil, fmt.Errorf("read project logs: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return nil, apiError("read project logs", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// ExportRuntimeLogs returns a bounded redacted plain-text or NDJSON export.
func (c *Client) ExportRuntimeLogs(ctx context.Context, projectID, service, runID, operationID string, format generated.ExportProjectLogsParamsFormat) ([]byte, error) {
	params := &generated.ExportProjectLogsParams{Format: format}
	if service != "" {
		params.Service = &service
	}
	if runID != "" {
		params.RunId = &runID
	}
	if operationID != "" {
		params.OperationId = &operationID
	}
	response, err := c.generated.ExportProjectLogsWithResponse(ctx, projectID, params)
	if err != nil {
		return nil, fmt.Errorf("export project logs: %w", err)
	}
	if response.StatusCode() != http.StatusOK {
		return nil, apiError("export project logs", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return response.Body, nil
}

// RuntimeMetrics reads current Compose resource samples.
func (c *Client) RuntimeMetrics(ctx context.Context, projectID, service string) ([]generated.RuntimeMetricSample, error) {
	params := &generated.GetProjectMetricsParams{}
	if service != "" {
		params.Service = &service
	}
	response, err := c.generated.GetProjectMetricsWithResponse(ctx, projectID, params)
	if err != nil {
		return nil, fmt.Errorf("read project metrics: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return nil, apiError("read project metrics", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// PortRegistry reconciles declarations, leases, and live listeners.
func (c *Client) PortRegistry(ctx context.Context) (generated.PortRegistry, error) {
	response, err := c.generated.GetPortRegistryWithResponse(ctx)
	if err != nil {
		return generated.PortRegistry{}, fmt.Errorf("read port registry: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.PortRegistry{}, apiError("read port registry", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// SuggestPort returns the first available port in a bounded range.
func (c *Client) SuggestPort(ctx context.Context, start, end int, protocol, projectID string, excluded []int, idempotencyKey string) (generated.PortSuggestion, error) {
	request := generated.PortSuggestionRequest{RangeStart: start, RangeEnd: end, Protocol: generated.PortSuggestionRequestProtocol(protocol)}
	if projectID != "" {
		request.ProjectId = &projectID
	}
	if len(excluded) > 0 {
		request.Excluded = &excluded
	}
	response, err := c.generated.CreatePortSuggestionWithResponse(ctx, &generated.CreatePortSuggestionParams{IdempotencyKey: idempotencyKey}, request)
	if err != nil {
		return generated.PortSuggestion{}, fmt.Errorf("suggest port: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.PortSuggestion{}, apiError("suggest port", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// GitState reads a fresh repository snapshot.
func (c *Client) GitState(ctx context.Context, projectID string) (generated.GitState, error) {
	response, err := c.generated.GetProjectGitWithResponse(ctx, projectID)
	if err != nil {
		return generated.GitState{}, fmt.Errorf("read project Git state: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.GitState{}, apiError("read project Git state", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// ProjectActions lists the trusted quick-action contract.
func (c *Client) ProjectActions(ctx context.Context, projectID string) (generated.ProjectActions, error) {
	response, err := c.generated.ListProjectActionsWithResponse(ctx, projectID)
	if err != nil {
		return generated.ProjectActions{}, fmt.Errorf("list project actions: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.ProjectActions{}, apiError("list project actions", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// CreateActionOperation queues one durable audited action.
func (c *Client) CreateActionOperation(ctx context.Context, projectID, actionID string, confirm, allowOutside bool, idempotencyKey string) (generated.Operation, error) {
	request := generated.ActionExecutionRequest{ConfirmRisk: &confirm, AllowOutsideRoot: &allowOutside}
	response, err := c.generated.CreateActionOperationWithResponse(ctx, projectID, actionID,
		&generated.CreateActionOperationParams{IdempotencyKey: idempotencyKey}, request)
	if err != nil {
		return generated.Operation{}, fmt.Errorf("create action operation: %w", err)
	}
	if response.StatusCode() != http.StatusAccepted || response.JSON202 == nil {
		return generated.Operation{}, apiError("create action operation", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON202, nil
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
