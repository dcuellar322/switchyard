package application

import (
	"testing"

	"switchyard.dev/switchyard/internal/actions/domain"
)

func TestPrimaryEndpointActionSortsBeforeOtherBrowserActions(t *testing.T) {
	t.Parallel()
	actions := []domain.Definition{
		{ID: "open-backend", Type: "browser.open"},
		{ID: "open-frontend", Type: "browser.open"},
		{ID: "claude", Type: "agent.start"},
	}
	sortDefinitions(actions, "open-frontend")
	if actions[0].ID != "open-frontend" {
		t.Fatalf("first action = %q", actions[0].ID)
	}
}

func TestBuiltInTerminalActionPreservesExternalProviderChoice(t *testing.T) {
	t.Parallel()
	if _, available := builtInTerminalAction("integrated"); available {
		t.Fatal("integrated preference exposed an external terminal action")
	}
	system, available := builtInTerminalAction("system")
	if !available || system.Provider != "" {
		t.Fatalf("system action = %#v, available = %t", system, available)
	}
	iterm, available := builtInTerminalAction("iterm")
	if !available || iterm.Provider != "iterm" {
		t.Fatalf("iTerm action = %#v, available = %t", iterm, available)
	}
}
