package adapters_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"switchyard.dev/switchyard/internal/discovery/adapters"
	"switchyard.dev/switchyard/internal/discovery/application"
	manifestApplication "switchyard.dev/switchyard/internal/manifest/application"
	manifest "switchyard.dev/switchyard/internal/manifest/domain"
)

func TestMixedFixtureProducesEvidenceBackedProposalWithoutSecrets(t *testing.T) {
	t.Parallel()
	fixture, err := filepath.Abs("../../../test/fixtures/mixed-project")
	if err != nil {
		t.Fatal(err)
	}
	root, err := application.SelectRoot(fixture)
	if err != nil {
		t.Fatal(err)
	}
	items, err := application.ScanAll(context.Background(), root, adapters.Defaults())
	if err != nil {
		t.Fatalf("ScanAll() error = %v", err)
	}
	proposal := application.BuildProposal(root, "project_test", "proposal_test", items)
	validation := manifestApplication.Validate(root.Path, proposal.Candidate)
	if !validation.Valid {
		t.Fatalf("candidate validation = %#v", validation)
	}

	if proposal.Candidate.Runtime.Driver != "compose" {
		t.Fatalf("driver = %q", proposal.Candidate.Runtime.Driver)
	}
	if got := serviceIDs(proposal.Candidate.Services); !slices.Equal(got, []string{"api", "web"}) {
		t.Fatalf("services = %v", got)
	}
	if len(proposal.Candidate.Ports) != 2 || proposal.Candidate.Ports[0].Host != 18082 || proposal.Candidate.Ports[1].Host != 15174 {
		t.Fatalf("ports = %#v", proposal.Candidate.Ports)
	}
	commands := actionCommands(proposal.Candidate.Actions)
	for _, expected := range []string{"uv run pytest", "npm run dev", "npm run test", "make test", "just check"} {
		if !slices.Contains(commands, expected) {
			t.Errorf("commands missing %q: %v", expected, commands)
		}
	}
	for _, item := range items {
		if item.SourcePath == "" || item.Location.StartLine < 1 || item.Location.EndLine < item.Location.StartLine {
			t.Fatalf("imprecise evidence: %#v", item)
		}
	}
	encoded, err := json.Marshal(proposal)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"switchyard-secret-canary-never-return", "sk-fixture-secret-never-return"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("proposal leaked secret %q", secret)
		}
	}
	if _, err := os.Stat(filepath.Join(fixture, ".env")); err != nil {
		t.Fatalf("secret canary fixture missing: %v", err)
	}
}

func TestReadmeScannerRedactsCredentialLikeTitle(t *testing.T) {
	t.Parallel()
	path := t.TempDir()
	if err := os.WriteFile(filepath.Join(path, "README.md"), []byte("# token=very-secret-token-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := application.SelectRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	items, err := application.ScanAll(context.Background(), root, adapters.Defaults())
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(items)
	if strings.Contains(string(encoded), "very-secret-token-value") || !strings.Contains(string(encoded), "[redacted]") {
		t.Fatalf("README evidence was not redacted: %s", encoded)
	}
}

func TestPortableProcessManifestWinsAsReviewedProposal(t *testing.T) {
	t.Parallel()
	fixture, err := filepath.Abs("../../../test/fixtures/uv-single-process")
	if err != nil {
		t.Fatal(err)
	}
	root, err := application.SelectRoot(fixture)
	if err != nil {
		t.Fatal(err)
	}
	items, err := application.ScanAll(context.Background(), root, adapters.Defaults())
	if err != nil {
		t.Fatal(err)
	}
	proposal := application.BuildProposal(root, "project_process", "proposal_process", items)
	if proposal.Candidate.Runtime.Driver != "process" || proposal.Candidate.Runtime.Process == nil {
		t.Fatalf("candidate runtime = %#v", proposal.Candidate.Runtime)
	}
	if len(proposal.Unresolved) != 0 || len(proposal.Candidate.Services) != 1 {
		t.Fatalf("proposal = %#v", proposal)
	}
	if proposal.ConfidenceByField["/"] != 1 {
		t.Fatalf("confidence = %#v", proposal.ConfidenceByField)
	}
}

func serviceIDs(services []manifest.Service) []string {
	result := make([]string, 0, len(services))
	for _, service := range services {
		result = append(result, service.ID)
	}
	return result
}

func actionCommands(actions []manifest.Action) []string {
	result := make([]string, 0, len(actions))
	for _, action := range actions {
		result = append(result, strings.Join(action.Command, " "))
	}
	return result
}
