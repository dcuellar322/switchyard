package application

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
	"switchyard.dev/switchyard/internal/runtime/domain"
)

const observationCoalescingWindow = time.Second

type observationCache struct {
	mu          sync.Mutex
	entries     map[string]cachedObservation
	generations map[string]uint64
	requests    singleflight.Group
}

type cachedObservation struct {
	observation domain.Observation
	expiresAt   time.Time
	generation  uint64
}

func newObservationCache() *observationCache {
	return &observationCache{
		entries:     make(map[string]cachedObservation),
		generations: make(map[string]uint64),
	}
}

func (c *observationCache) load(
	ctx context.Context,
	projectID string,
	inspect func() (domain.Observation, error),
) (domain.Observation, error) {
	if observation, ok := c.cached(projectID, time.Now()); ok {
		return cloneObservation(observation), nil
	}
	result, err, _ := c.requests.Do(projectID, func() (any, error) {
		if observation, ok := c.cached(projectID, time.Now()); ok {
			return observation, nil
		}
		if err := ctx.Err(); err != nil {
			return domain.Observation{}, err
		}
		generation := c.generation(projectID)
		observation, err := inspect()
		if err != nil {
			return domain.Observation{}, err
		}
		c.store(projectID, generation, observation, time.Now().Add(observationCoalescingWindow))
		return observation, nil
	})
	if err != nil {
		return domain.Observation{}, err
	}
	return cloneObservation(result.(domain.Observation)), nil
}

func (c *observationCache) cached(projectID string, now time.Time) (domain.Observation, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[projectID]
	if !ok || !now.Before(entry.expiresAt) || entry.generation != c.generations[projectID] {
		if ok {
			delete(c.entries, projectID)
		}
		return domain.Observation{}, false
	}
	return entry.observation, true
}

func (c *observationCache) generation(projectID string) uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.generations[projectID]
}

func (c *observationCache) store(
	projectID string,
	generation uint64,
	observation domain.Observation,
	expiresAt time.Time,
) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.generations[projectID] != generation {
		return
	}
	c.entries[projectID] = cachedObservation{
		observation: cloneObservation(observation),
		expiresAt:   expiresAt,
		generation:  generation,
	}
}

func (c *observationCache) invalidate(projectID string) {
	if projectID == "" {
		return
	}
	c.mu.Lock()
	c.generations[projectID]++
	delete(c.entries, projectID)
	c.mu.Unlock()
	c.requests.Forget(projectID)
}

func cloneObservation(observation domain.Observation) domain.Observation {
	result := observation
	result.AvailableProfiles = append([]string(nil), observation.AvailableProfiles...)
	if observation.Engine != nil {
		engine := *observation.Engine
		result.Engine = &engine
	}
	result.Services = make([]domain.ServiceObservation, len(observation.Services))
	for index, service := range observation.Services {
		result.Services[index] = service
		result.Services[index].Ports = append([]domain.PublishedPort(nil), service.Ports...)
		if service.Container != nil {
			container := *service.Container
			container.StartedAt = cloneTimePointer(service.Container.StartedAt)
			container.FinishedAt = cloneTimePointer(service.Container.FinishedAt)
			container.ExitCode = cloneIntPointer(service.Container.ExitCode)
			result.Services[index].Container = &container
		}
		if service.Process != nil {
			process := *service.Process
			process.StartedAt = cloneTimePointer(service.Process.StartedAt)
			process.FinishedAt = cloneTimePointer(service.Process.FinishedAt)
			process.ExitCode = cloneIntPointer(service.Process.ExitCode)
			result.Services[index].Process = &process
		}
	}
	return result
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneIntPointer(value *int) *int {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}
