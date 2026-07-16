package application

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestRegistrationCoordinatorAllocatesUniqueExactPortsAndPreservesThem(t *testing.T) {
	t.Parallel()
	repository := &repositoryStub{}
	service := newTestService(repository, []WorktreeObservation{
		{Path: "/repo/main", Branch: "main"}, {Path: "/repo/feature", Branch: "feature"},
	})
	allocator := &allocatorStub{next: 21000}
	coordinator := NewRegistrationCoordinator(service, declarationStub{items: []PortDeclaration{
		{ID: "api", Protocol: "tcp", TargetPort: 8080}, {ID: "debug", Protocol: "tcp", TargetPort: 4000},
	}}, allocator)
	first, err := coordinator.RegisterWorktrees(context.Background(), "project-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Environments) != 2 || allocator.calls != 4 {
		t.Fatalf("registration=%#v allocator calls=%d", first, allocator.calls)
	}
	hosts := make(map[int]string)
	for _, environment := range first.Environments {
		if environment.State != "inactive" || len(environment.Allocation.PortLeases) != 2 {
			t.Fatalf("environment = %#v", environment)
		}
		for _, lease := range environment.Allocation.PortLeases {
			if owner, exists := hosts[lease.HostPort]; exists {
				t.Fatalf("host port %d shared by %s and %s", lease.HostPort, owner, environment.ID)
			}
			hosts[lease.HostPort] = environment.ID
		}
	}
	allocator.next = 31000
	second, err := coordinator.RegisterWorktrees(context.Background(), "project-1")
	if err != nil {
		t.Fatal(err)
	}
	if allocator.calls != 4 {
		t.Fatalf("stable compatible leases triggered new allocations: %d", allocator.calls)
	}
	for index := range first.Environments {
		if !slices.Equal(first.Environments[index].Allocation.PortLeases, second.Environments[index].Allocation.PortLeases) {
			t.Fatalf("leases changed: %#v -> %#v", first.Environments[index].Allocation.PortLeases, second.Environments[index].Allocation.PortLeases)
		}
	}
}

func TestRegistrationCoordinatorDoesNotPersistPartialAllocationFailure(t *testing.T) {
	t.Parallel()
	repository := &repositoryStub{}
	service := newTestService(repository, []WorktreeObservation{{Path: "/repo/main", Branch: "main"}})
	coordinator := NewRegistrationCoordinator(service, declarationStub{items: []PortDeclaration{{ID: "api", Protocol: "tcp", TargetPort: 8080}}}, &allocatorStub{err: errors.New("no port")})
	if _, err := coordinator.RegisterWorktrees(context.Background(), "project-1"); err == nil {
		t.Fatal("RegisterWorktrees() succeeded")
	}
	items, err := repository.List(context.Background())
	if err != nil || len(items) != 1 || len(items[0].Allocation.PortLeases) != 0 {
		t.Fatalf("items=%#v error=%v", items, err)
	}
}

type declarationStub struct{ items []PortDeclaration }

func (s declarationStub) Declarations(context.Context, string) ([]PortDeclaration, error) {
	return slices.Clone(s.items), nil
}

type allocatorStub struct {
	next  int
	calls int
	err   error
}

func (a *allocatorStub) Suggest(_ context.Context, _, _ int, _ string, _ string, excluded []int) (int, error) {
	if a.err != nil {
		return 0, a.err
	}
	a.calls++
	for slices.Contains(excluded, a.next) {
		a.next++
	}
	result := a.next
	a.next++
	return result, nil
}
