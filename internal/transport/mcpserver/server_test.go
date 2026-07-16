package mcpserver

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	agents "switchyard.dev/switchyard/internal/agents/application"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

type fakeBackend struct {
	projects         []generated.Project
	actions          []generated.ActionDefinition
	logs             []generated.RuntimeLogEntry
	operation        generated.Operation
	lastAction       generated.RuntimeAction
	lastServices     []string
	lastRequestID    string
	lastTail         int
	cancelled        bool
	proposalCreated  bool
	proposalAccepted bool
	proposalProject  string
	registry         generated.PortRegistry
}

func (f *fakeBackend) System(context.Context) (generated.SystemInfo, error) {
	return generated.SystemInfo{Status: "ready", Version: "test", ApiVersion: "v1"}, nil
}
func (f *fakeBackend) Projects(context.Context) ([]generated.Project, error) {
	return slices.Clone(f.projects), nil
}
func (f *fakeBackend) Project(_ context.Context, id string) (generated.Project, error) {
	for _, project := range f.projects {
		if project.Id == id {
			return project, nil
		}
	}
	return generated.Project{Id: id, DisplayName: id}, nil
}
func (f *fakeBackend) Runtime(_ context.Context, id string) (generated.RuntimeObservation, error) {
	return generated.RuntimeObservation{ProjectId: id, Services: []generated.RuntimeServiceObservation{}}, nil
}
func (f *fakeBackend) RuntimeLogs(_ context.Context, _, _, _, _, _ string, tail int) ([]generated.RuntimeLogEntry, error) {
	f.lastTail = tail
	if len(f.logs) > tail {
		return slices.Clone(f.logs[:tail]), nil
	}
	return slices.Clone(f.logs), nil
}
func (f *fakeBackend) Health(_ context.Context, id string) (generated.ProjectHealth, error) {
	return generated.ProjectHealth{ProjectId: id, Results: []generated.HealthResult{}}, nil
}
func (f *fakeBackend) GitState(_ context.Context, id string) (generated.GitState, error) {
	return generated.GitState{ProjectId: id, Remotes: []generated.GitRemote{}, Worktrees: []generated.GitWorktree{}}, nil
}
func (f *fakeBackend) PortRegistry(context.Context) (generated.PortRegistry, error) {
	if f.registry.Facts == nil {
		f.registry.Facts = []generated.PortFact{}
	}
	if f.registry.Conflicts == nil {
		f.registry.Conflicts = []generated.PortConflict{}
	}
	if f.registry.Warnings == nil {
		f.registry.Warnings = []string{}
	}
	return f.registry, nil
}
func (f *fakeBackend) SuggestPort(_ context.Context, start, _ int, protocol, _ string, _ []int, _ string) (generated.PortSuggestion, error) {
	return generated.PortSuggestion{Port: start, RangeStart: start, RangeEnd: start, Protocol: generated.PortSuggestionProtocol(protocol)}, nil
}
func (f *fakeBackend) ProjectActions(_ context.Context, id string) (generated.ProjectActions, error) {
	return generated.ProjectActions{ProjectId: id, Actions: slices.Clone(f.actions)}, nil
}
func (f *fakeBackend) Operation(context.Context, string) (generated.Operation, error) {
	return f.operation, nil
}
func (f *fakeBackend) ExplainManifest(_ context.Context, id string) (generated.EffectiveManifest, error) {
	return generated.EffectiveManifest{Manifest: map[string]any{"projectId": id}, Provenance: map[string]string{}, Sources: []generated.ManifestSource{}}, nil
}
func (f *fakeBackend) CreateRuntimeOperationForServices(_ context.Context, projectID string, action generated.RuntimeAction, _ bool, services []string, requestID string) (generated.Operation, error) {
	f.lastAction, f.lastServices, f.lastRequestID = action, slices.Clone(services), requestID
	return generated.Operation{Id: "operation-created", ProjectId: projectID, Kind: "runtime." + string(action), State: generated.OperationStateQueued}, nil
}
func (f *fakeBackend) CreateActionOperation(_ context.Context, projectID, actionID string, _, _ bool, requestID string) (generated.Operation, error) {
	f.lastRequestID = requestID
	return generated.Operation{Id: "action-created", ProjectId: projectID, Kind: "action." + actionID, State: generated.OperationStateQueued}, nil
}
func (f *fakeBackend) CancelOperation(_ context.Context, _, _ string) (generated.Operation, error) {
	f.cancelled = true
	return f.operation, nil
}
func (f *fakeBackend) CreateManifestProposal(context.Context, string, string) (generated.ManifestProposal, error) {
	f.proposalCreated = true
	return generated.ManifestProposal{Id: "proposal-1", Evidence: []generated.DiscoveryEvidence{}, Unresolved: []string{}}, nil
}
func (f *fakeBackend) ManifestProposal(context.Context, string) (generated.ManifestProposal, error) {
	projectID := f.proposalProject
	if projectID == "" {
		projectID = "project-1"
	}
	return generated.ManifestProposal{Id: "proposal-1", ProjectId: projectID, Evidence: []generated.DiscoveryEvidence{}, Unresolved: []string{}}, nil
}
func (f *fakeBackend) AcceptManifestProposal(context.Context, string, string) (generated.AcceptedManifestProposal, error) {
	f.proposalAccepted = true
	return generated.AcceptedManifestProposal{}, nil
}

