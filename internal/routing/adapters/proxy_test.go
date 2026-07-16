package adapters

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"switchyard.dev/switchyard/internal/routing/domain"
)

func TestProxyForwardsActiveRouteToLoopbackHTTP(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/base/api" || request.URL.RawQuery != "page=1" {
			t.Errorf("upstream URL = %s", request.URL.String())
		}
		if request.Header.Get("X-Forwarded-Host") != "project.localhost" || request.Header.Get("X-Forwarded-Proto") != "http" {
			t.Errorf("forwarding headers = %#v", request.Header)
		}
		if request.Header.Get("X-Forwarded-For") == "203.0.113.9" {
			t.Error("proxy trusted a client-supplied forwarding header")
		}
		response.WriteHeader(http.StatusCreated)
		_, _ = response.Write([]byte("proxied"))
	}))
	t.Cleanup(upstream.Close)

	resolver := resolverStub{route: domain.Route{
		Hostname: "project.localhost", Status: domain.StatusActive,
		EnvironmentID: "env-one", Target: upstream.URL + "/base",
	}}
	request := httptest.NewRequest(http.MethodPost, "http://project.localhost/api?page=1", strings.NewReader("body"))
	request.Header.Set("X-Forwarded-For", "203.0.113.9")
	response := httptest.NewRecorder()
	NewProxy(resolver).ServeHTTP(response, request)
	if response.Code != http.StatusCreated || strings.TrimSpace(response.Body.String()) != "proxied" {
		t.Fatalf("response = %d %q", response.Code, response.Body.String())
	}
}

func TestProxyReturnsExplicitFailureStates(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		route  domain.Route
		status int
	}{
		{name: "disabled", route: domain.Route{Status: domain.StatusDisabled, Reason: "routing disabled"}, status: http.StatusServiceUnavailable},
		{name: "unavailable", route: domain.Route{Status: domain.StatusUnavailable, Reason: "not active"}, status: http.StatusServiceUnavailable},
		{name: "conflict", route: domain.Route{Status: domain.StatusConflict, Reason: "two active environments"}, status: http.StatusConflict},
		{name: "invalid target", route: domain.Route{Status: domain.StatusActive, Target: "https://127.0.0.1:8080"}, status: http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "http://project.localhost/", nil)
			response := httptest.NewRecorder()
			NewProxy(resolverStub{route: test.route}).ServeHTTP(response, request)
			result := response.Result()
			t.Cleanup(func() { _ = result.Body.Close() })
			body, _ := io.ReadAll(result.Body)
			if response.Code != test.status || len(body) == 0 || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("response = %d %q headers=%v", response.Code, body, response.Header())
			}
		})
	}
}

type resolverStub struct {
	route domain.Route
	err   error
}

func (r resolverStub) Resolve(context.Context, string) (domain.Route, error) { return r.route, r.err }
