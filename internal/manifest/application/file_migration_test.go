package application

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateFilePreviewsThenAppliesWithBackup(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "project.yml")
	contents := []byte("schemaVersion: switchyard.dev/v1alpha1\nkind: Project\nmetadata:\n  id: fixture\n  name: Fixture\nrepository:\n  root: .\n")
	if err := os.WriteFile(path, contents, 0o640); err != nil {
		t.Fatal(err)
	}
	preview, err := MigrateFile(path, false)
	if err != nil || !preview.Changed || preview.Applied || !strings.Contains(preview.Preview, "switchyard.dev/v1") {
		t.Fatalf("preview = %#v error=%v", preview, err)
	}
	unchanged, _ := os.ReadFile(path)
	if string(unchanged) != string(contents) {
		t.Fatal("preview changed the source file")
	}
	applied, err := MigrateFile(path, true)
	if err != nil || !applied.Applied {
		t.Fatalf("applied = %#v error=%v", applied, err)
	}
	backup, err := os.ReadFile(applied.BackupPath)
	if err != nil || string(backup) != string(contents) {
		t.Fatalf("backup = %q error=%v", backup, err)
	}
	migrated, _ := os.ReadFile(path)
	if strings.Contains(string(migrated), "v1alpha1") {
		t.Fatalf("migrated = %s", migrated)
	}
}

func TestMigrateFileNeverOverwritesExistingBackup(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "project.yml")
	contents := []byte("schemaVersion: switchyard.dev/v1alpha1\nkind: Project\nmetadata:\n  id: fixture\n  name: Fixture\nrepository:\n  root: .\n")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".v1alpha1.bak", []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := MigrateFile(path, true); err == nil {
		t.Fatal("MigrateFile() error = nil")
	}
	current, _ := os.ReadFile(path)
	if string(current) != string(contents) {
		t.Fatal("failed migration changed source")
	}
}
