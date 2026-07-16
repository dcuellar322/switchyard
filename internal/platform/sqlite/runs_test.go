package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

func TestRunRepositoryPersistsFingerprintsAndTerminalOutcome(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database, err := Open(ctx, filepath.Join(t.TempDir(), "switchyard.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	now := time.Now().UTC().Truncate(time.Millisecond)
	_, err = database.connection.ExecContext(ctx, `INSERT INTO projects
        (id, slug, display_name, trust_state, primary_location, created_at, updated_at)
        VALUES ('project-run', 'project-run', 'Project Run', 'trusted', '/tmp/project-run', ?, ?)`, formatTime(now), formatTime(now))
	if err != nil {
		t.Fatal(err)
	}
	repository := NewRunRepository(database)
	run := domain.RunRecord{
		ID: "run-1", ProjectID: "project-run", ServiceID: "api", RuntimeDriver: domain.KindProcess,
		Origin: domain.OriginSwitchyard, StartedAt: now, IdentityFingerprint: "fingerprint-1", OperationID: "op-1",
	}
	if err := repository.CreateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	identity := domain.ProcessIdentity{
		RunID: run.ID, PID: 123, ProcessGroup: 123, Executable: "/usr/bin/uv", StartedAt: now,
		WorkingDirectory: "/tmp/project-run", Fingerprint: "fingerprint-1", ObservedAt: now,
	}
	if err := repository.RecordProcess(ctx, identity); err != nil {
		t.Fatal(err)
	}
	if err := repository.SetRestartCount(ctx, run.ID, 1); err != nil {
		t.Fatal(err)
	}
	exitCode := 17
	endedAt := now.Add(time.Second)
	if err := repository.FinishRun(ctx, run.ID, endedAt, &exitCode, "crashed"); err != nil {
		t.Fatal(err)
	}
	runs, err := repository.ListProjectRuns(ctx, run.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].EndedAt == nil || runs[0].ExitCode == nil || *runs[0].ExitCode != 17 ||
		runs[0].RestartCount != 1 || runs[0].OperationID != "op-1" || len(runs[0].Processes) != 1 || runs[0].Processes[0].Fingerprint != "fingerprint-1" {
		t.Fatalf("runs = %#v", runs)
	}
}
