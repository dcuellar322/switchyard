package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"switchyard.dev/switchyard/internal/runtime/domain"
)

// RunRepository persists native-process ownership evidence and lifecycle outcomes.
type RunRepository struct{ database *Database }

// NewRunRepository creates a durable native-process run adapter.
func NewRunRepository(database *Database) *RunRepository {
	return &RunRepository{database: database}
}

// CreateRun writes a new run before its process is exposed as managed.
func (r *RunRepository) CreateRun(ctx context.Context, run domain.RunRecord) error {
	_, err := r.database.connection.ExecContext(ctx, `INSERT INTO runs
        (id, project_id, service_id, runtime_driver, origin, started_at, ended_at, exit_code,
         termination_reason, identity_fingerprint, restart_count)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.ProjectID, run.ServiceID, run.RuntimeDriver, run.Origin, formatTime(run.StartedAt),
		nullTime(run.EndedAt), nullInt(run.ExitCode), run.TerminationReason, run.IdentityFingerprint, run.RestartCount)
	if err != nil {
		return fmt.Errorf("create runtime run: %w", err)
	}
	return nil
}

// RecordProcess upserts one verified member of a managed process group.
func (r *RunRepository) RecordProcess(ctx context.Context, identity domain.ProcessIdentity) error {
	_, err := r.database.connection.ExecContext(ctx, `INSERT INTO run_processes
        (run_id, pid, process_group_id, executable, started_at, working_directory, identity_fingerprint, observed_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(run_id, pid, started_at) DO UPDATE SET
          process_group_id = excluded.process_group_id,
          executable = excluded.executable,
          working_directory = excluded.working_directory,
          identity_fingerprint = excluded.identity_fingerprint,
          observed_at = excluded.observed_at`,
		identity.RunID, identity.PID, identity.ProcessGroup, identity.Executable, formatTime(identity.StartedAt),
		identity.WorkingDirectory, identity.Fingerprint, formatTime(identity.ObservedAt))
	if err != nil {
		return fmt.Errorf("record run process: %w", err)
	}
	return nil
}

// FinishRun records an immutable terminal outcome for an active run.
func (r *RunRepository) FinishRun(ctx context.Context, runID string, endedAt time.Time, exitCode *int, reason string) error {
	_, err := r.database.connection.ExecContext(ctx, `UPDATE runs
        SET ended_at = ?, exit_code = ?, termination_reason = ?
        WHERE id = ? AND ended_at IS NULL`, formatTime(endedAt), nullInt(exitCode), reason, runID)
	if err != nil {
		return fmt.Errorf("finish runtime run: %w", err)
	}
	return nil
}

// SetRestartCount records each opt-in supervisor restart attempt.
func (r *RunRepository) SetRestartCount(ctx context.Context, runID string, count int) error {
	_, err := r.database.connection.ExecContext(ctx, `UPDATE runs SET restart_count = ? WHERE id = ?`, count, runID)
	if err != nil {
		return fmt.Errorf("update runtime restart count: %w", err)
	}
	return nil
}

// ListProjectRuns returns newest runs first with every recorded process fingerprint.
func (r *RunRepository) ListProjectRuns(ctx context.Context, projectID string) ([]domain.RunRecord, error) {
	rows, err := r.database.connection.QueryContext(ctx, `SELECT id, project_id, service_id, runtime_driver,
        origin, started_at, ended_at, exit_code, termination_reason, identity_fingerprint, restart_count
        FROM runs WHERE project_id = ? ORDER BY started_at DESC, id DESC LIMIT 200`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list runtime runs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := []domain.RunRecord{}
	for rows.Next() {
		run, scanErr := scanRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, run)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range result {
		result[index].Processes, err = r.runProcesses(ctx, result[index].ID)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (r *RunRepository) runProcesses(ctx context.Context, runID string) ([]domain.ProcessIdentity, error) {
	rows, err := r.database.connection.QueryContext(ctx, `SELECT run_id, pid, process_group_id, executable,
        started_at, working_directory, identity_fingerprint, observed_at
        FROM run_processes WHERE run_id = ? ORDER BY observed_at DESC, pid`, runID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []domain.ProcessIdentity{}
	for rows.Next() {
		var item domain.ProcessIdentity
		var started, observed string
		if err := rows.Scan(&item.RunID, &item.PID, &item.ProcessGroup, &item.Executable, &started,
			&item.WorkingDirectory, &item.Fingerprint, &observed); err != nil {
			return nil, err
		}
		item.StartedAt, _ = parseTime(started)
		item.ObservedAt, _ = parseTime(observed)
		result = append(result, item)
	}
	return result, rows.Err()
}

func scanRun(row rowScanner) (domain.RunRecord, error) {
	var run domain.RunRecord
	var started string
	var ended sql.NullString
	var exitCode sql.NullInt64
	if err := row.Scan(&run.ID, &run.ProjectID, &run.ServiceID, &run.RuntimeDriver, &run.Origin,
		&started, &ended, &exitCode, &run.TerminationReason, &run.IdentityFingerprint, &run.RestartCount); err != nil {
		return domain.RunRecord{}, err
	}
	run.StartedAt, _ = parseTime(started)
	if ended.Valid {
		parsed, err := parseTime(ended.String)
		if err != nil {
			return domain.RunRecord{}, err
		}
		run.EndedAt = &parsed
	}
	if exitCode.Valid {
		value := int(exitCode.Int64)
		run.ExitCode = &value
	}
	return run, nil
}

func nullTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatTime(*value)
}

func nullInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}
