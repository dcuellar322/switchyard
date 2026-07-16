package application

import (
	"context"
	"fmt"
	"sort"

	"switchyard.dev/switchyard/internal/environments/domain"
)

const (
	defaultLeaseStart = 10000
	defaultLeaseEnd   = 60000
)

// PortDeclaration is the environment domain's bounded view of a project's
// accepted runtime ports. It contains no storage or Compose details.
type PortDeclaration struct {
	ID         string
	Protocol   string
	TargetPort int
}

// PortDeclarationSource reads accepted project port declarations.
type PortDeclarationSource interface {
	Declarations(context.Context, string) ([]PortDeclaration, error)
}

// PortAllocator selects one currently free, exact host port.
type PortAllocator interface {
	Suggest(context.Context, int, int, string, string, []int) (int, error)
}

// RegistrationCoordinator adds exact, collision-checked port leases to the
// deterministic worktree identities produced by Service.
type RegistrationCoordinator struct {
	environments *Service
	declarations PortDeclarationSource
	ports        PortAllocator
}

// NewRegistrationCoordinator creates the explicit registration workflow.
func NewRegistrationCoordinator(environments *Service, declarations PortDeclarationSource, ports PortAllocator) *RegistrationCoordinator {
	return &RegistrationCoordinator{environments: environments, declarations: declarations, ports: ports}
}

// RegisterWorktrees reconciles Git metadata, then allocates any missing exact
// leases. Existing compatible leases remain stable across reconciliation.
func (c *RegistrationCoordinator) RegisterWorktrees(ctx context.Context, projectID string) (Registration, error) {
	registration, err := c.environments.RegisterWorktrees(ctx, projectID)
	if err != nil {
		return Registration{}, err
	}
	declarations, err := c.declarations.Declarations(ctx, projectID)
	if err != nil {
		return Registration{}, fmt.Errorf("read environment port declarations: %w", err)
	}
	sort.Slice(declarations, func(left, right int) bool { return declarations[left].ID < declarations[right].ID })
	used := hostPorts(registration.Environments)
	for index := range registration.Environments {
		environment := registration.Environments[index]
		if environment.Availability != domain.AvailabilityAvailable {
			continue
		}
		leases, allocationErr := c.allocate(ctx, environment, declarations, used)
		if allocationErr != nil {
			return Registration{}, fmt.Errorf("allocate ports for environment %s: %w", environment.ID, allocationErr)
		}
		state := environment.State
		if state == domain.StateRegistered {
			state = domain.StateInactive
		}
		environment, allocationErr = c.environments.ConfigureRuntime(ctx, environment.ID, RuntimeConfiguration{
			State: state, Hostname: environment.Hostname, Target: environment.Target, PortLeases: leases,
		})
		if allocationErr != nil {
			return Registration{}, allocationErr
		}
		registration.Environments[index] = environment
	}
	return registration, nil
}

func (c *RegistrationCoordinator) allocate(
	ctx context.Context,
	environment domain.Environment,
	declarations []PortDeclaration,
	used []int,
) ([]domain.PortLease, error) {
	existing := make(map[string]domain.PortLease, len(environment.Allocation.PortLeases))
	for _, lease := range environment.Allocation.PortLeases {
		existing[lease.PortID] = lease
	}
	leases := make([]domain.PortLease, 0, len(declarations))
	for _, declaration := range declarations {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if declaration.ID == "" || declaration.TargetPort < 1 || declaration.TargetPort > 65535 || declaration.Protocol != "tcp" && declaration.Protocol != "udp" {
			return nil, fmt.Errorf("invalid port declaration %q", declaration.ID)
		}
		if lease, ok := existing[declaration.ID]; ok && lease.Protocol == declaration.Protocol && lease.TargetPort == declaration.TargetPort {
			leases = append(leases, lease)
			continue
		}
		start := defaultLeaseStart + environment.Allocation.PortOffset%(defaultLeaseEnd-defaultLeaseStart+1)
		hostPort, err := c.ports.Suggest(ctx, start, defaultLeaseEnd, declaration.Protocol, environment.ID, used)
		if err != nil && start == defaultLeaseStart {
			return nil, err
		}
		if err != nil {
			hostPort, err = c.ports.Suggest(ctx, defaultLeaseStart, start-1, declaration.Protocol, environment.ID, used)
		}
		if err != nil {
			return nil, err
		}
		used = append(used, hostPort)
		leases = append(leases, domain.PortLease{
			PortID: declaration.ID, Protocol: declaration.Protocol,
			TargetPort: declaration.TargetPort, HostPort: hostPort,
		})
	}
	return leases, nil
}

func hostPorts(environments []domain.Environment) []int {
	var result []int
	for _, environment := range environments {
		for _, lease := range environment.Allocation.PortLeases {
			result = append(result, lease.HostPort)
		}
	}
	return result
}
