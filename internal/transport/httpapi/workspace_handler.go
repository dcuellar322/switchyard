package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	operations "switchyard.dev/switchyard/internal/operations/application"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
	workspace "switchyard.dev/switchyard/internal/workspace/application"
	workspaceDomain "switchyard.dev/switchyard/internal/workspace/domain"
)

const maximumWorkspaceDocumentBytes = 256 << 10

func (h *handler) ListWorkspaces(w http.ResponseWriter, r *http.Request) {
	items, err := h.workspaces.List(r.Context())
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	response := make([]generated.Workspace, 0, len(items))
	for _, item := range items {
		response = append(response, workspaceResponse(item))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *handler) GetWorkspace(w http.ResponseWriter, r *http.Request, workspaceID generated.WorkspaceId) {
	item, err := h.workspaces.Get(r.Context(), workspaceID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, workspaceResponse(item))
}

func (h *handler) CreateWorkspace(w http.ResponseWriter, r *http.Request, _ generated.CreateWorkspaceParams) {
	var request generated.WorkspaceDefinition
	if !decodeWorkspaceDocument(w, r, &request) {
		return
	}
	item, err := h.workspaces.Create(r.Context(), saveWorkspaceRequest(request, 0))
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, workspaceResponse(item))
}

func (h *handler) UpdateWorkspace(
	w http.ResponseWriter,
	r *http.Request,
	workspaceID generated.WorkspaceId,
	_ generated.UpdateWorkspaceParams,
) {
	var request generated.WorkspaceUpdate
	if !decodeWorkspaceDocument(w, r, &request) {
		return
	}
	definition := generated.WorkspaceDefinition{
		Name: request.Name, Description: request.Description, Policy: request.Policy, Profile: request.Profile,
		Members: request.Members, Dependencies: request.Dependencies, Recipes: request.Recipes, Profiles: request.Profiles,
	}
	item, err := h.workspaces.Update(r.Context(), workspaceID, saveWorkspaceRequest(definition, request.Revision))
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, workspaceResponse(item))
}

func (h *handler) DeleteWorkspace(
	w http.ResponseWriter,
	r *http.Request,
	workspaceID generated.WorkspaceId,
	_ generated.DeleteWorkspaceParams,
) {
	if err := h.workspaces.Delete(r.Context(), workspaceID); err != nil {
		writeApplicationError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) CreateWorkspaceOperation(
	w http.ResponseWriter,
	r *http.Request,
	workspaceID generated.WorkspaceId,
	params generated.CreateWorkspaceOperationParams,
) {
	var request generated.WorkspaceOperationRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "REQUEST_INVALID", "Request body invalid", "Provide a workspace start or stop action and supported policy options.")
		return
	}
	if _, err := h.workspaces.Get(r.Context(), workspaceID); err != nil {
		writeApplicationError(w, r, err)
		return
	}
	removeData := request.RemoveData != nil && *request.RemoveData
	confirmed := request.ConfirmDataRemoval != nil && *request.ConfirmDataRemoval
	if removeData && !confirmed {
		writeProblem(w, r, http.StatusConflict, "WORKSPACE_DATA_CONFIRMATION_REQUIRED", "Data removal confirmation required", "Bulk stop preserves runtime data unless removeData and confirmDataRemoval are both explicit.")
		return
	}
	input, _ := json.Marshal(request)
	identity := identityFrom(r.Context())
	operation, err := h.operations.Submit(r.Context(), operations.SubmitRequest{
		ProjectID: "workspace:" + workspaceID, WorkspaceID: workspaceID, Kind: "workspace." + string(request.Action), Input: input,
		IdempotencyKey: params.IdempotencyKey, ActorType: string(identity.Access), ActorID: identity.ActorID,
	})
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, operationResponse(operation))
}

func decodeWorkspaceDocument(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maximumWorkspaceDocumentBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "REQUEST_INVALID", "Workspace document invalid", "Provide a bounded workspace graph with known members, dependencies, profiles, and recipes.")
		return false
	}
	return true
}

func saveWorkspaceRequest(request generated.WorkspaceDefinition, revision int64) workspace.SaveRequest {
	result := workspace.SaveRequest{
		Name: request.Name, Description: optionalString(request.Description),
		DefaultFailurePolicy: workspaceDomain.FailurePolicy(request.Policy), DefaultProfileID: optionalString(request.Profile),
		Revision: revision,
	}
	for _, member := range request.Members {
		result.Members = append(result.Members, workspaceDomain.Member{
			ProjectID: member.ProjectId, Role: workspaceDomain.MemberRole(member.Role), Order: member.Order,
			HealthGate: member.HealthGate, HealthTimeout: time.Duration(member.HealthTimeoutSeconds) * time.Second,
		})
	}
	for _, dependency := range request.Dependencies {
		result.Dependencies = append(result.Dependencies, workspaceDomain.Dependency{
			ProjectID: dependency.ProjectId, DependsOnProjectID: dependency.DependsOnProjectId,
		})
	}
	for _, recipe := range request.Recipes {
		result.Recipes = append(result.Recipes, workspaceDomain.Recipe{
			ID: recipe.Id, Name: recipe.Name, Kind: workspaceDomain.RecipeKind(recipe.Kind),
			ProjectID: optionalString(recipe.ProjectId), Target: optionalString(recipe.Target),
			Arguments: append([]string(nil), recipe.Arguments...), Order: recipe.Order,
		})
	}
	for _, profile := range request.Profiles {
		result.Profiles = append(result.Profiles, workspaceDomain.Profile{
			ID: profile.Id, Name: profile.Name, Description: optionalString(profile.Description),
			ProjectIDs: append([]string(nil), profile.ProjectIds...), MaxParallel: profile.MaxParallel,
			LowMemory: profile.LowMemory, MemoryBudgetBytes: optionalUint64(profile.MemoryBudgetBytes),
		})
	}
	return result
}

