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

func TestComposeDiscoveryExcludesProfilesAndPrefersFrontendEndpoint(t *testing.T) {
	t.Parallel()
	path := t.TempDir()
	compose := `services:
  backend:
    image: example/backend
    ports:
      - "8000:8000"
  frontend:
    image: example/frontend
    ports:
      - "8080:5173"
      - "5173:5173"
  marketing:
    image: example/marketing
    profiles: [marketing]
    ports:
      - "8081:8081"
`
	if err := os.WriteFile(filepath.Join(path, "compose.yaml"), []byte(compose), 0o600); err != nil {
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
	proposal := application.BuildProposal(root, "project_profiles", "proposal_profiles", items)
	if got := serviceIDs(proposal.Candidate.Services); !slices.Equal(got, []string{"backend", "frontend"}) {
		t.Fatalf("default services = %v", got)
	}
	if len(proposal.Candidate.Ports) != 3 {
		t.Fatalf("default ports = %#v", proposal.Candidate.Ports)
	}
	if got := proposal.Candidate.Runtime.Compose.Profiles; !slices.Equal(got, []string{"marketing"}) {
		t.Fatalf("compose profiles = %v", got)
	}
	primary := ""
	for _, endpoint := range proposal.Candidate.Endpoints {
		if endpoint.Primary {
			primary = endpoint.ID
		}
	}
	if primary != "frontend" {
		t.Fatalf("primary endpoint = %q, endpoints = %#v", primary, proposal.Candidate.Endpoints)
	}
}

func TestComposeDiscoverySupportsDevelopmentFilenameAndDefaultedPorts(t *testing.T) {
	t.Parallel()
	path := t.TempDir()
	compose := `services:
  api:
    image: example/api
    ports:
      - "127.0.0.1:${API_PORT:-18000}:8000"
      - target: 5173
        published: ${WEB_PORT-15173}
  worker:
    image: example/worker
    ports:
      - "${WORKER_PORT}:9000"
`
	if err := os.WriteFile(filepath.Join(path, "docker-compose.local.yml"), []byte(compose), 0o600); err != nil {
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
	proposal := application.BuildProposal(root, "project_compose", "proposal_compose", items)
	if proposal.Candidate.Runtime.Compose == nil || !slices.Equal(proposal.Candidate.Runtime.Compose.Files, []string{"docker-compose.local.yml"}) {
		t.Fatalf("compose runtime = %#v", proposal.Candidate.Runtime)
	}
	if len(proposal.Candidate.Ports) != 2 || proposal.Candidate.Ports[0].Host != 18000 || proposal.Candidate.Ports[1].Host != 15173 {
		t.Fatalf("ports = %#v", proposal.Candidate.Ports)
	}
	var fallbackWarning, unresolvedWarning bool
	for _, item := range items {
		if item.Kind == "compose.project" && len(item.Warnings) > 0 {
			fallbackWarning = true
		}
		if item.Kind == "compose.port.unresolved" && item.Location.StartLine == 11 && len(item.Warnings) > 0 {
			unresolvedWarning = true
		}
	}
	if !fallbackWarning || !unresolvedWarning {
		t.Fatalf("expected fallback and unresolved-port warnings: %#v", items)
	}
}

func TestNodeDiscoveryInfersReviewedProcessAndLockfileManager(t *testing.T) {
	t.Parallel()
	path := t.TempDir()
	packageJSON := `{"name":"site","scripts":{"dev":"astro dev","build":"astro build"}}`
	if err := os.WriteFile(filepath.Join(path, "package.json"), []byte(packageJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "pnpm-lock.yaml"), []byte("lockfileVersion: '9.0'\n"), 0o600); err != nil {
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
	proposal := application.BuildProposal(root, "project_node", "proposal_node", items)
	if proposal.Candidate.Runtime.Process == nil || len(proposal.Candidate.Runtime.Process.Processes) != 1 {
		t.Fatalf("process runtime = %#v", proposal.Candidate.Runtime)
	}
	if got := proposal.Candidate.Runtime.Process.Processes[0].Command; !slices.Equal(got, []string{"pnpm", "run", "dev"}) {
		t.Fatalf("command = %v", got)
	}
	if len(proposal.Unresolved) != 0 || !slices.Contains(proposal.Candidate.Metadata.Tags, "pnpm") {
		t.Fatalf("proposal = %#v", proposal)
	}
}

func TestMixedPythonAndNodeRuntimeRemainsUnresolved(t *testing.T) {
	t.Parallel()
	path := t.TempDir()
	if err := os.WriteFile(filepath.Join(path, "package.json"), []byte(`{"name":"mixed","scripts":{"dev":"vite"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "pyproject.toml"), []byte("[project]\nname = \"mixed-worker\"\n"), 0o600); err != nil {
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
	proposal := application.BuildProposal(root, "project_mixed", "proposal_mixed", items)
	if proposal.Candidate.Runtime.Driver != "" || !slices.Contains(proposal.Unresolved, "/runtime/driver") {
		t.Fatalf("ambiguous runtime was inferred: %#v", proposal.Candidate.Runtime)
	}
}

func TestDocumentedUVicornCommandInfersProcessAndRejectsShellSyntax(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, command string
		supported     bool
	}{
		{"shell free", "uv run uvicorn app.main:app --reload --port 8123", true},
		{"shell operator", "uv run uvicorn app.main:app --reload; touch /tmp/not-allowed", false},
		{"unsafe path option", "uv run uvicorn app.main:app --app-dir ../../outside", false},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			path := t.TempDir()
			if err := os.WriteFile(filepath.Join(path, "pyproject.toml"), []byte("[project]\nname = \"api\"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(path, "uv.lock"), []byte("version = 1\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			readme := "# API\n\n```bash\n" + test.command + "\n```\n"
			if err := os.WriteFile(filepath.Join(path, "README.md"), []byte(readme), 0o600); err != nil {
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
			proposal := application.BuildProposal(root, "project_python", "proposal_python", items)
			if test.supported {
				if proposal.Candidate.Runtime.Process == nil || len(proposal.Candidate.Ports) != 1 || proposal.Candidate.Ports[0].Host != 8123 {
					t.Fatalf("supported proposal = %#v", proposal)
				}
				if len(proposal.Unresolved) != 0 {
					t.Fatalf("unresolved = %v", proposal.Unresolved)
				}
			} else if proposal.Candidate.Runtime.Driver != "" || !slices.Contains(proposal.Unresolved, "/runtime/driver") {
				t.Fatalf("unsafe README command was inferred: %#v", proposal.Candidate.Runtime)
			}
		})
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
