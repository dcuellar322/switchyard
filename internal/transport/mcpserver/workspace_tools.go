package mcpserver

import (
	"context"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	agents "switchyard.dev/switchyard/internal/agents/application"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

type workspaceBackend interface {
	Workspaces(context.Context) ([]generated.Workspace, error)
	Workspace(context.Context, string) (generated.Workspace, error)
	CreateWorkspaceOperation(context.Context, string, generated.WorkspaceOperationRequest, string) (generated.Operation, error)
}

type environmentBackend interface {
	ProjectEnvironments(context.Context, string) ([]generated.ProjectEnvironment, error)
	Environment(context.Context, string) (generated.ProjectEnvironment, error)
	RegisterProjectEnvironments(context.Context, string, string) (generated.EnvironmentRegistration, error)
	LocalRoutes(context.Context) ([]generated.LocalRoute, error)
}

func (s *Server) workspacesList(ctx context.Context, _ *mcp.CallToolRequest, input projectsInput) (*mcp.CallToolResult, workspacesOutput, error) {
	limit := input.Limit
	if limit == 0 {
		limit = 100
	}
	if limit < 1 || limit > 100 {
		return nil, workspacesOutput{}, errors.New("limit must be between 1 and 100")
	}
	api, ok := s.backend.(workspaceBackend)
	if !ok {
		return nil, workspacesOutput{}, errors.New("workspace API is unavailable")
	}
	items, err := api.Workspaces(ctx)
	if err != nil {
		return nil, workspacesOutput{}, err
	}
	result := make([]generated.Workspace, 0, min(limit, len(items)))
	for _, workspace := range items {
		if s.authorizeWorkspaceRead(ctx, workspace) == nil {
			result = append(result, workspace)
		}
		if len(result) == limit {
			break
		}
	}
	return nil, workspacesOutput{SchemaVersion: schemaVersion, Workspaces: result, Truncated: len(result) < len(items)}, nil
}

func (s *Server) workspaceGet(ctx context.Context, _ *mcp.CallToolRequest, input workspaceInput) (*mcp.CallToolResult, workspaceOutput, error) {
	if err := required(input.WorkspaceID, "workspaceId"); err != nil {
		return nil, workspaceOutput{}, err
	}
	api, ok := s.backend.(workspaceBackend)
	if !ok {
		return nil, workspaceOutput{}, errors.New("workspace API is unavailable")
	}
	workspace, err := api.Workspace(ctx, input.WorkspaceID)
	if err == nil {
		err = s.authorizeWorkspaceRead(ctx, workspace)
	}
	return nil, workspaceOutput{SchemaVersion: schemaVersion, Workspace: workspace}, err
}

func (s *Server) workspaceStart(ctx context.Context, _ *mcp.CallToolRequest, input workspaceMutationInput) (*mcp.CallToolResult, mutationOutput, error) {
	api, workspace, err := s.validateWorkspaceMutation(ctx, input.WorkspaceID, input.RequestID, agents.CapabilityLifecycle)
	if err != nil {
		return nil, mutationOutput{}, err
	}
	request := generated.WorkspaceOperationRequest{Action: generated.WorkspaceStart, RunRecipes: &input.RunRecipes}
	if input.ProfileID != "" {
		request.ProfileId = &input.ProfileID
	}
	if input.Policy != "" {
		if input.Policy != "rollback" && input.Policy != "continue" {
			return nil, mutationOutput{}, errors.New("policy must be rollback or continue")
		}
		policy := generated.WorkspaceFailurePolicy(input.Policy)
		request.Policy = &policy
	}
	operation, err := api.CreateWorkspaceOperation(ctx, workspace.Id, request, input.RequestID)
	return nil, mutationOutput{SchemaVersion: schemaVersion, Operation: operation}, err
}

func (s *Server) workspaceStop(ctx context.Context, _ *mcp.CallToolRequest, input workspaceStopInput) (*mcp.CallToolResult, mutationOutput, error) {
	capability := agents.CapabilityLifecycle
	if input.RemoveData {
		capability = agents.CapabilityDestructive
		if !input.ConfirmRisk {
			return nil, mutationOutput{}, errors.New("confirmRisk is required when removeData is true")
		}
	}
	api, workspace, err := s.validateWorkspaceMutation(ctx, input.WorkspaceID, input.RequestID, capability)
	if err != nil {
		return nil, mutationOutput{}, err
	}
	request := generated.WorkspaceOperationRequest{
		Action: generated.WorkspaceStop, ProfileId: stringValue(input.ProfileID),
		RemoveData: &input.RemoveData, ConfirmDataRemoval: &input.ConfirmRisk,
	}
	operation, err := api.CreateWorkspaceOperation(ctx, workspace.Id, request, input.RequestID)
	return nil, mutationOutput{SchemaVersion: schemaVersion, Operation: operation}, err
}

func (s *Server) environmentsList(ctx context.Context, _ *mcp.CallToolRequest, input projectInput) (*mcp.CallToolResult, environmentsOutput, error) {
	if err := s.validateProjectRead(input.ProjectID); err != nil {
		return nil, environmentsOutput{}, err
	}
	api, ok := s.backend.(environmentBackend)
	if !ok {
		return nil, environmentsOutput{}, errors.New("environment API is unavailable")
	}
	environments, err := api.ProjectEnvironments(ctx, input.ProjectID)
	return nil, environmentsOutput{SchemaVersion: schemaVersion, Environments: environments}, err
}

func (s *Server) routesList(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, routesOutput, error) {
	api, ok := s.backend.(environmentBackend)
	if !ok {
		return nil, routesOutput{}, errors.New("environment API is unavailable")
	}
	routes, err := api.LocalRoutes(ctx)
	if err != nil {
		return nil, routesOutput{}, err
	}
	filtered := routes[:0]
	for _, route := range routes {
		if route.ProjectId != nil && s.scope.AuthorizeRead(*route.ProjectId) == nil {
			filtered = append(filtered, route)
		}
	}
	return nil, routesOutput{SchemaVersion: schemaVersion, Routes: filtered}, nil
}

func (s *Server) environmentsRegister(ctx context.Context, _ *mcp.CallToolRequest, input environmentRegistrationInput) (*mcp.CallToolResult, environmentRegistrationOutput, error) {
	if err := s.validateProjectMutation(agents.CapabilityLifecycle, input.ProjectID, input.RequestID); err != nil {
		return nil, environmentRegistrationOutput{}, err
	}
	api, ok := s.backend.(environmentBackend)
	if !ok {
		return nil, environmentRegistrationOutput{}, errors.New("environment API is unavailable")
	}
	registration, err := api.RegisterProjectEnvironments(ctx, input.ProjectID, input.RequestID)
	return nil, environmentRegistrationOutput{SchemaVersion: schemaVersion, Registration: registration}, err
}

func (s *Server) validateWorkspaceMutation(ctx context.Context, workspaceID, requestID string, capability agents.Capability) (workspaceBackend, generated.Workspace, error) {
	if err := required(workspaceID, "workspaceId"); err != nil {
		return nil, generated.Workspace{}, err
	}
	if err := requestIDValue(requestID); err != nil {
		return nil, generated.Workspace{}, err
	}
	api, ok := s.backend.(workspaceBackend)
	if !ok {
		return nil, generated.Workspace{}, errors.New("workspace API is unavailable")
	}
	workspace, err := api.Workspace(ctx, workspaceID)
	if err != nil {
		return nil, generated.Workspace{}, err
	}
	for _, member := range workspace.Members {
		projectID, resolveErr := s.workspaceMemberProject(ctx, member.ProjectId)
		if resolveErr != nil {
			return nil, generated.Workspace{}, resolveErr
		}
		if err := s.scope.Authorize(capability, projectID); err != nil {
			return nil, generated.Workspace{}, fmt.Errorf("workspace member %s: %w", member.ProjectId, err)
		}
	}
	return api, workspace, nil
}

func (s *Server) authorizeWorkspaceRead(ctx context.Context, workspace generated.Workspace) error {
	for _, member := range workspace.Members {
		projectID, err := s.workspaceMemberProject(ctx, member.ProjectId)
		if err != nil {
			return err
		}
		if err := s.scope.AuthorizeRead(projectID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) workspaceMemberProject(ctx context.Context, memberID string) (string, error) {
	if len(memberID) < 4 || memberID[:4] != "env-" {
		return memberID, nil
	}
	api, ok := s.backend.(environmentBackend)
	if !ok {
		return "", errors.New("environment API is unavailable")
	}
	environment, err := api.Environment(ctx, memberID)
	if err != nil {
		return "", err
	}
	return environment.ProjectId, nil
}

func stringValue(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func requestIDValue(value string) error { return requestID(value) }
