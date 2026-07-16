// Package adapters supplies local routing sources and the HTTP reverse proxy.
package adapters

import (
	"context"

	environmentDomain "switchyard.dev/switchyard/internal/environments/domain"
	routingDomain "switchyard.dev/switchyard/internal/routing/domain"
)

// EnvironmentReader is the narrow registry surface needed to build routes.
type EnvironmentReader interface {
	List(context.Context) ([]environmentDomain.Environment, error)
}

// EnvironmentSource maps registered environments to route candidates.
type EnvironmentSource struct{ environments EnvironmentReader }

// NewEnvironmentSource creates the routing-facing environment adapter.
func NewEnvironmentSource(environments EnvironmentReader) *EnvironmentSource {
	return &EnvironmentSource{environments: environments}
}

// Candidates returns all friendly host claims including unavailable entries so
// conflicts and inactive environments remain visible.
func (s *EnvironmentSource) Candidates(ctx context.Context) ([]routingDomain.Candidate, error) {
	environments, err := s.environments.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]routingDomain.Candidate, 0, len(environments))
	for _, environment := range environments {
		result = append(result, routingDomain.Candidate{
			ProjectID: environment.ProjectID, EnvironmentID: environment.ID,
			Hostname: environment.Hostname, Target: environment.Target,
			Active:            environment.State == environmentDomain.StateActive,
			Available:         environment.Availability == environmentDomain.AvailabilityAvailable,
			UnavailableReason: environment.UnavailableReason,
		})
	}
	return result, nil
}
