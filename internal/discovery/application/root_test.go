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
}
