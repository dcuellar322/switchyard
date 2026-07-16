package application

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"switchyard.dev/switchyard/internal/manifest/domain"
)

func TestResolveLocalOverlayWinsWithoutModifyingPortableManifest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	manifestDirectory := filepath.Join(root, ".switchyard")
	if err := os.MkdirAll(manifestDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	portable := []byte(`
schemaVersion: switchyard.dev/v1alpha1
kind: Project
metadata: {id: overlay-test, name: Portable Name}
repository: {root: .}
`)
	if err := os.WriteFile(filepath.Join(manifestDirectory, "project.yml"), portable, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(manifestDirectory, "project.local.yml"), []byte("metadata:\n  name: Local Name\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	resolved, err := Resolve(root, domain.Manifest{}, domain.Manifest{}, nil)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.Manifest.Metadata.Name != "Local Name" {
		t.Fatalf("name = %q, want local overlay", resolved.Manifest.Metadata.Name)
	}
	if resolved.Provenance["/metadata/name"] != "local-overlay" {
		t.Fatalf("provenance = %#v", resolved.Provenance)
	}
	after, err := os.ReadFile(filepath.Join(manifestDirectory, "project.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, portable) {
		t.Fatal("portable manifest was modified")
	}
}
