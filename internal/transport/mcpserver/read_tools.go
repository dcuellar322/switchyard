package mcpserver

import (
	"context"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

func (s *Server) addReadTools() {
	mcp.AddTool(s.mcp, readTool("switchyard_system_info", "Switchyard system info", "Read daemon version, API, schema, and readiness."), s.systemInfo)
	mcp.AddTool(s.mcp, readTool("switchyard_projects_list", "List Switchyard projects", "List a bounded set of registered projects and trust states."), s.projectsList)
	mcp.AddTool(s.mcp, readTool("switchyard_project_get", "Get Switchyard project", "Read one registered project by opaque identifier."), s.projectGet)
	mcp.AddTool(s.mcp, readTool("switchyard_project_status", "Get project status", "Read catalog, runtime, and health status for one project."), s.projectStatus)
	mcp.AddTool(s.mcp, readTool("switchyard_project_services", "List project services", "Read bounded service observations for one project."), s.projectServices)
	mcp.AddTool(s.mcp, readTool("switchyard_project_logs_query", "Query project logs", "Read bounded, redacted recent project logs."), s.projectLogs)
	mcp.AddTool(s.mcp, readTool("switchyard_project_health", "Get project health", "Read structured persisted health diagnostics."), s.projectHealth)
	mcp.AddTool(s.mcp, readTool("switchyard_project_health_wait", "Wait for project health", "Wait up to 30 seconds for structured project health to become healthy."), s.projectHealthWait)
	mcp.AddTool(s.mcp, readTool("switchyard_project_git_status", "Get project Git status", "Read bounded Git branch, changes, remotes, worktrees, and last commit."), s.projectGit)
	mcp.AddTool(s.mcp, readTool("switchyard_ports_list", "List local ports", "Read bounded declarations, leases, listeners, and conflicts."), s.portsList)
	mcp.AddTool(s.mcp, readTool("switchyard_ports_suggest", "Suggest local port", "Find an available port in an explicit bounded range without reserving it."), s.portsSuggest)
	mcp.AddTool(s.mcp, readTool("switchyard_actions_list", "List trusted actions", "List bounded trusted project actions and their risk metadata."), s.actionsList)
	mcp.AddTool(s.mcp, readTool("switchyard_operation_get", "Get operation", "Read one durable operation and its terminal or cancellation state."), s.operationGet)
	mcp.AddTool(s.mcp, readTool("switchyard_operation_wait", "Wait for operation", "Wait up to 30 seconds for an operation and emit MCP progress notifications."), s.operationWait)
	mcp.AddTool(s.mcp, readTool("switchyard_manifest_explain", "Explain effective manifest", "Read the effective trusted manifest and field provenance."), s.manifestExplain)
}

func (s *Server) systemInfo(ctx context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, systemOutput, error) {
	info, err := s.backend.System(ctx)
	return nil, systemOutput{SchemaVersion: schemaVersion, System: info}, err
}

func (s *Server) projectsList(ctx context.Context, _ *mcp.CallToolRequest, input projectsInput) (*mcp.CallToolResult, projectsOutput, error) {
	limit, err := bounded(input.Limit, 50, 100, "limit")
	if err != nil {
		return nil, projectsOutput{}, err
	}
	projects, err := s.backend.Projects(ctx)
	if err != nil {
		return nil, projectsOutput{}, err
	}
	if len(s.scope.ProjectIDs) > 0 {
		projects = slices.DeleteFunc(projects, func(project generated.Project) bool {
			return s.scope.AuthorizeRead(project.Id) != nil
		})
	}
	truncated := len(projects) > limit
	if truncated {
		projects = projects[:limit]
	}
	return nil, projectsOutput{SchemaVersion: schemaVersion, Projects: projects, Truncated: truncated}, nil
}

func (s *Server) projectGet(ctx context.Context, _ *mcp.CallToolRequest, input projectInput) (*mcp.CallToolResult, projectOutput, error) {
	if err := s.validateProjectRead(input.ProjectID); err != nil {
		return nil, projectOutput{}, err
	}
	project, err := s.backend.Project(ctx, input.ProjectID)
	return nil, projectOutput{SchemaVersion: schemaVersion, Project: project}, err
}

func (s *Server) projectStatus(ctx context.Context, _ *mcp.CallToolRequest, input projectInput) (*mcp.CallToolResult, statusOutput, error) {
	if err := s.validateProjectRead(input.ProjectID); err != nil {
		return nil, statusOutput{}, err
	}
	project, err := s.backend.Project(ctx, input.ProjectID)
	if err != nil {
		return nil, statusOutput{}, err
	}
	runtime, err := s.backend.Runtime(ctx, input.ProjectID)
	if err != nil {
		return nil, statusOutput{}, err
	}
	health, err := s.backend.Health(ctx, input.ProjectID)
	return nil, statusOutput{SchemaVersion: schemaVersion, Project: project, Runtime: runtime, Health: health}, err
}

func (s *Server) projectServices(ctx context.Context, _ *mcp.CallToolRequest, input projectInput) (*mcp.CallToolResult, servicesOutput, error) {
	if err := s.validateProjectRead(input.ProjectID); err != nil {
		return nil, servicesOutput{}, err
	}
	observation, err := s.backend.Runtime(ctx, input.ProjectID)
	if err != nil {
		return nil, servicesOutput{}, err
	}
	services, truncated := observation.Services, false
	if len(services) > 100 {
		services, truncated = services[:100], true
	}
	return nil, servicesOutput{SchemaVersion: schemaVersion, ProjectID: input.ProjectID, Services: services, Truncated: truncated}, nil
}

func (s *Server) projectLogs(ctx context.Context, _ *mcp.CallToolRequest, input logsInput) (*mcp.CallToolResult, logsOutput, error) {
	if err := s.validateProjectRead(input.ProjectID); err != nil {
		return nil, logsOutput{}, err
	}
	tail, err := bounded(input.Tail, 100, 500, "tail")
	if err != nil {
		return nil, logsOutput{}, err
	}
	entries, err := s.backend.RuntimeLogs(ctx, input.ProjectID, input.ServiceID, input.Since, input.RunID, input.OperationID, tail)
	if err != nil {
		return nil, logsOutput{}, err
	}
	for index := range entries {
		if entries[index].Attributes == nil {
			entries[index].Attributes = map[string]string{}
		}
	}
	return nil, logsOutput{SchemaVersion: schemaVersion, Entries: entries, Truncated: len(entries) == tail}, nil
}

func (s *Server) projectHealth(ctx context.Context, _ *mcp.CallToolRequest, input projectInput) (*mcp.CallToolResult, healthOutput, error) {
	if err := s.validateProjectRead(input.ProjectID); err != nil {
		return nil, healthOutput{}, err
	}
	health, err := s.backend.Health(ctx, input.ProjectID)
	return nil, healthOutput{SchemaVersion: schemaVersion, Health: health}, err
}

func (s *Server) projectGit(ctx context.Context, _ *mcp.CallToolRequest, input projectInput) (*mcp.CallToolResult, gitOutput, error) {
	if err := s.validateProjectRead(input.ProjectID); err != nil {
		return nil, gitOutput{}, err
	}
	state, err := s.backend.GitState(ctx, input.ProjectID)
	return nil, gitOutput{SchemaVersion: schemaVersion, Git: state}, err
}

func (s *Server) portsList(ctx context.Context, _ *mcp.CallToolRequest, input portsInput) (*mcp.CallToolResult, portsOutput, error) {
	if input.ProjectID != "" {
		if err := s.validateProjectRead(input.ProjectID); err != nil {
			return nil, portsOutput{}, err
		}
	}
	limit, err := bounded(input.Limit, 200, 500, "limit")
	if err != nil {
		return nil, portsOutput{}, err
	}
	registry, err := s.backend.PortRegistry(ctx)
	if err != nil {
		return nil, portsOutput{}, err
	}
	facts := make([]generated.PortFact, 0, min(len(registry.Facts), limit))
	for _, fact := range registry.Facts {
		if fact.ProjectId != nil && s.scope.AuthorizeRead(*fact.ProjectId) != nil {
			continue
		}
		if input.ProjectID == "" || fact.ProjectId != nil && *fact.ProjectId == input.ProjectID {
			facts = append(facts, fact)
		}
		if len(facts) == limit {
			break
		}
	}
	truncated := len(facts) == limit && len(registry.Facts) > len(facts)
	conflicts := s.visiblePortConflicts(registry.Conflicts, input.ProjectID)
	if len(conflicts) > 100 {
		conflicts, truncated = conflicts[:100], true
	}
	return nil, portsOutput{SchemaVersion: schemaVersion, Facts: facts, Conflicts: conflicts, Warnings: registry.Warnings, Truncated: truncated}, nil
}

func (s *Server) portsSuggest(ctx context.Context, _ *mcp.CallToolRequest, input portSuggestionInput) (*mcp.CallToolResult, portSuggestionOutput, error) {
	if err := requestID(input.RequestID); err != nil {
		return nil, portSuggestionOutput{}, err
	}
	if input.ProjectID != "" {
		if err := s.validateProjectRead(input.ProjectID); err != nil {
			return nil, portSuggestionOutput{}, err
		}
	}
	if input.RangeStart < 1 || input.RangeEnd > 65535 || input.RangeEnd < input.RangeStart || input.RangeEnd-input.RangeStart > 10_000 {
		return nil, portSuggestionOutput{}, mcpError("port range must be ordered, valid, and contain at most 10001 candidates")
	}
	suggestion, err := s.backend.SuggestPort(ctx, input.RangeStart, input.RangeEnd, input.Protocol, input.ProjectID, input.Excluded, input.RequestID)
	return nil, portSuggestionOutput{SchemaVersion: schemaVersion, Suggestion: suggestion}, err
}

func (s *Server) actionsList(ctx context.Context, _ *mcp.CallToolRequest, input projectInput) (*mcp.CallToolResult, actionsOutput, error) {
	if err := s.validateProjectRead(input.ProjectID); err != nil {
		return nil, actionsOutput{}, err
	}
	actions, err := s.backend.ProjectActions(ctx, input.ProjectID)
	if err != nil {
		return nil, actionsOutput{}, err
	}
	truncated := len(actions.Actions) > 100
	if truncated {
		actions.Actions = actions.Actions[:100]
	}
	return nil, actionsOutput{SchemaVersion: schemaVersion, Actions: actions, Truncated: truncated}, nil
}

func (s *Server) operationGet(ctx context.Context, _ *mcp.CallToolRequest, input operationInput) (*mcp.CallToolResult, operationOutput, error) {
	if err := required(input.OperationID, "operationId"); err != nil {
		return nil, operationOutput{}, err
	}
	operation, err := s.backend.Operation(ctx, input.OperationID)
	if err == nil {
		err = s.scope.AuthorizeRead(operation.ProjectId)
	}
	return nil, operationOutput{SchemaVersion: schemaVersion, Operation: operation}, err
}

func (s *Server) manifestExplain(ctx context.Context, _ *mcp.CallToolRequest, input projectInput) (*mcp.CallToolResult, manifestOutput, error) {
	if err := s.validateProjectRead(input.ProjectID); err != nil {
		return nil, manifestOutput{}, err
	}
	manifest, err := s.backend.ExplainManifest(ctx, input.ProjectID)
	return nil, manifestOutput{SchemaVersion: schemaVersion, Manifest: manifest}, err
}

func mcpError(message string) error { return &inputError{message: message} }

type inputError struct{ message string }

func (e *inputError) Error() string { return e.message }

func (s *Server) validateProjectRead(projectID string) error {
	if err := required(projectID, "projectId"); err != nil {
		return err
	}
	return s.scope.AuthorizeRead(projectID)
}

func (s *Server) visiblePortConflicts(conflicts []generated.PortConflict, projectID string) []generated.PortConflict {
	if projectID == "" && len(s.scope.ProjectIDs) == 0 {
		return conflicts
	}
	visible := make([]generated.PortConflict, 0, len(conflicts))
	for _, conflict := range conflicts {
		facts := make([]generated.PortFact, 0, len(conflict.Facts))
		hasOwnedFact := false
		for _, fact := range conflict.Facts {
			if fact.ProjectId == nil {
				facts = append(facts, fact)
				continue
			}
			if s.scope.AuthorizeRead(*fact.ProjectId) == nil && (projectID == "" || *fact.ProjectId == projectID) {
				facts = append(facts, fact)
				hasOwnedFact = true
			}
		}
		if hasOwnedFact {
			conflict.Facts = facts
			visible = append(visible, conflict)
		}
	}
	return visible
}
