package adapters

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	observability "switchyard.dev/switchyard/internal/observability/domain"
	runtime "switchyard.dev/switchyard/internal/runtime/domain"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestHealthEvaluatorHTTPJSONAndPortExpansion(t *testing.T) {
	t.Parallel()
	evaluator := NewHealthEvaluator()
	evaluator.client.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "http://127.0.0.1:19616/ready" {
			t.Fatalf("URL = %s", request.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"status":{"ready":true}}`)), Header: http.Header{}}, nil
	})
	result := evaluator.Evaluate(context.Background(), runtime.ProjectRuntime{Ports: map[string]runtime.PortDeclaration{
		"api": {ID: "api", Host: 19616},
	}}, runtime.Observation{}, runtime.HealthCheckDefinition{
		Type: "http", URL: "http://127.0.0.1:${ports.api}/ready", ExpectedStatus: 200, JSONPath: "$.status.ready", ExpectedValue: "true",
	})
	if result.Status != observability.StatusHealthy {
		t.Fatalf("result = %#v", result)
	}
}

func TestHealthEvaluatorRejectsNonLoopbackTargets(t *testing.T) {
	t.Parallel()
	evaluator := NewHealthEvaluator()
	for _, check := range []runtime.HealthCheckDefinition{{Type: "http", URL: "https://example.com"}, {Type: "tcp", Address: "example.com:443"}} {
		result := evaluator.Evaluate(context.Background(), runtime.ProjectRuntime{}, runtime.Observation{}, check)
		if result.Status != observability.StatusUnhealthy || !strings.Contains(result.Message, "loopback") {
			t.Fatalf("result = %#v", result)
		}
	}
}

func TestHealthEvaluatorUsesRuntimeEvidence(t *testing.T) {
	t.Parallel()
	evaluator := NewHealthEvaluator()
	observation := runtime.Observation{Services: []runtime.ServiceObservation{{ID: "api", State: "running", Health: "healthy"}}}
	for _, kind := range []string{"process", "docker"} {
		result := evaluator.Evaluate(context.Background(), runtime.ProjectRuntime{}, observation, runtime.HealthCheckDefinition{Type: kind, ServiceID: "api"})
		if result.Status != observability.StatusHealthy {
			t.Fatalf("%s result = %#v", kind, result)
		}
	}
}
