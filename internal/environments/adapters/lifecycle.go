package adapters

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"switchyard.dev/switchyard/internal/environments/application"
	"switchyard.dev/switchyard/internal/environments/domain"
	runtimeDomain "switchyard.dev/switchyard/internal/runtime/domain"
)

// RuntimeObserver reads a post-lifecycle environment observation.
type RuntimeObserver interface {
	Inspect(context.Context, string) (runtimeDomain.Observation, error)
}

// Lifecycle synchronizes durable environment routing state after runtime work.
type Lifecycle struct {
	environments *application.Service
	runtime      RuntimeObserver
	refresh      func(context.Context) error
}

// NewLifecycle creates a post-operation environment state synchronizer.
func NewLifecycle(environments *application.Service, runtime RuntimeObserver, refresh func(context.Context) error) *Lifecycle {
	return &Lifecycle{environments: environments, runtime: runtime, refresh: refresh}
}

// Started records an active route only after a matching leased TCP binding is observed.
func (l *Lifecycle) Started(ctx context.Context, environmentID string) error {
	if !strings.HasPrefix(environmentID, "env-") {
		return nil
	}
	environment, err := l.environments.Get(ctx, environmentID)
	if err != nil {
		return err
	}
	observation, err := l.runtime.Inspect(ctx, environmentID)
	if err != nil {
		return err
	}
	hostPort := firstObservedLease(environment, observation)
	state, target := domain.StateInactive, ""
	if hostPort != 0 {
		state = domain.StateActive
		target = fmt.Sprintf("http://127.0.0.1:%d", hostPort)
	}
	_, err = l.environments.ConfigureRuntime(ctx, environmentID, application.RuntimeConfiguration{
		State: state, Hostname: environment.Hostname, Target: target, PortLeases: environment.Allocation.PortLeases,
	})
	if err == nil && l.refresh != nil {
		err = l.refresh(ctx)
	}
	return err
}

// Stopped removes the active target while preserving identity and exact leases.
func (l *Lifecycle) Stopped(ctx context.Context, environmentID string) error {
	if !strings.HasPrefix(environmentID, "env-") {
		return nil
	}
	environment, err := l.environments.Get(ctx, environmentID)
	if err != nil {
		return err
	}
	_, err = l.environments.ConfigureRuntime(ctx, environmentID, application.RuntimeConfiguration{
		State: domain.StateInactive, Hostname: environment.Hostname, PortLeases: environment.Allocation.PortLeases,
	})
	if err == nil && l.refresh != nil {
		err = l.refresh(ctx)
	}
	return err
}

func firstObservedLease(environment domain.Environment, observation runtimeDomain.Observation) int {
	bound := make(map[int]struct{})
	for _, service := range observation.Services {
		for _, port := range service.Ports {
			if port.Protocol == "tcp" && port.HostPort > 0 {
				bound[port.HostPort] = struct{}{}
			}
		}
	}
	leases := append([]domain.PortLease(nil), environment.Allocation.PortLeases...)
	sort.Slice(leases, func(left, right int) bool { return leases[left].PortID < leases[right].PortID })
	for _, lease := range leases {
		if lease.Protocol == "tcp" {
			if _, exists := bound[lease.HostPort]; exists {
				return lease.HostPort
			}
		}
	}
	return 0
}
