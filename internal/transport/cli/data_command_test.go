package cli

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"switchyard.dev/switchyard/internal/bootstrap"
)

func TestDataInspectAndBackupCommandsDoNotStartDaemon(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	if _, err := bootstrap.MigrateData(context.Background(), dataDir); err != nil {
		t.Fatal(err)
	}
	databasePath := filepath.Join(dataDir, "switchyard.db")
	database, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(`
		INSERT INTO settings (singleton, revision, document_json, updated_at)
		VALUES (1, 4, '{"fixture":"settings-survive-backup"}', '2026-07-16T00:00:00Z');
		INSERT INTO settings_audit_events (revision, actor_type, actor_id, sections_json, occurred_at)
		VALUES (4, 'user', 'fixture', '["appearance"]', '2026-07-16T00:00:00Z');
	`)
	if err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := Execute(context.Background(), []string{"--data-dir", dataDir, "data", "inspect"}, &output, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "schema=17 target=17 migration=false") {
		t.Fatalf("inspect output = %q", output.String())
	}
	output.Reset()
	backup := filepath.Join(dataDir, "manual.bak")
	if err := Execute(context.Background(), []string{"--data-dir", dataDir, "data", "backup", "--output", backup}, &output, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Verified backup") {
		t.Fatalf("backup output = %q", output.String())
	}
	if info, err := os.Stat(backup); err != nil || info.Size() == 0 {
		t.Fatalf("backup info=%v error=%v", info, err)
	}
	backupDatabase, err := sql.Open("sqlite", backup)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = backupDatabase.Close() }()
	var revision, auditCount int
	var document string
	if err := backupDatabase.QueryRow(`SELECT revision, document_json FROM settings WHERE singleton=1`).Scan(&revision, &document); err != nil {
		t.Fatal(err)
	}
	if err := backupDatabase.QueryRow(`SELECT COUNT(*) FROM settings_audit_events WHERE revision=4`).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if revision != 4 || document != `{"fixture":"settings-survive-backup"}` || auditCount != 1 {
		t.Fatalf("backup settings revision=%d document=%q audits=%d", revision, document, auditCount)
	}
}
