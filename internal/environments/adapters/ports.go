package adapters

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"switchyard.dev/switchyard/internal/environments/application"
	"switchyard.dev/switchyard/internal/environments/domain"
	portsApplication "switchyard.dev/switchyard/internal/ports/application"
	portsDomain "switchyard.dev/switchyard/internal/ports/domain"
)

// PortDeclarations adapts the accepted-manifest port registry to registration.
type PortDeclarations struct{ source portsApplication.FactSource }

// NewPortDeclarations creates a declaration source without exposing manifests.
func NewPortDeclarations(source portsApplication.FactSource) *PortDeclarations {
	return &PortDeclarations{source: source}
}

// Declarations returns only accepted declarations for the selected project.
func (s *PortDeclarations) Declarations(ctx context.Context, projectID string) ([]application.PortDeclaration, error) {
	facts, err := s.source.Facts(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]application.PortDeclaration, 0)
	for _, fact := range facts {
		if fact.ProjectID != projectID || fact.Kind != portsDomain.KindDeclaration {
			continue
		}
		target := fact.Target
		if target == 0 {
			target = fact.Port
		}
		result = append(result, application.PortDeclaration{ID: fact.PortID, Protocol: fact.Protocol, TargetPort: target})
	}
	return result, nil
}

// PortAllocator adapts the port registry's richer suggestion response.
type PortAllocator struct{ service *portsApplication.Service }

// NewPortAllocator creates an exact-host-port allocator.
func NewPortAllocator(service *portsApplication.Service) *PortAllocator {
	return &PortAllocator{service: service}
}

// Suggest returns only the allocated port needed by environment registration.
func (a *PortAllocator) Suggest(ctx context.Context, start, end int, protocol, projectID string, excluded []int) (int, error) {
	suggestion, err := a.service.Suggest(ctx, start, end, protocol, projectID, excluded)
	return suggestion.Port, err
}

// PortLeaseSource exposes exact environment leases as independent registry
// facts. They are not reconciled as base-project manifest reservations.
type PortLeaseSource struct {
	environments interface {
		List(context.Context) ([]domain.Environment, error)
	}
	now func() time.Time
}

// NewPortLeaseSource creates a current environment lease source.
func NewPortLeaseSource(environments interface {
	List(context.Context) ([]domain.Environment, error)
}) *PortLeaseSource {
	return &PortLeaseSource{environments: environments, now: time.Now}
}

// Facts returns one reservation fact per exact lease.
func (s *PortLeaseSource) Facts(ctx context.Context) ([]portsDomain.Fact, error) {
	environments, err := s.environments.List(ctx)
	if err != nil {
		return nil, err
	}
	var result []portsDomain.Fact
	for _, environment := range environments {
		for _, lease := range environment.Allocation.PortLeases {
			digest := sha256.Sum256([]byte(environment.ID + "\x00" + lease.PortID + "\x00" + lease.Protocol))
			result = append(result, portsDomain.Fact{
				ID: "envport_" + hex.EncodeToString(digest[:12]), Kind: portsDomain.KindReservation,
				ProjectID: environment.ID, ProjectName: environment.Name, PortID: lease.PortID,
				Host: "127.0.0.1", Port: lease.HostPort, Target: lease.TargetPort, Protocol: lease.Protocol,
				Source: "worktree-environment", Evidence: fmt.Sprintf("exact lease owned by %s", environment.Allocation.PortLeaseNamespace),
				ObservedAt: s.now().UTC(),
			})
		}
	}
	return result, nil
}
