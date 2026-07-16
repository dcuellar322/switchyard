package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	agents "switchyard.dev/switchyard/internal/agents/application"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

func (s *Server) addMutationTools() {
	if s.scope.Allows(agents.CapabilityLifecycle) {
		mcp.AddTool(s.mcp, mutationTool("switchyard_project_start", "Start project", "Queue an idempotent start for a project or selected declared services.", "mutating", agents.ProfileDevelop, false, true, false), lifecycleHandler(s, generated.RuntimeActionStart, agents.CapabilityLifecycle))
		mcp.AddTool(s.mcp, mutationTool("switchyard_project_stop", "Stop project", "Queue an idempotent stop for a project or selected declared services.", "mutating", agents.ProfileDevelop, false, true, false), lifecycleHandler(s, generated.RuntimeActionStop, agents.CapabilityLifecycle))
		mcp.AddTool(s.mcp, mutationTool("switchyard_project_restart", "Restart project", "Queue a restart for a project or selected declared services.", "mutating", agents.ProfileDevelop, false, false, false), lifecycleHandler(s, generated.RuntimeActionRestart, agents.CapabilityLifecycle))
		mcp.AddTool(s.mcp, mutationTool("switchyard_project_pause", "Pause project", "Queue an idempotent pause for a project or selected declared services.", "mutating", agents.ProfileDevelop, false, true, false), lifecycleHandler(s, generated.RuntimeActionPause, agents.CapabilityLifecycle))
		mcp.AddTool(s.mcp, mutationTool("switchyard_project_resume", "Resume project", "Queue an idempotent resume for a project or selected declared services.", "mutating", agents.ProfileDevelop, false, true, false), lifecycleHandler(s, generated.RuntimeActionUnpause, agents.CapabilityLifecycle))
		mcp.AddTool(s.mcp, mutationTool("switchyard_workspace_start", "Start workspace", "Queue dependency-ordered workspace start with bounded concurrency.", "mutating", agents.ProfileDevelop, false, true, false), s.workspaceStart)
		mcp.AddTool(s.mcp, mutationTool("switchyard_workspace_stop", "Stop workspace", "Queue dependency-safe workspace stop; data removal is separately authorized.", "conditional-destructive", agents.ProfileDevelop, true, true, false), s.workspaceStop)
		mcp.AddTool(s.mcp, mutationTool("switchyard_environments_register", "Register worktree environments", "Reconcile trusted Git worktrees and allocate exact ports without running repository code.", "filesystem-read", agents.ProfileDevelop, false, true, false), s.environmentsRegister)
	}
	if s.scope.Allows(agents.CapabilityRebuild) {
		mcp.AddTool(s.mcp, mutationTool("switchyard_project_rebuild", "Rebuild project", "Queue an explicit rebuild for a project or selected declared services.", "mutating", agents.ProfileMaintain, false, false, true), lifecycleHandler(s, generated.RuntimeActionRebuild, agents.CapabilityRebuild))
	}
	if s.scope.Allows(agents.CapabilityAction) {
		mcp.AddTool(s.mcp, mutationTool("switchyard_action_run", "Run trusted action", "Queue one reviewed manifest action; risk confirmation is enforced.", "declared", agents.ProfileDevelop, true, false, true), s.actionRun)
		mcp.AddTool(s.mcp, mutationTool("switchyard_operation_cancel", "Cancel operation", "Request cooperative cancellation of one durable operation.", "mutating", agents.ProfileDevelop, false, true, false), s.operationCancel)
	}
	if s.scope.Allows(agents.CapabilityProposalCreate) {
		mcp.AddTool(s.mcp, mutationTool("switchyard_manifest_proposal_create", "Create manifest proposal", "Scan a local repository without executing its code and return an untrusted proposal for review.", "filesystem-read", agents.ProfileMaintain, false, true, false), s.proposalCreate)
	}
	if s.scope.Allows(agents.CapabilityProposalAccept) {
		mcp.AddTool(s.mcp, mutationTool("switchyard_manifest_proposal_accept", "Accept manifest proposal", "Accept a previously validated proposal as an explicit trust decision.", "trust-decision", agents.ProfileAdmin, false, true, false), s.proposalAccept)
	}
	if s.scope.Allows(agents.CapabilityDestructive) {
		mcp.AddTool(s.mcp, mutationTool("switchyard_project_teardown", "Tear down project", "Tear down Compose resources; volume removal is explicit and destructive.", "destructive", agents.ProfileAdmin, true, true, false), s.teardown)
	}
}

func lifecycleHandler(s *Server, action generated.RuntimeAction, capability agents.Capability) mcp.ToolHandlerFor[lifecycleInput, mutationOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input lifecycleInput) (*mcp.CallToolResult, mutationOutput, error) {
		if err := s.validateProjectMutation(capability, input.ProjectID, input.RequestID); err != nil {
			return nil, mutationOutput{}, err
		}
		services, err := serviceIDs(input.ServiceIDs)
		if err != nil {
			return nil, mutationOutput{}, err
		}
		operation, err := s.backend.CreateRuntimeOperationForServices(ctx, input.ProjectID, action, false, services, input.RequestID)
		return nil, mutationOutput{SchemaVersion: schemaVersion, Operation: operation}, err
	}
}

func (s *Server) teardown(ctx context.Context, _ *mcp.CallToolRequest, input teardownInput) (*mcp.CallToolResult, mutationOutput, error) {
	if err := s.validateProjectMutation(agents.CapabilityDestructive, input.ProjectID, input.RequestID); err != nil {
		return nil, mutationOutput{}, err
	}
	operation, err := s.backend.CreateRuntimeOperationForServices(ctx, input.ProjectID, generated.RuntimeActionTeardown, input.RemoveVolumes, nil, input.RequestID)
	return nil, mutationOutput{SchemaVersion: schemaVersion, Operation: operation}, err
}