func TestProfileToolListsAreStaticAndLeastPrivilege(t *testing.T) {
	profiles := []struct {
		profile agents.Profile
		present []string
		absent  []string
	}{
		{agents.ProfileObserve, []string{"switchyard_system_info", "switchyard_operation_wait"}, []string{"switchyard_project_start", "switchyard_action_run", "switchyard_project_teardown"}},
		{agents.ProfileDevelop, []string{"switchyard_project_start", "switchyard_action_run", "switchyard_operation_cancel"}, []string{"switchyard_project_rebuild", "switchyard_project_teardown"}},
		{agents.ProfileMaintain, []string{"switchyard_project_rebuild", "switchyard_manifest_proposal_create"}, []string{"switchyard_manifest_proposal_accept", "switchyard_project_teardown"}},
		{agents.ProfileAdmin, []string{"switchyard_manifest_proposal_accept", "switchyard_project_teardown"}, nil},
	}
	for _, test := range profiles {
		t.Run(string(test.profile), func(t *testing.T) {
			session := connectTestClient(t, &fakeBackend{}, test.profile, nil)
			listed, err := session.ListTools(context.Background(), nil)
			if err != nil {
				t.Fatal(err)
			}
			names := make([]string, len(listed.Tools))
			for index, tool := range listed.Tools {
				names[index] = tool.Name
			}
			for _, name := range test.present {
				if !slices.Contains(names, name) {
					t.Errorf("missing tool %s from %v", name, names)
				}
			}
			for _, name := range test.absent {
				if slices.Contains(names, name) {
					t.Errorf("unexpected tool %s in %v", name, names)
				}
			}
		})
	}
}

func TestDevelopCanStartAndWaitButCannotRunDestructiveAction(t *testing.T) {
	backend := &fakeBackend{
		actions:   []generated.ActionDefinition{{Id: "destroy", Risk: generated.ActionDefinitionRiskDestructive}},
		operation: generated.Operation{Id: "operation-1", ProjectId: "project-1", State: generated.OperationStateSucceeded},
	}
	session := connectTestClient(t, backend, agents.ProfileDevelop, []string{"project-1"})
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "switchyard_project_start", Arguments: map[string]any{"projectId": "project-1", "serviceIds": []string{"api"}, "requestId": "request-123"}})
	if err != nil || result.IsError {
		t.Fatalf("start error=%v result=%#v", err, result)
	}
	if backend.lastAction != generated.Start || !slices.Equal(backend.lastServices, []string{"api"}) || backend.lastRequestID != "request-123" {
		t.Fatalf("backend call = action=%s services=%v request=%s", backend.lastAction, backend.lastServices, backend.lastRequestID)
	}
	result, err = session.CallTool(context.Background(), &mcp.CallToolParams{Name: "switchyard_operation_wait", Arguments: map[string]any{"operationId": "operation-1", "timeoutSeconds": 1}})
	if err != nil || result.IsError {
		t.Fatalf("wait error=%v result=%#v", err, result)
	}
	result, err = session.CallTool(context.Background(), &mcp.CallToolParams{Name: "switchyard_action_run", Arguments: map[string]any{"projectId": "project-1", "actionId": "destroy", "confirmRisk": true, "requestId": "request-456"}})
	if err == nil && !result.IsError {
		t.Fatal("develop profile ran a destructive action")
	}
}

