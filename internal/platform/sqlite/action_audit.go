package sqlite

import (
	"context"
	"fmt"
	"time"

	"switchyard.dev/switchyard/internal/actions/domain"
)

// ActionAuditRepository persists command-free, environment-free action audits.
type ActionAuditRepository struct{ database *Database }

// NewActionAuditRepository creates an action audit store.
func NewActionAuditRepository(database *Database) *ActionAuditRepository {
	return &ActionAuditRepository{database: database}
}

// Begin records action identity before platform execution starts.
func (r *ActionAuditRepository) Begin(ctx context.Context, audit domain.Audit) error {
	_, err := r.database.connection.ExecContext(ctx, `INSERT INTO action_audit
        (id, operation_id, project_id, action_id, action_type, risk, actor_type, actor_id, state, working_directory, started_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET state='running', finished_at=NULL, error_code=NULL, started_at=excluded.started_at`, audit.ID, audit.OperationID, audit.ProjectID, audit.ActionID,
		audit.ActionType, audit.Risk, audit.ActorType, audit.ActorID, audit.State, audit.WorkingDirectory, formatTime(audit.StartedAt))
	if err != nil {
		return fmt.Errorf("insert action audit: %w", err)
	}
	return nil
}

// Recover marks action audits left running by a daemon interruption.
func (r *ActionAuditRepository) Recover(ctx context.Context, at time.Time) error {
	_, err := r.database.connection.ExecContext(ctx, `UPDATE action_audit
        SET state='failed', error_code='DAEMON_RESTARTED', finished_at=? WHERE state='running'`, formatTime(at))
	if err != nil {
		return fmt.Errorf("recover action audits: %w", err)
	}
	return nil
}

// Finish stores the terminal outcome without persisting output or environment values.
func (r *ActionAuditRepository) Finish(ctx context.Context, id, state, errorCode string, at time.Time) error {
	_, err := r.database.connection.ExecContext(ctx, `UPDATE action_audit SET state = ?, error_code = NULLIF(?, ''), finished_at = ? WHERE id = ?`,
		state, errorCode, formatTime(at), id)
	if err != nil {
		return fmt.Errorf("finish action audit: %w", err)
	}
	return nil
}
