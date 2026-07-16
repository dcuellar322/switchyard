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
	mu      sync.RWMutex
	owned   map[string]map[string]struct{}
	pending map[string]domain.Action
}

func newManagedContainers() *managedContainers {
	return &managedContainers{owned: make(map[string]map[string]struct{}), pending: make(map[string]domain.Action)}
}

func (m *managedContainers) RecordAction(project string, action domain.Action) {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch action {
	case domain.ActionStop, domain.ActionTeardown:
		delete(m.owned, project)
		delete(m.pending, project)
	case domain.ActionStart, domain.ActionRestart, domain.ActionPause, domain.ActionUnpause, domain.ActionRebuild:
		m.pending[project] = action
	}
}

func (m *managedContainers) Reconcile(project string, containerIDs []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.pending[project]; !ok {
		return
	}
	owned := make(map[string]struct{}, len(containerIDs))
	for _, id := range containerIDs {
		owned[id] = struct{}{}
	}
	m.owned[project] = owned
	delete(m.pending, project)
}

func (m *managedContainers) Owns(project, containerID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.owned[project][containerID]
	return ok
}
