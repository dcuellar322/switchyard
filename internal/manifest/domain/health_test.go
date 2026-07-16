package domain

import (
	"strings"
	"testing"
)

func TestValidateServicesAcceptsAllHealthCheckTypes(t *testing.T) {
	t.Parallel()
	services := []Service{{
		ID: "api", Source: ServiceSource{ComposeService: "api"},
		HealthChecks: []HealthCheck{
			{ID: "http", Type: "http", URL: "http://127.0.0.1:8080/ready"},
			{ID: "tcp", Type: "tcp", Address: "127.0.0.1:8080"},
			{ID: "process", Type: "process"},
			{ID: "docker", Type: "docker"},
			{ID: "command", Type: "command", Command: []string{"true"}},
			{ID: "ready", Type: "composite", Mode: "all", Members: []string{"http", "tcp"}},
		},
	}}
	if problems := validateServices(services); len(problems) != 0 {
		t.Fatalf("problems = %v", problems)
	}
}

func TestValidateServicesRejectsIncompleteAndCyclicComposites(t *testing.T) {
	t.Parallel()
	services := []Service{{
		ID: "api", Source: ServiceSource{Process: "api"},
		HealthChecks: []HealthCheck{
			{ID: "tcp", Type: "tcp"},
			{ID: "one", Type: "composite", Members: []string{"two"}},
			{ID: "two", Type: "composite", Members: []string{"one"}},
		},
	}}
	problems := validateServices(services)
	joined := make([]string, 0, len(problems))
	for _, problem := range problems {
		joined = append(joined, problem.Error())
	}
	message := strings.Join(joined, "\n")
	if !strings.Contains(message, "TCP health check requires an address") || !strings.Contains(message, "contain a cycle") {
		t.Fatalf("problems = %s", message)
	}
}
