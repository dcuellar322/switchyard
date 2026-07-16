package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/routing/domain"
)

func TestRoutingRegistryReportsActiveUnavailableAndConflict(t *testing.T) {
	t.Parallel()

	service := NewService(true)
	service.now = func() time.Time { return time.Unix(20, 0) }
	routes, err := service.Reconcile(context.Background(), []domain.Candidate{
		{ProjectID: "one", EnvironmentID: "env-one", Hostname: "project.localhost", Target: "http://127.0.0.1:18080", Active: true, Available: true},
		{ProjectID: "two", EnvironmentID: "env-two", Hostname: "inactive.localhost", Active: false, Available: true},
		{ProjectID: "three", EnvironmentID: "env-three", Hostname: "broken.localhost", Target: "https://127.0.0.1:18081", Active: true, Available: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 3 {
		t.Fatalf("routes = %#v", routes)
	}
	assertRouteStatus(t, service, "project.localhost", domain.StatusActive)
	assertRouteStatus(t, service, "inactive.localhost", domain.StatusUnavailable)
	assertRouteStatus(t, service, "broken.localhost", domain.StatusUnavailable)

	_, err = service.Reconcile(context.Background(), []domain.Candidate{
		{ProjectID: "one", EnvironmentID: "env-one", Hostname: "project.localhost", Target: "http://127.0.0.1:18080", Active: true, Available: true},
		{ProjectID: "two", EnvironmentID: "env-two", Hostname: "project.localhost", Target: "http://127.0.0.1:18081", Active: true, Available: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	conflict, err := service.Resolve(context.Background(), "project.localhost:8080")
	if err != nil {
		t.Fatal(err)
	}
	if conflict.Status != domain.StatusConflict || len(conflict.CandidateEnvironmentIDs) != 2 {
		t.Fatalf("conflict = %#v", conflict)
	}
}

func TestRoutingRegistryIsOptionalAndAtomic(t *testing.T) {
	t.Parallel()

	service := NewService(false)
	if _, err := service.Reconcile(context.Background(), []domain.Candidate{{
		ProjectID: "one", EnvironmentID: "env-one", Hostname: "project.localhost",
		Target: "http://127.0.0.1:18080", Active: true, Available: true,
	}}); err != nil {
		t.Fatal(err)
	}
	assertRouteStatus(t, service, "project.localhost", domain.StatusDisabled)
	service.SetEnabled(true)
	assertRouteStatus(t, service, "project.localhost", domain.StatusActive)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.Reconcile(ctx, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("Reconcile() error = %v", err)
	}
	assertRouteStatus(t, service, "project.localhost", domain.StatusActive)
	unknown, err := service.Resolve(context.Background(), "unknown.localhost")
	if err != nil || unknown.Status != domain.StatusUnavailable {
		t.Fatalf("unknown=%#v error=%v", unknown, err)
	}
}

func TestRoutingRegistryRejectsInvalidClaims(t *testing.T) {
	t.Parallel()

	service := NewService(true)
	for _, candidate := range []domain.Candidate{
		{EnvironmentID: "env", Hostname: "project.localhost"},
		{ProjectID: "project", EnvironmentID: "env", Hostname: "project.example.com"},
	} {
		if _, err := service.Reconcile(context.Background(), []domain.Candidate{candidate}); err == nil {
			t.Fatalf("Reconcile(%#v) succeeded", candidate)
		}
	}
}

func assertRouteStatus(t *testing.T, service *Service, host string, expected domain.Status) {
	t.Helper()
	route, err := service.Resolve(context.Background(), host)
	if err != nil {
		t.Fatal(err)
	}
	if route.Status != expected {
		t.Fatalf("route %s status = %s, want %s (%#v)", host, route.Status, expected, route)
	}
}
