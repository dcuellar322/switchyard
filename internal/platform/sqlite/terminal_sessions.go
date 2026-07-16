package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	terminalApplication "switchyard.dev/switchyard/internal/terminal/application"
	"switchyard.dev/switchyard/internal/terminal/domain"
)

// TerminalSessionRepository persists metadata and audit facts, never PTY bytes.
type TerminalSessionRepository struct{ database *Database }

// NewTerminalSessionRepository creates the durable terminal metadata adapter.
func NewTerminalSessionRepository(database *Database) *TerminalSessionRepository {
	return &TerminalSessionRepository{database: database}
}

// Create stores one validated starting session.
func (r *TerminalSessionRepository) Create(ctx context.Context, session domain.Session) error {
	if err := session.Validate(); err != nil {
		return err
	}
	_, err := r.database.connection.ExecContext(ctx, `INSERT INTO terminal_sessions
        (id, project_id, environment_id, kind, display_name, owner_type, owner_id, provider, service_id, action_id,
         working_directory, status, persistence_policy, capture_policy, output_bytes, output_truncated,
         last_output_at, exit_code, created_at, last_attached_at, detached_at, finished_at, error_code)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.ProjectID, session.EnvironmentID, session.Kind, session.DisplayName,
		session.Owner.Type, session.Owner.ID, session.Provider, session.ServiceID, session.ActionID,
		session.WorkingDirectory, session.Status, session.PersistencePolicy, session.CapturePolicy,
		session.OutputBytes, boolInteger(session.OutputTruncated), nullableTime(session.LastOutputAt), nullableIntPointer(session.ExitCode),
		formatTime(session.CreatedAt), nullableTime(session.LastAttachedAt), nullableTime(session.DetachedAt), nullableTime(session.FinishedAt), session.ErrorCode)
	if err != nil {
		return fmt.Errorf("insert terminal session: %w", err)
	}
	return nil
}

// Update replaces mutable lifecycle and output metadata.
func (r *TerminalSessionRepository) Update(ctx context.Context, session domain.Session) error {
	if err := session.Validate(); err != nil {
		return err
	}
	result, err := r.database.connection.ExecContext(ctx, `UPDATE terminal_sessions SET
        status=?, output_bytes=?, output_truncated=?, last_output_at=?, exit_code=?, last_attached_at=?,
        detached_at=?, finished_at=?, error_code=? WHERE id=? AND owner_type=? AND owner_id=?`,
		session.Status, session.OutputBytes, boolInteger(session.OutputTruncated), nullableTime(session.LastOutputAt),
		nullableIntPointer(session.ExitCode), nullableTime(session.LastAttachedAt), nullableTime(session.DetachedAt),
		nullableTime(session.FinishedAt), session.ErrorCode, session.ID, session.Owner.Type, session.Owner.ID)
	if err != nil {
		return fmt.Errorf("update terminal session: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return terminalApplication.ErrNotFound
	}
	return nil
}

// Get reads one durable session record.
func (r *TerminalSessionRepository) Get(ctx context.Context, id string) (domain.Session, error) {
	row := r.database.connection.QueryRowContext(ctx, terminalSessionSelect+` WHERE id = ?`, id)
	session, err := scanTerminalSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Session{}, terminalApplication.ErrNotFound
	}
	return session, err
}

// List reads recent sessions in stable newest-first order.
func (r *TerminalSessionRepository) List(ctx context.Context, projectID string) ([]domain.Session, error) {
	query := terminalSessionSelect
	arguments := []any{}
	if projectID != "" {
		query += ` WHERE project_id = ?`
		arguments = append(arguments, projectID)
	}
	query += ` ORDER BY created_at DESC, id`
	rows, err := r.database.connection.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("list terminal sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make([]domain.Session, 0)
	for rows.Next() {
		session, scanErr := scanTerminalSession(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, session)
	}
	return result, rows.Err()
}

// InterruptActive closes unrecoverable PTY ownership after daemon restart.
func (r *TerminalSessionRepository) InterruptActive(ctx context.Context, at time.Time) error {
	_, err := r.database.connection.ExecContext(ctx, `UPDATE terminal_sessions
        SET status='interrupted', finished_at=?, error_code='DAEMON_RESTARTED'
        WHERE status IN ('starting','active')`, formatTime(at))
	if err != nil {
		return fmt.Errorf("interrupt recovered terminal sessions: %w", err)
	}
	return nil
}

// AppendAudit stores one metadata-only ownership or lifecycle event.
func (r *TerminalSessionRepository) AppendAudit(ctx context.Context, audit domain.Audit) error {
	detail, err := json.Marshal(audit.Detail)
	if err != nil {
		return fmt.Errorf("encode terminal audit detail: %w", err)
	}
	_, err = r.database.connection.ExecContext(ctx, `INSERT INTO terminal_session_audits
        (id, session_id, event, actor_type, actor_id, detail_json, occurred_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		audit.ID, audit.SessionID, audit.Event, audit.Actor.Type, audit.Actor.ID, detail, formatTime(audit.OccurredAt))
	if err != nil {
		return fmt.Errorf("insert terminal session audit: %w", err)
	}
	return nil
}

const terminalSessionSelect = `SELECT id, project_id, environment_id, kind, display_name, owner_type, owner_id,
    provider, service_id, action_id, working_directory, status, persistence_policy, capture_policy,
    output_bytes, output_truncated, last_output_at, exit_code, created_at, last_attached_at,
    detached_at, finished_at, error_code FROM terminal_sessions`

type terminalSessionScanner interface{ Scan(...any) error }

func scanTerminalSession(row terminalSessionScanner) (domain.Session, error) {
	var session domain.Session
	var truncated int
	var lastOutput, lastAttached, detached, finished sql.NullString
	var exitCode sql.NullInt64
	var created string
	err := row.Scan(
		&session.ID, &session.ProjectID, &session.EnvironmentID, &session.Kind, &session.DisplayName,
		&session.Owner.Type, &session.Owner.ID, &session.Provider, &session.ServiceID, &session.ActionID,
		&session.WorkingDirectory, &session.Status, &session.PersistencePolicy, &session.CapturePolicy,
		&session.OutputBytes, &truncated, &lastOutput, &exitCode, &created, &lastAttached, &detached, &finished, &session.ErrorCode,
	)
	if err != nil {
		return domain.Session{}, err
	}
	session.OutputTruncated = truncated == 1
	if session.CreatedAt, err = parseTime(created); err != nil {
		return domain.Session{}, err
	}
	if session.LastOutputAt, err = parseNullableTime(lastOutput); err != nil {
		return domain.Session{}, err
	}
	if session.LastAttachedAt, err = parseNullableTime(lastAttached); err != nil {
		return domain.Session{}, err
	}
	if session.DetachedAt, err = parseNullableTime(detached); err != nil {
		return domain.Session{}, err
	}
	if session.FinishedAt, err = parseNullableTime(finished); err != nil {
		return domain.Session{}, err
	}
	if exitCode.Valid {
		value := int(exitCode.Int64)
		session.ExitCode = &value
	}
	return session, session.Validate()
}

func nullableIntPointer(value *int) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*value), Valid: true}
}
