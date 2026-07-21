package application

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRootRejectsSecretsAndEscapingSymlinks(t *testing.T) {
	t.Parallel()
	rootPath := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootPath, ".env"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(rootPath, "README.md")); err != nil {
		t.Fatal(err)
	}
	root, err := SelectRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := root.ReadFile(".env"); err == nil {
		t.Fatal("ReadFile(.env) unexpectedly succeeded")
	}
	if _, err := root.ReadFile("README.md"); err == nil {
		t.Fatal("ReadFile(escaping symlink) unexpectedly succeeded")
	}
	if _, err := root.HasFile(".env"); err == nil {
		t.Fatal("HasFile(.env) unexpectedly succeeded")
	}
	if _, err := root.HasFile("README.md"); err == nil {
		t.Fatal("HasFile(escaping symlink) unexpectedly succeeded")
	}
}

func TestRootHasFileDoesNotReadContents(t *testing.T) {
	t.Parallel()
	rootPath := t.TempDir()
	large := filepath.Join(rootPath, "pnpm-lock.yaml")
	file, err := os.Create(large) //nolint:gosec // The temporary path is test-owned.
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxDiscoveryFileSize + 1); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	root, err := SelectRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	exists, err := root.HasFile("pnpm-lock.yaml")
	if err != nil || !exists {
		t.Fatalf("HasFile() = %t, %v", exists, err)
	}
	if _, err := root.ReadFile("pnpm-lock.yaml"); err == nil {
		t.Fatal("ReadFile() unexpectedly accepted an oversized lockfile")
	}
}
