package adapters

import (
	"context"

	"switchyard.dev/switchyard/internal/routing/application"
	"switchyard.dev/switchyard/internal/routing/domain"
)

// Registry binds the routing application service to its environment source so
// callers cannot accidentally reconcile a different domain's candidates.
type Registry struct {
	service *application.Service
	source  *EnvironmentSource
}

// NewRegistry creates the environment-backed local route registry.
func NewRegistry(service *application.Service, source *EnvironmentSource) *Registry {
	return &Registry{service: service, source: source}
}

// Refresh rebuilds current routes from durable project environments.
func (r *Registry) Refresh(ctx context.Context) ([]domain.Route, error) {
	return r.service.Refresh(ctx, r.source)
}

// Snapshot returns the last atomically reconciled routes.
func (r *Registry) Snapshot() []domain.Route { return r.service.Snapshot() }

// Resolve delegates safe hostname resolution to the registry.
func (r *Registry) Resolve(ctx context.Context, host string) (domain.Route, error) {
	return r.service.Resolve(ctx, host)
}

// Enabled reports whether the optional proxy listener is configured.
func (r *Registry) Enabled() bool { return r.service.Enabled() }
