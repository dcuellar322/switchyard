package sqlite

import (
	"context"
	"fmt"
	"time"

	"switchyard.dev/switchyard/internal/workspace/domain"
)

// RecoverWorkspaceExecutions closes snapshots interrupted by daemon restart.
// Already-running projects remain visible as running because the runtime may
// survive the daemon; in-flight states become cancelled pending inspection.
func (r *WorkspaceRepository) RecoverWorkspaceExecutions(ctx context.Context, at time.Time) error {
	tx, err := r.database.connection.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE workspace_run_projects
        SET state='cancelled', message='daemon restarted during workspace execution', finished_at=?
        WHERE run_id IN (SELECT id FROM workspace_runs WHERE state='running')
          AND state IN ('queued','starting','checking_health','stopping','rolling_back')`, formatTime(at)); err != nil {
		return fmt.Errorf("recover workspace project progress: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE workspace_runs
        SET state=CASE WHEN EXISTS (
              SELECT 1 FROM workspace_run_projects p WHERE p.run_id=workspace_runs.id AND p.state='running'
            ) THEN 'partially_succeeded' ELSE 'failed' END,
            error_message='daemon restarted during workspace execution', finished_at=?
        WHERE state='running'`, formatTime(at)); err != nil {
		return fmt.Errorf("recover workspace executions: %w", err)
	}
	return tx.Commit()
}

// SaveExecution atomically upserts one complete project-visible execution snapshot.
func (r *WorkspaceRepository) SaveExecution(ctx context.Context, execution domain.ExecutionSummary) error {
	tx, err := r.database.connection.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin workspace execution snapshot: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `INSERT INTO workspace_runs
        (id, workspace_id, kind, state, failure_policy, profile_id, remove_data, error_message, started_at, finished_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET state=excluded.state, failure_policy=excluded.failure_policy,
          profile_id=excluded.profile_id, remove_data=excluded.remove_data,
          error_message=excluded.error_message, finished_at=excluded.finished_at`, execution.ID,
		execution.WorkspaceID, execution.Kind, execution.State, execution.Policy, execution.ProfileID,
		boolInteger(execution.RemoveData), execution.ErrorMessage, formatTime(execution.StartedAt), nullTime(execution.FinishedAt))
	if err != nil {
		return fmt.Errorf("upsert workspace execution: %w", err)
	}
	for _, project := range execution.Projects {
		_, err := tx.ExecContext(ctx, `INSERT INTO workspace_run_projects
            (run_id, project_id, role, state, message, sort_order, started_at, finished_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?)
            ON CONFLICT(run_id, project_id) DO UPDATE SET state=excluded.state, message=excluded.message,
              sort_order=excluded.sort_order, started_at=excluded.started_at, finished_at=excluded.finished_at`,
			execution.ID, project.ProjectID, project.Role, project.Status, project.Message, project.Order,
			nullTime(project.StartedAt), nullTime(project.FinishedAt))
		if err != nil {
			return fmt.Errorf("upsert workspace project result %s: %w", project.ProjectID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit workspace execution snapshot: %w", err)
	}
	return nil
}
