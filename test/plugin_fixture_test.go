package test_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"switchyard.dev/switchyard/internal/platform/sqlite"
	pluginsAdapters "switchyard.dev/switchyard/internal/plugins/adapters"
	pluginsApplication "switchyard.dev/switchyard/internal/plugins/application"
	pluginsdk "switchyard.dev/switchyard/sdk/plugin"
)

type trustedFixtureProject struct{ root string }

func (p trustedFixtureProject) Project(context.Context, string) (pluginsApplication.Project, error) {
	return pluginsApplication.Project{ID: "fixture-project", DisplayName: "Node fixture", Root: p.root, Trusted: true}, nil
}

func TestSampleExternalPluginInspectsAndOperatesFixture(t *testing.T) {
	root := repositoryRoot(t)
	data := t.TempDir()
	packageDir := filepath.Join(data, "plugins", "fixture-inspector")
	if err := os.MkdirAll(packageDir, 0o700); err != nil {
		t.Fatal(err)
	}
	executableName := "switchyard-fixture-plugin"
	if runtime.GOOS == "windows" {
		executableName += ".exe"
	}
	executable := filepath.Join(packageDir, executableName)
	command := exec.CommandContext(t.Context(), "go", "build", "-trimpath", "-o", executable, "./examples/plugins/fixture")
	command.Dir = root
	command.Env = os.Environ()
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build sample plugin: %v\n%s", err, output)
	}
	manifest := pluginsdk.Manifest{
		SchemaVersion: pluginsdk.ManifestVersion, ID: "fixture-inspector", Name: "Fixture inspector", Version: "1.0.0", ProtocolVersion: pluginsdk.ProtocolVersion,
		Executable: executableName, Capabilities: []pluginsdk.Capability{pluginsdk.CapabilityProjectInspect, pluginsdk.CapabilityProjectOperate},
		RequestedScopes: []pluginsdk.Scope{pluginsdk.ScopeProjectMetadataRead, pluginsdk.ScopeProjectFilesRead, pluginsdk.ScopeProjectOperate},
	}
	raw, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(packageDir, "plugin.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	database, err := sqlite.Open(t.Context(), filepath.Join(data, "switchyard.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	service, err := pluginsApplication.NewService(
		sqlite.NewPluginRepository(database), pluginsAdapters.NewDirectoryDiscovery(filepath.Join(data, "plugins")),
		pluginsAdapters.NewProcessRunner("test-host", nil), trustedFixtureProject{root: filepath.Join(root, "test", "fixtures", "node-single-process")},
	)
	if err != nil {
		t.Fatal(err)
	}
	items, err := service.Refresh(t.Context())
	if err != nil || len(items) != 1 {
		t.Fatalf("Refresh() = %#v, %v", items, err)
	}
	if _, err := service.Trust(t.Context(), items[0].ID, items[0].Fingerprint); err != nil {
		t.Fatal(err)
	}
	scopes := []string{string(pluginsdk.ScopeProjectMetadataRead), string(pluginsdk.ScopeProjectFilesRead), string(pluginsdk.ScopeProjectOperate)}
	if _, err := service.Enable(t.Context(), items[0].ID, scopes); err != nil {
		t.Fatal(err)
	}
	inspection, err := service.Inspect(t.Context(), items[0].ID, "fixture-project")
	if err != nil || len(inspection.Facts) == 0 || inspection.Facts[0].Value != "package.json" {
		t.Fatalf("Inspect() = %#v, %v", inspection, err)
	}
	operation, err := service.Operate(t.Context(), items[0].ID, "fixture-project", "fixture.echo", json.RawMessage(`{"reviewed":true}`))
	if err != nil || operation.Status != "succeeded" || string(operation.Output) != `{"reviewed":true}` {
		t.Fatalf("Operate() = %#v, %v", operation, err)
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	return filepath.Dir(filepath.Dir(file))
}