func (s *Server) actionRun(ctx context.Context, _ *mcp.CallToolRequest, input actionInput) (*mcp.CallToolResult, mutationOutput, error) {
	if err := s.validateProjectMutation(agents.CapabilityAction, input.ProjectID, input.RequestID); err != nil {
		return nil, mutationOutput{}, err
	}
	if err := required(input.ActionID, "actionId"); err != nil {
		return nil, mutationOutput{}, err
	}
	actions, err := s.backend.ProjectActions(ctx, input.ProjectID)
	if err != nil {
		return nil, mutationOutput{}, err
	}
	index := slices.IndexFunc(actions.Actions, func(action generated.ActionDefinition) bool { return action.Id == input.ActionID })
	if index < 0 {
		return nil, mutationOutput{}, fmt.Errorf("trusted action %q was not found", input.ActionID)
	}
	action := actions.Actions[index]
	if action.Risk != generated.ActionDefinitionRiskReadOnly && !input.ConfirmRisk {
		return nil, mutationOutput{}, errors.New("confirmRisk is required for a risk-bearing action")
	}
	if action.Risk == generated.ActionDefinitionRiskDestructive {
		if err := s.scope.Authorize(agents.CapabilityDestructive, input.ProjectID); err != nil {
			return nil, mutationOutput{}, err
		}
	}
	if input.AllowOutsideRoot && s.scope.Profile != agents.ProfileAdmin {
		return nil, mutationOutput{}, errors.New("allowOutsideRoot requires the admin profile")
	}
	operation, err := s.backend.CreateActionOperation(ctx, input.ProjectID, input.ActionID, input.ConfirmRisk, input.AllowOutsideRoot, input.RequestID)
	return nil, mutationOutput{SchemaVersion: schemaVersion, Operation: operation}, err
}

func (s *Server) operationCancel(ctx context.Context, _ *mcp.CallToolRequest, input cancelInput) (*mcp.CallToolResult, mutationOutput, error) {
	if err := required(input.OperationID, "operationId"); err != nil {
		return nil, mutationOutput{}, err
	}
	if err := requestID(input.RequestID); err != nil {
		return nil, mutationOutput{}, err
	}
	operation, err := s.backend.Operation(ctx, input.OperationID)
	if err != nil {
		return nil, mutationOutput{}, err
	}
	if err := s.scope.Authorize(agents.CapabilityOperationCancel, operation.ProjectId); err != nil {
		return nil, mutationOutput{}, err
	}
	operation, err = s.backend.CancelOperation(ctx, input.OperationID, input.RequestID)
	return nil, mutationOutput{SchemaVersion: schemaVersion, Operation: operation}, err
}

func (s *Server) proposalCreate(ctx context.Context, _ *mcp.CallToolRequest, input proposalCreateInput) (*mcp.CallToolResult, proposalOutput, error) {
	if err := s.scope.Authorize(agents.CapabilityProposalCreate, ""); err != nil {
		return nil, proposalOutput{}, err
	}
	if len(s.scope.ProjectIDs) > 0 {
		return nil, proposalOutput{}, fmt.Errorf("%w: project-scoped agents cannot register a new project", agents.ErrPermissionDenied)
	}
	if err := required(input.Path, "path"); err != nil {
		return nil, proposalOutput{}, err
	}
	if err := requestID(input.RequestID); err != nil {
		return nil, proposalOutput{}, err
	}
	proposal, err := s.backend.CreateManifestProposal(ctx, input.Path, input.RequestID)
	return nil, proposalOutput{SchemaVersion: schemaVersion, Proposal: proposal}, err
}

func (s *Server) proposalAccept(ctx context.Context, _ *mcp.CallToolRequest, input proposalAcceptInput) (*mcp.CallToolResult, proposalAcceptOutput, error) {
	if err := s.scope.Authorize(agents.CapabilityProposalAccept, ""); err != nil {
		return nil, proposalAcceptOutput{}, err
	}
	if err := required(input.ProposalID, "proposalId"); err != nil {
		return nil, proposalAcceptOutput{}, err
	}
	if err := requestID(input.RequestID); err != nil {
		return nil, proposalAcceptOutput{}, err
	}
	proposal, err := s.backend.ManifestProposal(ctx, input.ProposalID)
	if err != nil {
		return nil, proposalAcceptOutput{}, err
	}
	if err := s.scope.AuthorizeRead(proposal.ProjectId); err != nil {
		return nil, proposalAcceptOutput{}, err
	}
	accepted, err := s.backend.AcceptManifestProposal(ctx, input.ProposalID, input.RequestID)
	return nil, proposalAcceptOutput{SchemaVersion: schemaVersion, Accepted: accepted}, err
}

func (s *Server) validateProjectMutation(capability agents.Capability, projectID, idempotencyKey string) error {
	if err := required(projectID, "projectId"); err != nil {
		return err
	}
	if err := requestID(idempotencyKey); err != nil {
		return err
	}
	return s.scope.Authorize(capability, projectID)
}

func serviceIDs(values []string) ([]string, error) {
	if len(values) > 16 {
		return nil, errors.New("serviceIds may contain at most 16 entries")
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if err := required(value, "serviceIds"); err != nil {
			return nil, err
		}
		if slices.Contains(result, value) {
			return nil, fmt.Errorf("serviceIds contains duplicate %q", value)
		}
		result = append(result, value)
	}
	return result, nil
}
