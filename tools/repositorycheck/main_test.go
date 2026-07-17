package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanSecretsRejectsCredentialAndAllowsMarkedFixture(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.env")
	contents := "TOKEN=ghp_abcdefghijklmnopqrstuvwxyz1234567890\nFIXTURE=sk-fixture-secret-never-send\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	findings, err := scanSecrets(path)
	if err != nil {
		t.Fatalf("scanSecrets() error = %v", err)
	}
	if len(findings) != 1 || findings[0].line != 1 {
		t.Fatalf("scanSecrets() findings = %#v", findings)
	}
}

func TestInspectGovernanceReportsMissingRoadmapInventory(t *testing.T) {
	t.Parallel()
	findings := inspectGovernance(t.TempDir())
	if len(findings) < 60 {
		t.Fatalf("inspectGovernance() findings = %d, want complete missing inventory", len(findings))
	}
	for _, item := range findings {
		if item.reason != "required roadmap or governance artifact is missing" {
			t.Fatalf("unexpected finding: %s", item)
		}
	}
}
