package fixtures_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	discoveryAdapters "switchyard.dev/switchyard/internal/discovery/adapters"
	discovery "switchyard.dev/switchyard/internal/discovery/application"
	discoveryDomain "switchyard.dev/switchyard/internal/discovery/domain"
	manifest "switchyard.dev/switchyard/internal/manifest/application"
)

var requiredScenarios = []string{
	"compose-healthy",
	"compose-degraded",
	"compose-port-conflict",
	"uv-single-process",
	"node-single-process",
	"mixed-compose-and-process",
	"monorepo-two-apps",
	"external-process",
	"worktree-project",
	"malicious-readme",
	"secret-redaction",
}

func TestImplementationPlanFixtureInventory(t *testing.T) {
	t.Parallel()
	for _, scenario := range requiredScenarios {
		info, err := os.Stat(scenario)
		if err != nil {
			t.Errorf("required fixture %q: %v", scenario, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("required fixture %q is not a directory", scenario)
		}
	}
}

func TestPortableFixtureManifestsAreStrictAndValid(t *testing.T) {
	t.Parallel()
	paths, err := filepath.Glob(filepath.Join("*", ".switchyard", "project.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("portable fixture manifest inventory is empty")
	}
	for _, path := range paths {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			contents, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			document, parseErr := manifest.ParseYAML(contents)
			if parseErr != nil {
				t.Fatalf("strict parse: %v", parseErr)
			}
			root, absoluteErr := filepath.Abs(filepath.Dir(filepath.Dir(path)))
			if absoluteErr != nil {
				t.Fatal(absoluteErr)
			}
			result := manifest.Validate(root, document)
			if !result.Valid {
				t.Fatalf("validation: %#v", result.Errors)
			}
		})
	}
}

func TestComposeScenarioFactsRemainDistinct(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		services []string
		ports    []int
	}{
		{name: "compose-healthy", services: []string{"web"}},
		{name: "compose-degraded", services: []string{"web"}},
		{name: "compose-port-conflict", services: []string{"first", "second"}, ports: []int{19870, 19870}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			proposal := scanFixture(t, test.name)
			if proposal.Candidate.Runtime.Driver != "compose" {
				t.Fatalf("driver = %q", proposal.Candidate.Runtime.Driver)
			}
			services := make([]string, 0, len(proposal.Candidate.Services))
			for _, service := range proposal.Candidate.Services {
				services = append(services, service.ID)
			}
			if !slices.Equal(services, test.services) {
				t.Fatalf("services = %v, want %v", services, test.services)
			}
			ports := make([]int, 0, len(proposal.Candidate.Ports))
			for _, port := range proposal.Candidate.Ports {
				ports = append(ports, port.Host)
			}
			if !slices.Equal(ports, test.ports) {
				t.Fatalf("ports = %v, want %v", ports, test.ports)
			}
		})
	}
}

func TestAdversarialFixturesRemainInertAndSecretFree(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"malicious-readme", "secret-redaction"} {
		proposal := scanFixture(t, name)
		encoded, err := json.Marshal(proposal)
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{
			"fixture-malicious-readme-secret-never-return",
			"fixture-password-never-return",
			"sk-fixture-redaction-secret-never-return",
		} {
			if strings.Contains(string(encoded), secret) {
				t.Fatalf("%s proposal leaked fixture secret %q", name, secret)
			}
		}
	}
	if _, err := os.Stat("/tmp/switchyard-malicious-readme-executed"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("adversarial README sentinel exists: %v", err)
	}
}

func scanFixture(t *testing.T, name string) discoveryDomain.Proposal {
	t.Helper()
	rootPath, err := filepath.Abs(name)
	if err != nil {
		t.Fatal(err)
	}
	root, err := discovery.SelectRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	items, err := discovery.ScanAll(context.Background(), root, discoveryAdapters.Defaults())
	if err != nil {
		t.Fatal(err)
	}
	return discovery.BuildProposal(root, "project_"+strings.ReplaceAll(name, "-", "_"), "proposal_fixture", items)
}
