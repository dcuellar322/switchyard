package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	terminalApplication "switchyard.dev/switchyard/internal/terminal/application"
	"switchyard.dev/switchyard/internal/terminal/domain"
)

func TestTerminalSessionRepositoryRoundTripsMetadataAuditAndRecovery(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database, err := Open(ctx, filepath.Join(t.TempDir(), "switchyard.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	now := time.Date(2026, 7, 16, 12, 0, 0, 123000000, time.UTC)
	_, err = database.connection.ExecContext(ctx, `INSERT INTO projects
        (id, slug, display_name, trust_state, primary_location, created_at, updated_at)
        VALUES ('project-terminal', 'project-terminal', 'Terminal', 'trusted', '/tmp/project-terminal', ?, ?)`, formatTime(now), formatTime(now))
	if err != nil {
		t.Fatal(err)
	}
	repository := NewTerminalSessionRepository(database)
	session := domain.Session{
		ID: "terminal_one", ProjectID: "project-terminal", Kind: domain.KindAgent, DisplayName: "Terminal · Codex",
		Owner: domain.Owner{Type: "browser", ID: "browser_one"}, Provider: "codex", WorkingDirectory: "/tmp/project-terminal",
		Status: domain.StatusActive, PersistencePolicy: domain.PersistenceDetachUntilIdle, CapturePolicy: domain.CaptureUserVisibleOutput,
		CreatedAt: now, DetachedAt: &now,
	}
	if err := repository.Create(ctx, session); err != nil {
		t.Fatal(err)
	}
	lastOutput := now.Add(time.Second)
	lastAttached := now.Add(2 * time.Second)
	session.OutputBytes = 4097
	session.OutputTruncated = true
	session.LastOutputAt = &lastOutput
	session.LastAttachedAt = &lastAttached
	session.DetachedAt = nil
	if err := repository.Update(ctx, session); err != nil {
		t.Fatal(err)
	}
	got, err := repository.Get(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Provider != "codex" || got.OutputBytes != 4097 || !got.OutputTruncated || got.DetachedAt != nil || got.LastAttachedAt == nil {
		t.Fatalf("Get() = %#v", got)
	}
	if err := repository.AppendAudit(ctx, domain.Audit{
		ID: "terminalaudit_one", SessionID: session.ID, Event: "resized", Actor: session.Owner,
		Detail: map[string]any{"columns": 120, "rows": 36}, OccurredAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	var detail string
	if err := database.connection.QueryRowContext(ctx, `SELECT detail_json FROM terminal_session_audits WHERE id='terminalaudit_one'`).Scan(&detail); err != nil {
		t.Fatal(err)
	}
	if detail != `{"columns":120,"rows":36}` {
		t.Fatalf("audit detail = %s", detail)
	}
	interruptedAt := now.Add(time.Minute)
	if err := repository.InterruptActive(ctx, interruptedAt); err != nil {
		t.Fatal(err)
	}
	got, err = repository.Get(ctx, session.ID)
	if err != nil || got.Status != domain.StatusInterrupted || got.ErrorCode != "DAEMON_RESTARTED" || got.FinishedAt == nil {
		t.Fatalf("recovered session = %#v, %v", got, err)
	}
	items, err := repository.List(ctx, "project-terminal")
	if err != nil || len(items) != 1 {
		t.Fatalf("List() = %#v, %v", items, err)
	}
	if _, err := repository.Get(ctx, "missing"); !errors.Is(err, terminalApplication.ErrNotFound) {
		t.Fatalf("missing Get() error = %v", err)
	}
}

func TestTerminalSchemaContainsNoInputOutputOrEnvironmentPayloadColumns(t *testing.T) {
	t.Parallel()
	database, err := Open(context.Background(), filepath.Join(t.TempDir(), "switchyard.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	rows, err := database.connection.Query(`SELECT name FROM pragma_table_info('terminal_sessions')`)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := rows.Close(); err != nil {
			t.Errorf("close schema rows: %v", err)
		}
	})
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		switch name {
		case "input", "output", "command", "arguments", "environment":
			t.Fatalf("sensitive payload column %q exists", name)
		}
	}
}
