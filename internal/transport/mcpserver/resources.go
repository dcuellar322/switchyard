package mcpserver

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

const resourceMIMEType = "application/json"

func (s *Server) addResources() {
	s.mcp.AddResource(&mcp.Resource{URI: "switchyard://system", Name: "switchyard-system", Title: "Switchyard system", Description: "Daemon version and readiness.", MIMEType: resourceMIMEType}, s.readResource)
	s.mcp.AddResource(&mcp.Resource{URI: "switchyard://projects", Name: "switchyard-projects", Title: "Switchyard projects", Description: "Registered projects visible to this agent scope.", MIMEType: resourceMIMEType}, s.readResource)
	s.mcp.AddResource(&mcp.Resource{URI: "switchyard://workspaces", Name: "switchyard-workspaces", Title: "Switchyard workspaces", Description: "Workspace graphs visible to this agent scope.", MIMEType: resourceMIMEType}, s.readResource)
	s.mcp.AddResource(&mcp.Resource{URI: "switchyard://routes", Name: "switchyard-routes", Title: "Switchyard local routes", Description: "Friendly localhost route states visible to this agent scope.", MIMEType: resourceMIMEType}, s.readResource)
	for _, template := range []*mcp.ResourceTemplate{
		{URITemplate: "switchyard://projects/{projectId}", Name: "switchyard-project", Title: "Switchyard project", Description: "One catalog project.", MIMEType: resourceMIMEType},
		{URITemplate: "switchyard://projects/{projectId}/status", Name: "switchyard-project-status", Title: "Project status", Description: "Catalog, runtime, and health status.", MIMEType: resourceMIMEType},
		{URITemplate: "switchyard://projects/{projectId}/manifest", Name: "switchyard-project-manifest", Title: "Effective manifest", Description: "Trusted effective manifest and provenance.", MIMEType: resourceMIMEType},
		{URITemplate: "switchyard://projects/{projectId}/services", Name: "switchyard-project-services", Title: "Project services", Description: "Bounded service observations.", MIMEType: resourceMIMEType},
		{URITemplate: "switchyard://projects/{projectId}/git", Name: "switchyard-project-git", Title: "Project Git status", Description: "Bounded Git state.", MIMEType: resourceMIMEType},
		{URITemplate: "switchyard://projects/{projectId}/ports", Name: "switchyard-project-ports", Title: "Project ports", Description: "Port facts owned by this project.", MIMEType: resourceMIMEType},
		{URITemplate: "switchyard://projects/{projectId}/recent-errors", Name: "switchyard-project-errors", Title: "Recent project errors", Description: "At most 50 redacted recent error log entries.", MIMEType: resourceMIMEType},
		{URITemplate: "switchyard://projects/{projectId}/environments", Name: "switchyard-project-environments", Title: "Project environments", Description: "Registered Git worktrees and exact runtime leases.", MIMEType: resourceMIMEType},
		{URITemplate: "switchyard://workspaces/{workspaceId}", Name: "switchyard-workspace", Title: "Switchyard workspace", Description: "One workspace graph and latest execution.", MIMEType: resourceMIMEType},
	} {
		s.mcp.AddResourceTemplate(template, s.readResource)
	}
}

func (s *Server) readResource(ctx context.Context, request *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	uri, err := url.Parse(request.Params.URI)
	if err != nil || uri.Scheme != "switchyard" {
		return nil, mcp.ResourceNotFoundError(request.Params.URI)
	}
	segments := resourceSegments(uri)
	var value any
	switch {
	case len(segments) == 1 && segments[0] == "system":
		value, err = s.backend.System(ctx)
	case len(segments) == 1 && segments[0] == "projects":
		value, err = s.scopedProjects(ctx)
	case len(segments) == 1 && segments[0] == "workspaces":
		value, err = s.scopedWorkspaces(ctx)
	case len(segments) == 2 && segments[0] == "workspaces":
		value, err = s.readWorkspaceResource(ctx, segments[1])
	case len(segments) == 1 && segments[0] == "routes":
		value, err = s.scopedRoutes(ctx)
	case len(segments) >= 2 && segments[0] == "projects":
		projectID := segments[1]
		if err = s.validateProjectRead(projectID); err == nil {
			value, err = s.readProjectResource(ctx, projectID, segments[2:])
		}
	default:
		return nil, mcp.ResourceNotFoundError(request.Params.URI)
	}
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(struct {
		SchemaVersion string `json:"schemaVersion"`
		Data          any    `json:"data"`
	}{SchemaVersion: schemaVersion, Data: value})
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: request.Params.URI, MIMEType: resourceMIMEType, Text: string(payload)}}}, nil
}

