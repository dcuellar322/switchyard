package sqlite

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
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
	if version != 13 {
		t.Fatalf("SchemaVersion() = %d, want 13", version)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("database permissions = %o, want 600", got)
	}
}

func TestOpenBacksUpAndPreservesAlphaDataBeforeUpgrade(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "switchyard.db")
	connection, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := migrationProvider(connection)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 3); err != nil {
		t.Fatal(err)
	}
	_, err = connection.Exec(`
		INSERT INTO projects (id, slug, display_name, trust_state, primary_location, created_at, updated_at)
		VALUES ('project-1','fixture','Fixture','trusted','/tmp/fixture','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z');
		INSERT INTO manifest_proposals (id, project_id, scanner_version, schema_version, candidate_json, confidence_json, unresolved_json, validation_json, status, created_at)
		VALUES ('proposal-1','project-1','deterministic/v1','switchyard.dev/v1alpha1','{}','{}','[]','{}','accepted','2026-01-01T00:00:00Z');
		INSERT INTO manifest_snapshots (project_id, revision, proposal_id, manifest_json, created_at)
		VALUES ('project-1',1,'proposal-1','{"schemaVersion":"switchyard.dev/v1alpha1"}','2026-01-01T00:00:00Z');
		INSERT INTO audit_events (event_type, actor_type, actor_id, project_id, detail_json, occurred_at)
		VALUES ('project.accepted','user','fixture','project-1','{}','2026-01-01T00:00:00Z');
	`)
	if err != nil {
		t.Fatal(err)
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	database, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	backupPath := preMigrationBackupPath(path, 3, 13)
	for _, candidate := range []string{path, backupPath} {
		check, err := sql.Open("sqlite", candidate)
		if err != nil {
			t.Fatal(err)
		}
		for _, table := range []string{"projects", "manifest_snapshots", "audit_events"} {
			var count int
			if err := check.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil || count != 1 {
				_ = check.Close()
				t.Fatalf("%s %s count=%d error=%v", candidate, table, count, err)
			}
		}
		_ = check.Close()
	}
	status, err := InspectMigrations(ctx, path)
	if err != nil || status.CurrentVersion != 13 || status.MigrationRequired {
		t.Fatalf("status=%#v error=%v", status, err)
	}
}

func TestOpenRejectsDatabaseFromNewerBinary(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "switchyard.db")
	database, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	connection, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	if _, err := connection.Exec(`INSERT INTO goose_db_version (version_id, is_applied, tstamp) VALUES (999, 1, CURRENT_TIMESTAMP)`); err != nil {
		_ = connection.Close()
		t.Fatalf("insert future migration version: %v", err)
	}
	if err := connection.Close(); err != nil {
		t.Fatalf("close future database: %v", err)
	}

	_, err = Open(context.Background(), path)
	if err == nil || !strings.Contains(err.Error(), "newer than this binary supports") {
		t.Fatalf("Open() error = %v, want newer-schema rejection", err)
	}
}
