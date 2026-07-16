// Package adapters supplies environment persistence and cross-domain adapters.
package adapters

import (
	"context"
	"sort"
	"sync"

	"switchyard.dev/switchyard/internal/environments/application"
	"switchyard.dev/switchyard/internal/environments/domain"
)

// ErrEnvironmentNotFound identifies an unknown environment record.
var ErrEnvironmentNotFound = application.ErrNotFound

// MemoryRepository is a constructor-injected, concurrency-safe registry useful
// until an installation opts into durable environment history.
type MemoryRepository struct {
	mu           sync.RWMutex
	environments map[string]domain.Environment
}

// Get returns one registered environment.
func (r *MemoryRepository) Get(ctx context.Context, environmentID string) (domain.Environment, error) {
	if err := ctx.Err(); err != nil {
		return domain.Environment{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	environment, exists := r.environments[environmentID]
	if !exists {
		return domain.Environment{}, ErrEnvironmentNotFound
	}
	return cloneEnvironment(environment), nil
}

// NewMemoryRepository creates an empty project environment registry.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{environments: make(map[string]domain.Environment)}
}

// Update replaces one existing environment atomically.
func (r *MemoryRepository) Update(ctx context.Context, environment domain.Environment) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.environments[environment.ID]; !exists {
		return ErrEnvironmentNotFound
	}
	r.environments[environment.ID] = cloneEnvironment(environment)
	return nil
}

// List returns a detached, stable snapshot.
func (r *MemoryRepository) List(ctx context.Context) ([]domain.Environment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]domain.Environment, 0, len(r.environments))
	for _, environment := range r.environments {
		result = append(result, cloneEnvironment(environment))
	}
	sort.Slice(result, func(left, right int) bool { return result[left].ID < result[right].ID })
	return result, nil
}

// ReplaceProject atomically replaces one project's observed environments.
func (r *MemoryRepository) ReplaceProject(ctx context.Context, projectID string, environments []domain.Environment) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, environment := range r.environments {
		if environment.ProjectID == projectID {
			delete(r.environments, id)
		}
	}
	for _, environment := range environments {
		r.environments[environment.ID] = cloneEnvironment(environment)
	}
	return nil
}

func cloneEnvironment(environment domain.Environment) domain.Environment {
	environment.Allocation.PortLeases = append([]domain.PortLease(nil), environment.Allocation.PortLeases...)
	return environment
}
