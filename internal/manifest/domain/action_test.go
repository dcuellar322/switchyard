package domain

import (
	"strings"
	"testing"
)

func TestValidateActionsAcceptsTypedSafeDefinitions(t *testing.T) {
	t.Parallel()
	manifest := validActionManifest()
	manifest.Actions = []Action{
		{ID: "verify", Name: "Verify", Type: "tests.run", Command: []string{"go", "test", "./..."}, Risk: "read_only"},
		{ID: "codex", Name: "Codex", Type: "agent.start", Provider: "codex", Risk: "interactive"},
		{ID: "docs", Name: "Documentation", Type: "browser.open", Target: "http://127.0.0.1:8080", Risk: "interactive"},
	}
	if err := manifest.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateActionsRejectsIncompleteTypedDefinitions(t *testing.T) {
	t.Parallel()
	manifest := validActionManifest()
	manifest.Actions = []Action{
		{ID: "verify", Name: "Verify", Type: "tests.run"},
		{ID: "shell", Name: "Shell", Type: "command", Command: []string{"one", "two"}, Shell: true},
		{ID: "agent", Name: "Agent", Type: "agent.start"},
		{ID: "browser", Name: "Browser", Type: "browser.open"},
	}
	err := manifest.Validate()
	for _, expected := range []string{"requires an argument array", "requires exactly one command string", "requires a provider", "requires a target"} {
		if err == nil || !strings.Contains(err.Error(), expected) {
			t.Fatalf("Validate() error = %v, want %q", err, expected)
		}
	}
}

func validActionManifest() Manifest {
	return Manifest{
		SchemaVersion: SchemaVersion,
		Kind:          KindProject,
		Metadata:      Metadata{ID: "actions", Name: "Actions"},
		Repository:    Repository{Root: "."},
	}
}