func (s *Server) readProjectResource(ctx context.Context, projectID string, suffix []string) (any, error) {
	switch strings.Join(suffix, "/") {
	case "":
		return s.backend.Project(ctx, projectID)
	case "status":
		return s.readStatusResource(ctx, projectID)
	case "manifest":
		return s.backend.ExplainManifest(ctx, projectID)
	case "services":
		return s.readServicesResource(ctx, projectID)
	case "git":
		return s.backend.GitState(ctx, projectID)
	case "ports":
		return s.readPortsResource(ctx, projectID)
	case "recent-errors":
		return s.readRecentErrorsResource(ctx, projectID)
	case "environments":
		api, ok := s.backend.(environmentBackend)
		if !ok {
			return nil, mcp.ResourceNotFoundError("switchyard://projects/" + projectID + "/environments")
		}
		return api.ProjectEnvironments(ctx, projectID)
	default:
		return nil, mcp.ResourceNotFoundError("switchyard://projects/" + projectID + "/" + strings.Join(suffix, "/"))
	}
}

func (s *Server) scopedWorkspaces(ctx context.Context) ([]generated.Workspace, error) {
	api, ok := s.backend.(workspaceBackend)
	if !ok {
		return nil, mcp.ResourceNotFoundError("switchyard://workspaces")
	}
	items, err := api.Workspaces(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]generated.Workspace, 0, min(100, len(items)))
	for _, workspace := range items {
		if s.authorizeWorkspaceRead(ctx, workspace) == nil {
			result = append(result, workspace)
		}
		if len(result) == 100 {
			break
		}
	}
	return result, nil
}

func (s *Server) readWorkspaceResource(ctx context.Context, workspaceID string) (generated.Workspace, error) {
	api, ok := s.backend.(workspaceBackend)
	if !ok {
		return generated.Workspace{}, mcp.ResourceNotFoundError("switchyard://workspaces/" + workspaceID)
	}
	workspace, err := api.Workspace(ctx, workspaceID)
	if err == nil {
		err = s.authorizeWorkspaceRead(ctx, workspace)
	}
	return workspace, err
}

func (s *Server) scopedRoutes(ctx context.Context) ([]generated.LocalRoute, error) {
	api, ok := s.backend.(environmentBackend)
	if !ok {
		return nil, mcp.ResourceNotFoundError("switchyard://routes")
	}
	routes, err := api.LocalRoutes(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]generated.LocalRoute, 0, len(routes))
	for _, route := range routes {
		if route.ProjectId != nil && s.scope.AuthorizeRead(*route.ProjectId) == nil {
			result = append(result, route)
		}
	}
	return result, nil
}

func (s *Server) readStatusResource(ctx context.Context, projectID string) (statusOutput, error) {
	project, err := s.backend.Project(ctx, projectID)
	if err != nil {
		return statusOutput{}, err
	}
	runtime, err := s.backend.Runtime(ctx, projectID)
	if err != nil {
		return statusOutput{}, err
	}
	health, err := s.backend.Health(ctx, projectID)
	return statusOutput{SchemaVersion: schemaVersion, Project: project, Runtime: runtime, Health: health}, err
}

func (s *Server) readServicesResource(ctx context.Context, projectID string) ([]generated.RuntimeServiceObservation, error) {
	runtime, err := s.backend.Runtime(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if len(runtime.Services) > 100 {
		runtime.Services = runtime.Services[:100]
	}
	return runtime.Services, nil
}

func (s *Server) readPortsResource(ctx context.Context, projectID string) ([]generated.PortFact, error) {
	registry, err := s.backend.PortRegistry(ctx)
	if err != nil {
		return nil, err
	}
	facts := make([]generated.PortFact, 0)
	for _, fact := range registry.Facts {
		if fact.ProjectId != nil && *fact.ProjectId == projectID {
			facts = append(facts, fact)
		}
		if len(facts) == 500 {
			break
		}
	}
	return facts, nil
}

func (s *Server) readRecentErrorsResource(ctx context.Context, projectID string) ([]generated.RuntimeLogEntry, error) {
	entries, err := s.backend.RuntimeLogs(ctx, projectID, "", "", "", "", 200)
	if err != nil {
		return nil, err
	}
	errorsOnly := make([]generated.RuntimeLogEntry, 0, 50)
	for _, entry := range entries {
		if strings.EqualFold(entry.Level, "error") || string(entry.Stream) == "stderr" {
			if entry.Attributes == nil {
				entry.Attributes = map[string]string{}
			}
			errorsOnly = append(errorsOnly, entry)
		}
		if len(errorsOnly) == 50 {
			break
		}
	}
	return errorsOnly, nil
}

func (s *Server) scopedProjects(ctx context.Context) ([]generated.Project, error) {
	projects, err := s.backend.Projects(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]generated.Project, 0, min(len(projects), 100))
	for _, project := range projects {
		if s.scope.AuthorizeRead(project.Id) == nil {
			result = append(result, project)
		}
		if len(result) == 100 {
			break
		}
	}
	return result, nil
}

func resourceSegments(uri *url.URL) []string {
	result := []string{uri.Host}
	for _, segment := range strings.Split(strings.Trim(uri.Path, "/"), "/") {
		if segment != "" {
			result = append(result, segment)
		}
	}
	return result
}