func TestResourcesPromptsBoundsAndScopeConformOverSDK(t *testing.T) {
	logs := make([]generated.RuntimeLogEntry, 500)
	backend := &fakeBackend{
		projects: []generated.Project{{Id: "project-1", DisplayName: "Visible"}, {Id: "project-2", DisplayName: "Hidden"}},
		logs:     logs,
	}
	session := connectTestClient(t, backend, agents.ProfileObserve, []string{"project-1"})
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name == "switchyard_system_info" && (tool.Annotations == nil || !tool.Annotations.ReadOnlyHint) {
			t.Fatal("system tool lacks read-only annotation")
		}
	}
	if _, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "switchyard_project_logs_query", Arguments: map[string]any{"projectId": "project-1", "tail": 500}}); err != nil {
		t.Fatal(err)
	}
	if backend.lastTail != 500 {
		t.Fatalf("log tail = %d", backend.lastTail)
	}
	resource, err := session.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "switchyard://projects"})
	if err != nil {
		t.Fatal(err)
	}
	text := resource.Contents[0].Text
	if !strings.Contains(text, "project-1") || strings.Contains(text, "project-2") {
		t.Fatalf("scoped project resource = %s", text)
	}
	prompts, err := session.ListPrompts(context.Background(), nil)
	if err != nil || len(prompts.Prompts) != 4 {
		t.Fatalf("prompts error=%v count=%d", err, len(prompts.Prompts))
	}
	prompt, err := session.GetPrompt(context.Background(), &mcp.GetPromptParams{Name: "switchyard_diagnose_project", Arguments: map[string]string{"projectId": "project-1"}})
	if err != nil || len(prompt.Messages) != 1 {
		t.Fatalf("prompt error=%v result=%#v", err, prompt)
	}
}

func TestProjectScopeProtectsProposalsAndPortConflictFacts(t *testing.T) {
	projectOne, projectTwo := "project-1", "project-2"
	backend := &fakeBackend{
		proposalProject: projectTwo,
		registry: generated.PortRegistry{
			Facts:     []generated.PortFact{{Id: "visible", ProjectId: &projectOne}, {Id: "hidden", ProjectId: &projectTwo}},
			Conflicts: []generated.PortConflict{{Id: "visible-conflict", Facts: []generated.PortFact{{Id: "visible", ProjectId: &projectOne}, {Id: "listener"}}}, {Id: "hidden-conflict", Facts: []generated.PortFact{{Id: "hidden", ProjectId: &projectTwo}, {Id: "listener"}}}},
			Warnings:  []string{},
		},
	}
	session := connectTestClient(t, backend, agents.ProfileAdmin, []string{projectOne})
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "switchyard_manifest_proposal_accept", Arguments: map[string]any{"proposalId": "proposal-1", "requestId": "request-accept"}})
	if err == nil && !result.IsError {
		t.Fatal("project-scoped admin accepted a proposal for a hidden project")
	}
	result, err = session.CallTool(context.Background(), &mcp.CallToolParams{Name: "switchyard_manifest_proposal_create", Arguments: map[string]any{"path": "/tmp/new-project", "requestId": "request-create"}})
	if err == nil && !result.IsError {
		t.Fatal("project-scoped agent registered a new project")
	}
	result, err = session.CallTool(context.Background(), &mcp.CallToolParams{Name: "switchyard_ports_list", Arguments: map[string]any{}})
	if err != nil || result.IsError {
		t.Fatalf("ports error=%v result=%#v", err, result)
	}
	payload, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), "visible-conflict") || strings.Contains(string(payload), "hidden-conflict") || strings.Contains(string(payload), `"id":"hidden"`) {
		t.Fatalf("scoped ports = %s", payload)
	}
}

func connectTestClient(t *testing.T, backend *fakeBackend, profile agents.Profile, projectIDs []string) *mcp.ClientSession {
	t.Helper()
	scope, err := agents.NewScope("test", "agent", profile, projectIDs)
	if err != nil {
		t.Fatal(err)
	}
	server := New(backend, scope, "test", slog.New(slog.NewTextHandler(io.Discard, nil)))
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.ProtocolServer().Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "switchyard-test", Version: "test"}, nil)
	clientSession, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		_ = serverSession.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = clientSession.Close()
		_ = serverSession.Close()
	})
	return clientSession
}
