package compose

import (
	"context"
	"fmt"
	"sync"

	"github.com/moby/moby/client"
	"switchyard.dev/switchyard/internal/runtime/domain"
)

const (
	labelProject = "com.docker.compose.project"
	labelService = "com.docker.compose.service"
	labelOneoff  = "com.docker.compose.oneoff"
	labelNumber  = "com.docker.compose.container-number"
)

// ErrEngineUnavailable identifies a disconnected or incompatible Docker Engine.
var ErrEngineUnavailable = domain.ErrRuntimeUnavailable

type engineClient interface {
	Ping(context.Context, client.PingOptions) (client.PingResult, error)
	ServerVersion(context.Context, client.ServerVersionOptions) (client.ServerVersionResult, error)
	ContainerList(context.Context, client.ContainerListOptions) (client.ContainerListResult, error)
	ContainerInspect(context.Context, string, client.ContainerInspectOptions) (client.ContainerInspectResult, error)
	ContainerLogs(context.Context, string, client.ContainerLogsOptions) (client.ContainerLogsResult, error)
	ContainerStats(context.Context, string, client.ContainerStatsOptions) (client.ContainerStatsResult, error)
	Events(context.Context, client.EventsListOptions) client.EventsResult
	Close() error
}

type engineFactory struct{}

type engineConnector interface {
	Connect(context.Context, dockerConnection) (engineClient, client.PingResult, client.ServerVersionResult, error)
}

func (engineFactory) Connect(ctx context.Context, connection dockerConnection) (engineClient, client.PingResult, client.ServerVersionResult, error) {
	engine, err := connection.newClient()
	if err != nil {
		return nil, client.PingResult{}, client.ServerVersionResult{}, fmt.Errorf("%w: %v", ErrEngineUnavailable, err)
	}
	ping, err := engine.Ping(ctx, client.PingOptions{NegotiateAPIVersion: true})
	if err != nil {
		_ = engine.Close()
		return nil, client.PingResult{}, client.ServerVersionResult{}, fmt.Errorf("%w for context %q: %v", ErrEngineUnavailable, connection.ContextName, err)
	}
	version, err := engine.ServerVersion(ctx, client.ServerVersionOptions{})
	if err != nil {
		_ = engine.Close()
		return nil, client.PingResult{}, client.ServerVersionResult{}, fmt.Errorf("%w for context %q: %v", ErrEngineUnavailable, connection.ContextName, err)
	}
	return engine, ping, version, nil
}

func projectFilters(projectName string) client.Filters {
	return client.Filters{}.Add("label", labelProject+"="+projectName)
}

type managedContainers struct {
	mu         sync.RWMutex
	owned      map[string]map[string]struct{}
	pending    map[string]ownershipIntent
	operations map[string]string
	actions    map[string]domain.Action
	next       uint64
}

type ownershipIntent struct {
	generation uint64
	ready      bool
}

type ownershipToken struct {
	generation uint64
	ready      bool
}

func newManagedContainers() *managedContainers {
	return &managedContainers{
		owned: make(map[string]map[string]struct{}), pending: make(map[string]ownershipIntent), operations: make(map[string]string), actions: make(map[string]domain.Action),
	}
}

func (m *managedContainers) RecordAction(project string, action domain.Action, operationID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.actions[project] = action
	switch action {
	case domain.ActionStop, domain.ActionTeardown:
		delete(m.owned, project)
		delete(m.pending, project)
		delete(m.operations, project)
	case domain.ActionStart, domain.ActionRestart, domain.ActionPause, domain.ActionUnpause, domain.ActionRebuild:
		m.next++
		m.pending[project] = ownershipIntent{generation: m.next}
		if operationID != "" {
			m.operations[project] = operationID
		}
	}
}

func (m *managedContainers) ExpectedStopped(project string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	action := m.actions[project]
	return action == domain.ActionStop || action == domain.ActionTeardown
}

func (m *managedContainers) CompletePending(project, operationID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	intent, ok := m.pending[project]
	if !ok || m.operations[project] != operationID {
		return
	}
	intent.ready = true
	m.pending[project] = intent
}

func (m *managedContainers) DiscardPending(project, operationID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.pending, project)
	if m.operations[project] == operationID {
		delete(m.operations, project)
	}
}

func (m *managedContainers) Operation(project string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.operations[project]
}

func (m *managedContainers) OwnershipToken(project string) ownershipToken {
	m.mu.RLock()
	defer m.mu.RUnlock()
	intent := m.pending[project]
	return ownershipToken(intent)
}

func (m *managedContainers) Reconcile(project string, containerIDs []string, expected int, token ownershipToken) {
	m.mu.Lock()
	defer m.mu.Unlock()
	intent, ok := m.pending[project]
	if !ok || !token.ready || intent.generation != token.generation || intent.ready != token.ready {
		return
	}
	if len(containerIDs) == 0 {
		return
	}
	owned := make(map[string]struct{}, len(containerIDs))
	for _, id := range containerIDs {
		owned[id] = struct{}{}
	}
	m.owned[project] = owned
	if expected <= 0 || len(owned) >= expected {
		delete(m.pending, project)
	}
}

func (m *managedContainers) Owns(project, containerID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.owned[project][containerID]
	return ok
}
