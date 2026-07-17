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
