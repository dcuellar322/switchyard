package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"switchyard.dev/switchyard/internal/telemetry/domain"
)

type TelemetryRepository struct{ database *Database }

func NewTelemetryRepository(database *Database) *TelemetryRepository {
	return &TelemetryRepository{database: database}
}

func (r *TelemetryRepository) Status(ctx context.Context) (domain.Status, error) {
	var status domain.Status
	var enabled int
	var lastSent sql.NullString
	var updated string
	if err := r.database.connection.QueryRowContext(ctx, `SELECT enabled, endpoint, installation_id,
        last_sent_at, last_error, updated_at FROM telemetry_settings WHERE singleton = 1`).Scan(
		&enabled, &status.Settings.Endpoint, &status.Settings.InstallationID, &lastSent, &status.LastError, &updated,
	); err != nil {
		return domain.Status{}, err
	}
	status.Settings.Enabled = enabled == 1
	var err error
	status.Settings.UpdatedAt, err = parseTime(updated)
	if err != nil {
		return domain.Status{}, err
	}
	if lastSent.Valid {
		value, err := parseTime(lastSent.String)
		if err != nil {
			return domain.Status{}, err
		}
		status.LastSentAt = &value
	}
	rows, err := r.database.connection.QueryContext(ctx, `SELECT name, value FROM telemetry_counters ORDER BY name`)
	if err != nil {
		return domain.Status{}, err
	}
	defer func() { _ = rows.Close() }()
	status.Counters = []domain.Counter{}
	for rows.Next() {
		var counter domain.Counter
		if err := rows.Scan(&counter.Name, &counter.Value); err != nil {
			return domain.Status{}, err
		}
		status.Counters = append(status.Counters, counter)
	}
	return status, rows.Err()
}

func (r *TelemetryRepository) Configure(ctx context.Context, settings domain.Settings, clear bool, event domain.AuditEvent) error {
	tx, err := r.database.connection.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE telemetry_settings SET enabled=?, endpoint=?, installation_id=?,
        last_sent_at=CASE WHEN ?=1 THEN NULL ELSE last_sent_at END, last_error='', updated_at=? WHERE singleton=1`,
		boolInteger(settings.Enabled), settings.Endpoint, settings.InstallationID, boolInteger(clear), formatTime(settings.UpdatedAt)); err != nil {
		return fmt.Errorf("configure anonymous telemetry: %w", err)
	}
	if clear {
		if _, err := tx.ExecContext(ctx, `DELETE FROM telemetry_counters`); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO telemetry_audit_events
        (event_type, actor_type, actor_id, detail, occurred_at) VALUES (?, ?, ?, ?, ?)`,
		event.Type, event.ActorType, event.ActorID, event.Detail, formatTime(event.OccurredAt)); err != nil {
		return fmt.Errorf("record telemetry audit: %w", err)
	}
	return tx.Commit()
}

func (r *TelemetryRepository) Increment(ctx context.Context, name string, now time.Time) error {
	_, err := r.database.connection.ExecContext(ctx, `INSERT INTO telemetry_counters(name, value, updated_at)
        SELECT ?, 1, ? FROM telemetry_settings WHERE singleton=1 AND enabled=1
        ON CONFLICT(name) DO UPDATE SET value=value+1, updated_at=excluded.updated_at`, name, formatTime(now))
	if err != nil {
		return fmt.Errorf("increment anonymous telemetry counter: %w", err)
	}
	return nil
}

func (r *TelemetryRepository) RecordDelivery(ctx context.Context, success bool, message string, now time.Time) error {
	var lastSent any
	if success {
		lastSent, message = formatTime(now), ""
	}
	_, err := r.database.connection.ExecContext(ctx, `UPDATE telemetry_settings SET
        last_sent_at=COALESCE(?, last_sent_at), last_error=?, updated_at=? WHERE singleton=1`, lastSent, message, formatTime(now))
	if err != nil {
		return fmt.Errorf("record anonymous telemetry delivery: %w", err)
	}
	return nil
}
