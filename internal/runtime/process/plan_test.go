package process

import (
	"context"
	"strings"
	"testing"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

func TestPlanOrdersDependenciesAndRejectsImplicitShell(t *testing.T) {
	t.Parallel()
	project := processProject(t.TempDir(), []servicePlan{
		{service: domain.ServiceDeclaration{ID: "web", RuntimeName: "web", Dependencies: []string{"api"}}, definition: domain.ProcessDefinition{ID: "web", Command: []string{"npm", "run", "dev"}}},
		{service: domain.ServiceDeclaration{ID: "api", RuntimeName: "api"}, definition: domain.ProcessDefinition{ID: "api", Command: []string{"uv", "run", "api.py"}}},
	})
	plan, err := buildPlan(domain.PlanRequest{Project: project, Action: domain.ActionStart})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Commands) != 2 || plan.Commands[0].Executable != "uv" || plan.Commands[1].Executable != "npm" {
		t.Fatalf("commands = %#v", plan.Commands)
	}
	project.Process.Processes[0].Command = []string{"sh", "-c", "echo unsafe"}
	if _, err := buildPlan(domain.PlanRequest{Project: project, Action: domain.ActionStart}); err == nil || !strings.Contains(err.Error(), "shell opt-in") {
		t.Fatalf("implicit shell error = %v", err)
	}
}

func TestEnvironmentOverlayResolvesOnlyWinningKeychainReferences(t *testing.T) {
	t.Parallel()
	resolver := &secretResolverFake{values: map[string]string{"service-token": "secret-value"}}
	driver := newDriver(context.Background(), newMemoryRunStore(), inspectorFake{}, resolver)
	values, err := driver.resolveEnvironment(context.Background(), &domain.ProcessRuntime{
		Environment: map[string]string{"PLAIN": "project"},
		Secrets: map[string]domain.SecretReference{
			"TOKEN": {Provider: "keychain", Key: "project-token"},
		},
	}, domain.ProcessDefinition{
		Environment: map[string]string{"PLAIN": "service", "TOKEN": "not-a-secret-reference"},
		Secrets: map[string]domain.SecretReference{
			"SERVICE_TOKEN": {Provider: "keychain", Key: "service-token"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(values, "\n")
	for _, wanted := range []string{"PLAIN=service", "TOKEN=not-a-secret-reference", "SERVICE_TOKEN=secret-value"} {
		if !strings.Contains(joined, wanted) {
			t.Fatalf("environment missing %q", wanted)
		}
	}
	if len(resolver.keys) != 1 || resolver.keys[0] != "service-token" {
		t.Fatalf("resolved keys = %v", resolver.keys)
	}
}

func processProject(root string, services []servicePlan) domain.ProjectRuntime {
	project := domain.ProjectRuntime{
		ProjectID: "project-process", ProjectSlug: "process", Root: root, Kind: domain.KindProcess,
		Process: &domain.ProcessRuntime{},
	}
	for _, service := range services {
		project.Services = append(project.Services, service.service)
		project.Process.Processes = append(project.Process.Processes, service.definition)
	}
	return project
}
