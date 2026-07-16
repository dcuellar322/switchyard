package httpapi

import (
	"encoding/json"
	"net/http"

	environmentApplication "switchyard.dev/switchyard/internal/environments/application"
	environmentDomain "switchyard.dev/switchyard/internal/environments/domain"
	routingDomain "switchyard.dev/switchyard/internal/routing/domain"
	"switchyard.dev/switchyard/internal/transport/contract/generated"
)

func (h *handler) ListProjectEnvironments(w http.ResponseWriter, r *http.Request, projectID generated.ProjectId) {
	environments, err := h.environments.ListProject(r.Context(), projectID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	response := make([]generated.ProjectEnvironment, 0, len(environments))
	for _, environment := range environments {
		response = append(response, environmentResponse(environment))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *handler) RegisterProjectEnvironments(
	w http.ResponseWriter,
	r *http.Request,
	projectID generated.ProjectId,
	_ generated.RegisterProjectEnvironmentsParams,
) {
	registration, err := h.environmentRegistration.RegisterWorktrees(r.Context(), projectID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	if h.routes != nil {
		if _, err := h.routes.Refresh(r.Context()); err != nil {
			writeApplicationError(w, r, err)
			return
		}
	}
	response := generated.EnvironmentRegistration{
		ProjectId: registration.ProjectID, RemovedIds: registration.RemovedIDs,
		ObservedAt: registration.ObservedAt, Environments: []generated.ProjectEnvironment{},
	}
	for _, environment := range registration.Environments {
		response.Environments = append(response.Environments, environmentResponse(environment))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *handler) GetEnvironment(w http.ResponseWriter, r *http.Request, environmentID string) {
	environment, err := h.environments.Get(r.Context(), environmentID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, environmentResponse(environment))
}

func (h *handler) UpdateEnvironment(
	w http.ResponseWriter,
	r *http.Request,
	environmentID string,
	_ generated.UpdateEnvironmentParams,
) {
	var request generated.EnvironmentUpdate
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "REQUEST_INVALID", "Environment update invalid", "Provide one valid .localhost hostname.")
		return
	}
	current, err := h.environments.Get(r.Context(), environmentID)
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	updated, err := h.environments.ConfigureRuntime(r.Context(), environmentID, environmentApplication.RuntimeConfiguration{
		State: current.State, Hostname: request.Hostname, Target: current.Target,
		PortLeases: current.Allocation.PortLeases,
	})
	if err != nil {
		writeApplicationError(w, r, err)
		return
	}
	if h.routes != nil {
		if _, err := h.routes.Refresh(r.Context()); err != nil {
			writeApplicationError(w, r, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, environmentResponse(updated))
}

func (h *handler) ListLocalRoutes(w http.ResponseWriter, _ *http.Request) {
	routes := h.routes.Snapshot()
	response := make([]generated.LocalRoute, 0, len(routes))
	for _, route := range routes {
		response = append(response, routeResponse(route))
	}
	writeJSON(w, http.StatusOK, response)
}

func environmentResponse(environment environmentDomain.Environment) generated.ProjectEnvironment {
	leasing := make([]generated.EnvironmentPortLease, 0, len(environment.Allocation.PortLeases))
	for _, lease := range environment.Allocation.PortLeases {
		leasing = append(leasing, generated.EnvironmentPortLease{
			PortId: lease.PortID, Protocol: generated.EnvironmentPortLeaseProtocol(lease.Protocol),
			TargetPort: lease.TargetPort, HostPort: lease.HostPort,
		})
	}
	return generated.ProjectEnvironment{
		Id: environment.ID, ProjectId: environment.ProjectID, Name: environment.Name, Path: environment.Path,
		Head: stringPointer(environment.Head), Branch: stringPointer(environment.Branch), Detached: environment.Detached,
		Bare: environment.Bare, Locked: environment.Locked, Primary: environment.Primary,
		Availability:      generated.ProjectEnvironmentAvailability(environment.Availability),
		UnavailableReason: stringPointer(environment.UnavailableReason), State: generated.ProjectEnvironmentState(environment.State),
		Hostname: environment.Hostname, Target: stringPointer(environment.Target),
		Allocation: generated.EnvironmentRuntimeAllocation{
			ComposeProjectName: environment.Allocation.ComposeProjectName,
			PortLeaseNamespace: environment.Allocation.PortLeaseNamespace,
			PortOffset:         environment.Allocation.PortOffset, PortLeases: leasing,
		},
		RegisteredAt: environment.RegisteredAt, LastObservedAt: environment.LastObservedAt, UpdatedAt: environment.UpdatedAt,
	}
}

func routeResponse(route routingDomain.Route) generated.LocalRoute {
	return generated.LocalRoute{
		Hostname: route.Hostname, Status: generated.LocalRouteStatus(route.Status),
		ProjectId: stringPointer(route.ProjectID), EnvironmentId: stringPointer(route.EnvironmentID),
		Target: stringPointer(route.Target), Reason: stringPointer(route.Reason),
		CandidateEnvironmentIds: append([]string(nil), route.CandidateEnvironmentIDs...), UpdatedAt: route.UpdatedAt,
	}
}
