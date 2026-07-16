package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenMigratesEmptyDatabase(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "switchyard.db")
	database, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	version, err := database.SchemaVersion(context.Background())
	if err != nil {
		t.Fatalf("SchemaVersion() error = %v", err)
	}
	if version != 1 {
		t.Fatalf("SchemaVersion() = %d, want 1", version)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("database permissions = %o, want 600", got)
	}
}