func workspaceResponse(item workspaceDomain.Workspace) generated.Workspace {
	response := generated.Workspace{
		Id: item.ID, Name: item.Name, Policy: generated.WorkspaceFailurePolicy(item.DefaultFailurePolicy),
		Members: []generated.WorkspaceMember{}, Dependencies: []generated.WorkspaceDependency{},
		Recipes: []generated.WorkspaceRecipe{}, Profiles: []generated.WorkspaceProfile{},
		Revision: item.Revision, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
	response.Description = stringPointer(item.Description)
	response.Profile = stringPointer(item.DefaultProfileID)
	for _, member := range item.Members {
		status, message := memberStatus(item.LastRun, member.ProjectID, member.Status, member.Message)
		response.Members = append(response.Members, generated.WorkspaceMember{
			ProjectId: member.ProjectID, Role: generated.WorkspaceMemberRole(member.Role), Order: member.Order,
			HealthGate: member.HealthGate, HealthTimeoutSeconds: int(member.HealthTimeout / time.Second),
			Status: generated.WorkspaceProjectStatus(status), Message: stringPointer(message),
		})
	}
	for _, dependency := range item.Dependencies {
		response.Dependencies = append(response.Dependencies, generated.WorkspaceDependency{
			ProjectId: dependency.ProjectID, DependsOnProjectId: dependency.DependsOnProjectID,
		})
	}
	for _, recipe := range item.Recipes {
		response.Recipes = append(response.Recipes, generated.WorkspaceRecipe{
			Id: recipe.ID, Name: recipe.Name, Kind: generated.WorkspaceRecipeKind(recipe.Kind),
			ProjectId: stringPointer(recipe.ProjectID), Target: stringPointer(recipe.Target),
			Arguments: append([]string(nil), recipe.Arguments...), Order: recipe.Order,
		})
	}
	for _, profile := range item.Profiles {
		response.Profiles = append(response.Profiles, generated.WorkspaceProfile{
			Id: profile.ID, Name: profile.Name, Description: stringPointer(profile.Description),
			ProjectIds: append([]string(nil), profile.ProjectIDs...), MaxParallel: profile.MaxParallel,
			LowMemory: profile.LowMemory, MemoryBudgetBytes: uint64Pointer(profile.MemoryBudgetBytes),
		})
	}
	if item.LastRun != nil {
		value := workspaceExecutionResponse(*item.LastRun)
		response.LastRun = &value
	}
	return response
}

func workspaceExecutionResponse(item workspaceDomain.ExecutionSummary) generated.WorkspaceExecution {
	response := generated.WorkspaceExecution{
		Id: item.ID, WorkspaceId: item.WorkspaceID, Kind: generated.WorkspaceExecutionKind(item.Kind),
		State: generated.WorkspaceExecutionState(item.State), Policy: generated.WorkspaceFailurePolicy(item.Policy),
		ProfileId: stringPointer(item.ProfileID), RemoveData: item.RemoveData,
		Projects: []generated.WorkspaceProjectResult{}, ErrorMessage: stringPointer(item.ErrorMessage),
		StartedAt: item.StartedAt, FinishedAt: item.FinishedAt,
	}
	for _, project := range item.Projects {
		response.Projects = append(response.Projects, generated.WorkspaceProjectResult{
			ProjectId: project.ProjectID, Role: generated.WorkspaceMemberRole(project.Role),
			Status: generated.WorkspaceProjectStatus(project.Status), Message: stringPointer(project.Message),
			Order: project.Order, StartedAt: project.StartedAt, FinishedAt: project.FinishedAt,
		})
	}
	return response
}

func memberStatus(run *workspaceDomain.ExecutionSummary, projectID string, fallback workspaceDomain.ProjectStatus, message string) (string, string) {
	if run != nil {
		for _, project := range run.Projects {
			if project.ProjectID == projectID {
				return string(project.Status), project.Message
			}
		}
	}
	if fallback != "" {
		return string(fallback), message
	}
	return "idle", ""
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func optionalUint64(value *int64) uint64 {
	if value == nil || *value <= 0 {
		return 0
	}
	return uint64(*value)
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func uint64Pointer(value uint64) *int64 {
	if value == 0 || value > uint64(^uint64(0)>>1) {
		return nil
	}
	converted := int64(value)
	return &converted
}
