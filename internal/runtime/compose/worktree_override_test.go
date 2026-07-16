package compose

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

func TestPortOverrideYAMLUsesExactLeasesWithoutEnvironment(t *testing.T) {
	t.Parallel()
	project := worktreeComposeProject()
	project.Process = &domain.ProcessRuntime{Environment: map[string]string{"SECRET_TOKEN": "must-not-appear"}}

	contents, err := portOverrideYAML(project)
	if err != nil {
		t.Fatalf("portOverrideYAML() error = %v", err)
	}
	for _, expected := range []string{`"api":`, "ports: !override", `"127.0.0.1:18443:443/tcp"`, `"127.0.0.1:18080:8080/tcp"`} {
		if !strings.Contains(contents, expected) {
			t.Fatalf("overlay missing %q:\n%s", expected, contents)
		}
	}
	if strings.Contains(contents, "SECRET_TOKEN") || strings.Contains(contents, "must-not-appear") {
		t.Fatalf("overlay persisted environment data: %s", contents)
	}
}

func TestWritePortOverrideIsPrivateAndStable(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	project := worktreeComposeProject()
	first, err := writePortOverride(directory, project)
	if err != nil {
		t.Fatalf("writePortOverride(first) error = %v", err)
	}
	second, err := writePortOverride(directory, project)
	if err != nil || first != second {
		t.Fatalf("writePortOverride(second) = %q, %v; want %q", second, err, first)
	}
	info, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 || filepath.Dir(first) != directory {
		t.Fatalf("overlay mode/path = %o, %q", info.Mode().Perm(), first)
	}
}

func TestAppendComposeFilePrecedesProjectName(t *testing.T) {
	t.Parallel()
	arguments := appendComposeFile([]string{"compose", "--file", "/repo/compose.yaml", "--project-name", "feature"}, "/data/ports.yaml")
	joined := strings.Join(arguments, " ")
	if joined != "compose --file /repo/compose.yaml --file /data/ports.yaml --project-name feature" {
		t.Fatalf("arguments = %q", joined)
	}
}

func worktreeComposeProject() domain.ProjectRuntime {
	return domain.ProjectRuntime{
		ProjectID: "env-feature", ManifestHash: "manifest", Compose: &domain.ComposeRuntime{
			Files: []string{"compose.yaml"}, PortOverrides: map[string]int{"web": 18080, "tls": 18443},
		},
		Services: []domain.ServiceDeclaration{{ID: "backend", RuntimeName: "api"}},
		Ports: map[string]domain.PortDeclaration{
			"web": {ID: "web", Service: "backend", Target: 8080, Protocol: "tcp"},
			"tls": {ID: "tls", Service: "backend", Target: 443, Protocol: "tcp"},
		},
	}
}
