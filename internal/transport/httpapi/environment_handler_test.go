package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	environmentApplication "switchyard.dev/switchyard/internal/environments/application"
	environmentDomain "switchyard.dev/switchyard/internal/environments/domain"
	routingDomain "switchyard.dev/switchyard/internal/routing/domain"
)

func TestEnvironmentHTTPRegistrationConfigurationAndRoutes(t *testing.T) {
	t.Parallel()
	environment := httpTestEnvironment()
	environments := &environmentHTTPStub{environment: environment}
	routes := &routeHTTPStub{routes: []routingDomain.Route{{
		Hostname: environment.Hostname, Status: routingDomain.StatusUnavailable,
		CandidateEnvironmentIDs: []string{environment.ID}, UpdatedAt: time.Unix(10, 0).UTC(),
	}}}
	handler := NewIPC(Dependencies{
		Environments: environments,
		EnvironmentRegistration: registrationHTTPStub{registration: environmentApplication.Registration{
			ProjectID: environment.ProjectID, Environments: []environmentDomain.Environment{environment}, ObservedAt: time.Unix(10, 0).UTC(),
		}},
		Routes: routes, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	list := httptest.NewRecorder()
	handler.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/v1/projects/project-1/environments", nil))
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), environment.ID) {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}

	register := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/projects/project-1/environments", nil)
	request.Header.Set("Idempotency-Key", "request-12345678")
	handler.ServeHTTP(register, request)
	if register.Code != http.StatusOK || routes.refreshes != 1 {
		t.Fatalf("register status=%d refreshes=%d body=%s", register.Code, routes.refreshes, register.Body.String())
	}

	update := httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPatch, "/api/v1/environments/"+environment.ID, strings.NewReader(`{"hostname":"feature.localhost"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "request-22345678")
	handler.ServeHTTP(update, request)
	if update.Code != http.StatusOK || environments.environment.Hostname != "feature.localhost" || routes.refreshes != 2 {
		t.Fatalf("update status=%d env=%#v refreshes=%d body=%s", update.Code, environments.environment, routes.refreshes, update.Body.String())
	}

	routeResponse := httptest.NewRecorder()
	handler.ServeHTTP(routeResponse, httptest.NewRequest(http.MethodGet, "/api/v1/routes", nil))
	if routeResponse.Code != http.StatusOK {
		t.Fatalf("routes status=%d body=%s", routeResponse.Code, routeResponse.Body.String())
	}
	var payload []map[string]any
	if err := json.Unmarshal(routeResponse.Body.Bytes(), &payload); err != nil || len(payload) != 1 {
		t.Fatalf("routes payload=%v error=%v", payload, err)
	}
}

type environmentHTTPStub struct{ environment environmentDomain.Environment }

func (s *environmentHTTPStub) Get(context.Context, string) (environmentDomain.Environment, error) {
	return s.environment, nil
}
func (s *environmentHTTPStub) ListProject(context.Context, string) ([]environmentDomain.Environment, error) {
	return []environmentDomain.Environment{s.environment}, nil
}
func (s *environmentHTTPStub) ConfigureRuntime(_ context.Context, _ string, configuration environmentApplication.RuntimeConfiguration) (environmentDomain.Environment, error) {
	s.environment.State, s.environment.Hostname, s.environment.Target = configuration.State, configuration.Hostname, configuration.Target
	s.environment.Allocation.PortLeases = configuration.PortLeases
	return s.environment, nil
}

type registrationHTTPStub struct {
	registration environmentApplication.Registration
}

func (s registrationHTTPStub) RegisterWorktrees(context.Context, string) (environmentApplication.Registration, error) {
	return s.registration, nil
}

type routeHTTPStub struct {
	routes    []routingDomain.Route
	refreshes int
}

func (s *routeHTTPStub) Refresh(context.Context) ([]routingDomain.Route, error) {
	s.refreshes++
	return s.routes, nil
}
func (s *routeHTTPStub) Snapshot() []routingDomain.Route { return s.routes }

func httpTestEnvironment() environmentDomain.Environment {
	now := time.Unix(10, 0).UTC()
	return environmentDomain.Environment{
		ID: "env-0123456789", ProjectID: "project-1", Name: "main", Path: "/repo",
		Availability: environmentDomain.AvailabilityAvailable, State: environmentDomain.StateInactive, Hostname: "project.localhost",
		Allocation: environmentDomain.RuntimeAllocation{
			ComposeProjectName: "sy-project-0123456789", PortLeaseNamespace: "worktree:0123456789", PortOffset: 1,
			PortLeases: []environmentDomain.PortLease{{PortID: "web", Protocol: "tcp", TargetPort: 8080, HostPort: 22080}},
		},
		RegisteredAt: now, LastObservedAt: now, UpdatedAt: now,
	}
}
