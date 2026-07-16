package adapters

import (
	"context"

	"switchyard.dev/switchyard/internal/environments/application"
	"switchyard.dev/switchyard/internal/environments/domain"
	runtimeApplication "switchyard.dev/switchyard/internal/runtime/application"
)

// RuntimeSource adapts durable environments to the runtime consumer's narrow DTO.
type RuntimeSource struct{ environments *application.Service }

// NewRuntimeSource creates the runtime-facing project-environment adapter.
func NewRuntimeSource(environments *application.Service) *RuntimeSource {
	return &RuntimeSource{environments: environments}
}

// ResolveRuntimeEnvironment returns only trusted runtime allocation facts.
func (s *RuntimeSource) ResolveRuntimeEnvironment(ctx context.Context, id string) (runtimeApplication.RuntimeEnvironment, error) {
	environment, err := s.environments.Get(ctx, id)
	if err != nil {
		return runtimeApplication.RuntimeEnvironment{}, err
	}
	leases := make(map[string]int, len(environment.Allocation.PortLeases))
	for _, lease := range environment.Allocation.PortLeases {
		leases[lease.PortID] = lease.HostPort
	}
	return runtimeApplication.RuntimeEnvironment{
		ID: environment.ID, ProjectID: environment.ProjectID, Name: environment.Name, Root: environment.Path,
		Available:          environment.Availability == domain.AvailabilityAvailable && environment.State != domain.StateUnavailable,
		ComposeProjectName: environment.Allocation.ComposeProjectName, PortLeases: leases,
	}, nil
}

// ListRuntimeEnvironmentIDs returns environments eligible for observation.
func (s *RuntimeSource) ListRuntimeEnvironmentIDs(ctx context.Context) ([]string, error) {
	environments, err := s.environments.List(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(environments))
	for _, environment := range environments {
		if environment.Availability == domain.AvailabilityAvailable && environment.State != domain.StateUnavailable {
			ids = append(ids, environment.ID)
		}
	}
	return ids, nil
}
