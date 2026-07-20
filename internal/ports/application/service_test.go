package application

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/ports/domain"
)

type sourceStub struct{ facts []domain.Fact }

func (s sourceStub) Facts(context.Context) ([]domain.Fact, error) { return s.facts, nil }

type countingSource struct{ calls atomic.Int32 }

func (s *countingSource) Facts(context.Context) ([]domain.Fact, error) {
	s.calls.Add(1)
	return []domain.Fact{{ID: "listener", Kind: domain.KindBinding, Port: 9000, Protocol: "tcp"}}, nil
}

type reservationStub struct{}

func (reservationStub) Reconcile(_ context.Context, declarations []domain.Fact, now time.Time) ([]domain.Fact, error) {
	result := make([]domain.Fact, 0, len(declarations))
	for _, item := range declarations {
		item.ID = "reservation_" + item.ID
		item.Kind, item.Source, item.ObservedAt = domain.KindReservation, "switchyard", now
		result = append(result, item)
	}
	return result, nil
}

func TestRegistryClassifiesStoppedReservationAgainstLiveProject(t *testing.T) {
	t.Parallel()
	declarations := sourceStub{facts: []domain.Fact{
		{ID: "decl-a", Kind: domain.KindDeclaration, ProjectID: "stopped", ProjectName: "Stopped", ServiceID: "web", PortID: "web", Port: 18081, Protocol: "tcp", Host: "127.0.0.1"},
	}}
	bindings := sourceStub{facts: []domain.Fact{
		{ID: "bind-b", Kind: domain.KindBinding, ProjectID: "running", ProjectName: "Running", ServiceID: "web", Port: 18081, Protocol: "tcp", Host: "127.0.0.1"},
	}}
	service := NewService(declarations, bindings, sourceStub{}, reservationStub{})
	registry, err := service.Registry(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(registry.Conflicts) != 2 {
		t.Fatalf("conflicts = %#v", registry.Conflicts)
	}
	if registry.Conflicts[0].Type != domain.ConflictDeclaredBound && registry.Conflicts[1].Type != domain.ConflictDeclaredBound {
		t.Fatalf("missing declared/bound conflict: %#v", registry.Conflicts)
	}
}

func TestRegistryCachesExpensiveListenerSnapshotButSuggestionRefreshesIt(t *testing.T) {
	t.Parallel()
	listeners := &countingSource{}
	service := NewService(sourceStub{}, sourceStub{}, listeners, reservationStub{})
	if _, err := service.Registry(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Registry(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := listeners.calls.Load(); got != 1 {
		t.Fatalf("cached listener scans = %d, want 1", got)
	}
	if _, err := service.Suggest(context.Background(), 9000, 9001, "tcp", "", nil); err != nil {
		t.Fatal(err)
	}
	if got := listeners.calls.Load(); got != 2 {
		t.Fatalf("listener scans after suggestion = %d, want 2", got)
	}
}

func TestClassifyUnknownListenerAndHostOverlap(t *testing.T) {
	t.Parallel()
	facts := []domain.Fact{
		{ID: "decl", Kind: domain.KindDeclaration, ProjectID: "one", Port: 9000, Protocol: "tcp", Host: "127.0.0.1"},
		{ID: "listener", Kind: domain.KindBinding, Port: 9000, Protocol: "tcp", Host: "0.0.0.0"},
	}
	conflicts := Classify(facts)
	if len(conflicts) != 1 || conflicts[0].Type != domain.ConflictUnknownBinding {
		t.Fatalf("conflicts = %#v", conflicts)
	}
}

func TestClassifyDoesNotTreatMultipleUnknownListenerOwnersAsManagedConflicts(t *testing.T) {
	t.Parallel()
	facts := []domain.Fact{
		{ID: "worker-one", Kind: domain.KindBinding, ProcessID: 101, Port: 9000, Protocol: "tcp", Host: "0.0.0.0"},
		{ID: "worker-two", Kind: domain.KindBinding, ProcessID: 102, Port: 9000, Protocol: "tcp", Host: "0.0.0.0"},
	}
	if conflicts := Classify(facts); len(conflicts) != 0 {
		t.Fatalf("conflicts = %#v", conflicts)
	}
}

func TestClassifyFindsSameProjectServicesCompetingForOneHostPort(t *testing.T) {
	t.Parallel()
	facts := []domain.Fact{
		{ID: "web", Kind: domain.KindDeclaration, ProjectID: "project", ServiceID: "web", PortID: "web", Port: 9000, Protocol: "tcp", Host: "127.0.0.1"},
		{ID: "api", Kind: domain.KindDeclaration, ProjectID: "project", ServiceID: "api", PortID: "api", Port: 9000, Protocol: "tcp", Host: "127.0.0.1"},
	}
	conflicts := Classify(facts)
	if len(conflicts) != 1 || conflicts[0].Type != domain.ConflictDeclaredDeclared {
		t.Fatalf("conflicts = %#v", conflicts)
	}
}

func TestClassifyTreatsOneManifestPortAndItsRuntimeBindingAsOneClaim(t *testing.T) {
	t.Parallel()
	facts := []domain.Fact{
		{ID: "declared", Kind: domain.KindDeclaration, ProjectID: "project", ServiceID: "web", PortID: "web", Port: 9000, Protocol: "tcp", Host: "0.0.0.0"},
		{ID: "reserved", Kind: domain.KindReservation, ProjectID: "project", ServiceID: "web", PortID: "web", Port: 9000, Protocol: "tcp", Host: "0.0.0.0"},
		{ID: "bound", Kind: domain.KindBinding, ProjectID: "project", ServiceID: "web", Port: 9000, Protocol: "tcp", Host: "127.0.0.1"},
	}
	if conflicts := Classify(facts); len(conflicts) != 0 {
		t.Fatalf("conflicts = %#v", conflicts)
	}
}

func TestSuggestionIncludesReservationsBindingsAndExclusions(t *testing.T) {
	t.Parallel()
	service := NewService(sourceStub{facts: []domain.Fact{{ID: "a", Kind: domain.KindDeclaration, ProjectID: "other", Port: 15000, Protocol: "tcp"}}},
		sourceStub{facts: []domain.Fact{{ID: "b", Kind: domain.KindBinding, Port: 15001, Protocol: "tcp"}}}, sourceStub{}, reservationStub{})
	suggestion, err := service.Suggest(context.Background(), 15000, 15003, "tcp", "project", []int{15002})
	if err != nil {
		t.Fatal(err)
	}
	if suggestion.Port != 15003 {
		t.Fatalf("port = %d", suggestion.Port)
	}
}
