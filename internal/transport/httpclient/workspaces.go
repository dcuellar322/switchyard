package httpclient

import (
	"context"
	"fmt"
	"net/http"

	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

// ProjectEnvironments lists durable worktree registrations for one project.
func (c *Client) ProjectEnvironments(ctx context.Context, projectID string) ([]generated.ProjectEnvironment, error) {
	response, err := c.generated.ListProjectEnvironmentsWithResponse(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project environments: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return nil, apiError("list project environments", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// RegisterProjectEnvironments reconciles Git worktrees and exact port leases.
func (c *Client) RegisterProjectEnvironments(ctx context.Context, projectID, idempotencyKey string) (generated.EnvironmentRegistration, error) {
	response, err := c.generated.RegisterProjectEnvironmentsWithResponse(ctx, projectID, &generated.RegisterProjectEnvironmentsParams{IdempotencyKey: idempotencyKey})
	if err != nil {
		return generated.EnvironmentRegistration{}, fmt.Errorf("register project environments: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.EnvironmentRegistration{}, apiError("register project environments", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// Environment reads one registered worktree environment.
func (c *Client) Environment(ctx context.Context, environmentID string) (generated.ProjectEnvironment, error) {
	response, err := c.generated.GetEnvironmentWithResponse(ctx, environmentID)
	if err != nil {
		return generated.ProjectEnvironment{}, fmt.Errorf("get environment: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.ProjectEnvironment{}, apiError("get environment", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// UpdateEnvironmentHostname changes only the friendly .localhost name.
func (c *Client) UpdateEnvironmentHostname(ctx context.Context, environmentID, hostname, idempotencyKey string) (generated.ProjectEnvironment, error) {
	response, err := c.generated.UpdateEnvironmentWithResponse(ctx, environmentID,
		&generated.UpdateEnvironmentParams{IdempotencyKey: idempotencyKey}, generated.EnvironmentUpdate{Hostname: hostname})
	if err != nil {
		return generated.ProjectEnvironment{}, fmt.Errorf("update environment: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.ProjectEnvironment{}, apiError("update environment", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// LocalRoutes reads the optional .localhost route registry.
func (c *Client) LocalRoutes(ctx context.Context) ([]generated.LocalRoute, error) {
	response, err := c.generated.ListLocalRoutesWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("list local routes: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return nil, apiError("list local routes", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// Workspaces lists durable workspace graphs.
func (c *Client) Workspaces(ctx context.Context) ([]generated.Workspace, error) {
	response, err := c.generated.ListWorkspacesWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return nil, apiError("list workspaces", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// Workspace reads one durable workspace graph.
func (c *Client) Workspace(ctx context.Context, workspaceID string) (generated.Workspace, error) {
	response, err := c.generated.GetWorkspaceWithResponse(ctx, workspaceID)
	if err != nil {
		return generated.Workspace{}, fmt.Errorf("get workspace: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.Workspace{}, apiError("get workspace", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// CreateWorkspace persists one validated graph document.
func (c *Client) CreateWorkspace(ctx context.Context, definition generated.WorkspaceDefinition, idempotencyKey string) (generated.Workspace, error) {
	response, err := c.generated.CreateWorkspaceWithResponse(ctx, &generated.CreateWorkspaceParams{IdempotencyKey: idempotencyKey}, definition)
	if err != nil {
		return generated.Workspace{}, fmt.Errorf("create workspace: %w", err)
	}
	if response.StatusCode() != http.StatusCreated || response.JSON201 == nil {
		return generated.Workspace{}, apiError("create workspace", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON201, nil
}

// UpdateWorkspace replaces one graph with optimistic revision checking.
func (c *Client) UpdateWorkspace(ctx context.Context, workspaceID string, update generated.WorkspaceUpdate, idempotencyKey string) (generated.Workspace, error) {
	response, err := c.generated.UpdateWorkspaceWithResponse(ctx, workspaceID, &generated.UpdateWorkspaceParams{IdempotencyKey: idempotencyKey}, update)
	if err != nil {
		return generated.Workspace{}, fmt.Errorf("update workspace: %w", err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return generated.Workspace{}, apiError("update workspace", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON200, nil
}

// DeleteWorkspace removes coordination metadata without touching runtime data.
func (c *Client) DeleteWorkspace(ctx context.Context, workspaceID, idempotencyKey string) error {
	response, err := c.generated.DeleteWorkspaceWithResponse(ctx, workspaceID, &generated.DeleteWorkspaceParams{IdempotencyKey: idempotencyKey})
	if err != nil {
		return fmt.Errorf("delete workspace: %w", err)
	}
	if response.StatusCode() != http.StatusNoContent {
		return apiError("delete workspace", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return nil
}

// CreateWorkspaceOperation queues an ordered start or dependency-safe stop.
func (c *Client) CreateWorkspaceOperation(ctx context.Context, workspaceID string, request generated.WorkspaceOperationRequest, idempotencyKey string) (generated.Operation, error) {
	response, err := c.generated.CreateWorkspaceOperationWithResponse(ctx, workspaceID,
		&generated.CreateWorkspaceOperationParams{IdempotencyKey: idempotencyKey}, request)
	if err != nil {
		return generated.Operation{}, fmt.Errorf("create workspace operation: %w", err)
	}
	if response.StatusCode() != http.StatusAccepted || response.JSON202 == nil {
		return generated.Operation{}, apiError("create workspace operation", response.StatusCode(), response.ApplicationproblemJSONDefault)
	}
	return *response.JSON202, nil
}
