package adapters

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	pluginsdk "switchyard.dev/switchyard/sdk/plugin"
)

func TestDirectoryDiscoveryFingerprintsManifestAndExecutable(t *testing.T) {
	root := t.TempDir()
	packageDir := filepath.Join(root, "fixture")
	if err := os.Mkdir(packageDir, 0o700); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(packageDir, "plugin")
	if err := os.WriteFile(executable, []byte("fixture-binary-v1"), 0o700); err != nil {
		t.Fatal(err)
	}
	manifest := pluginsdk.Manifest{
		SchemaVersion: pluginsdk.ManifestVersion, ID: "fixture", Name: "Fixture", Version: "1.0.0", ProtocolVersion: pluginsdk.ProtocolVersion,
		Executable: "plugin", Capabilities: []pluginsdk.Capability{pluginsdk.CapabilityProjectInspect}, RequestedScopes: []pluginsdk.Scope{pluginsdk.ScopeProjectMetadataRead},
	}
	raw, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(packageDir, "plugin.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	discovery := NewDirectoryDiscovery(root)
	first, err := discovery.Discover(t.Context())
	if err != nil || len(first) != 1 || len(first[0].Fingerprint) != 64 {
		t.Fatalf("Discover() = %#v, %v", first, err)
	}
	if err := os.WriteFile(executable, []byte("fixture-binary-v2"), 0o700); err != nil {
		t.Fatal(err)
	}
	second, err := discovery.Discover(t.Context())
	if err != nil || first[0].Fingerprint == second[0].Fingerprint {
		t.Fatalf("updated fingerprint = %#v, %v", second, err)
	}
}

func TestDirectoryDiscoveryRejectsEscapingExecutable(t *testing.T) {
	root := t.TempDir()
	packageDir := filepath.Join(root, "fixture")
	_ = os.Mkdir(packageDir, 0o700)
	manifest := pluginsdk.Manifest{
		SchemaVersion: pluginsdk.ManifestVersion, ID: "fixture", Name: "Fixture", Version: "1.0.0", ProtocolVersion: pluginsdk.ProtocolVersion,
		Executable: "../plugin", Capabilities: []pluginsdk.Capability{pluginsdk.CapabilityProjectInspect}, RequestedScopes: []pluginsdk.Scope{pluginsdk.ScopeProjectMetadataRead},
	}
	raw, _ := json.Marshal(manifest)
	_ = os.WriteFile(filepath.Join(packageDir, "plugin.json"), raw, 0o600)
	if _, err := NewDirectoryDiscovery(root).Discover(t.Context()); err == nil {
		t.Fatal("expected executable containment rejection")
	}
}
