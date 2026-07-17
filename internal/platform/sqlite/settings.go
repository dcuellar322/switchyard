package sqlite

import (
	"context"
	"encoding/json"
	"fmt"

	settingsApplication "switchyard.dev/switchyard/internal/settings/application"
	"switchyard.dev/switchyard/internal/settings/domain"
)

// SettingsRepository persists one revisioned settings document and value-free
// mutation audits.
type SettingsRepository struct{ database *Database }

// NewSettingsRepository constructs settings persistence.
func NewSettingsRepository(database *Database) *SettingsRepository {
	return &SettingsRepository{database: database}
}

// Initialize stores daemon defaults only when no durable document exists.
func (r *SettingsRepository) Initialize(ctx context.Context, defaults domain.Settings) (domain.Settings, error) {
	document, err := json.Marshal(defaults)
	if err != nil {
		return domain.Settings{}, err
	}
	if _, err := r.database.connection.ExecContext(ctx, `INSERT OR IGNORE INTO settings
        (singleton, revision, document_json, updated_at) VALUES (1, ?, ?, ?)`,
		defaults.Revision, document, formatTime(defaults.UpdatedAt)); err != nil {
		return domain.Settings{}, fmt.Errorf("initialize settings: %w", err)
	}
	return r.Get(ctx)
}

// Get returns the current durable settings singleton.
func (r *SettingsRepository) Get(ctx context.Context) (domain.Settings, error) {
	var revision int64
	var document, updated string
	err := r.database.connection.QueryRowContext(ctx, `SELECT revision, document_json, updated_at
        FROM settings WHERE singleton = 1`).Scan(&revision, &document, &updated)
	if err != nil {
		return domain.Settings{}, fmt.Errorf("read settings: %w", err)
	}
	var settings domain.Settings
	if err := json.Unmarshal([]byte(document), &settings); err != nil {
		return domain.Settings{}, fmt.Errorf("decode settings: %w", err)
	}
	settings.Revision = revision
	settings.UpdatedAt, err = parseTime(updated)
	if err != nil {
		return domain.Settings{}, fmt.Errorf("decode settings timestamp: %w", err)
	}
	return settings, nil
}

// Update performs one compare-and-swap replacement and audit transaction.
func (r *SettingsRepository) Update(ctx context.Context, expectedRevision int64, settings domain.Settings, audit settingsApplication.Audit) (domain.Settings, error) {
	document, err := json.Marshal(settings)
	if err != nil {
		return domain.Settings{}, err
	}
	sections, err := json.Marshal(audit.Sections)
	if err != nil {
		return domain.Settings{}, err
	}
	tx, err := r.database.connection.BeginTx(ctx, nil)
	if err != nil {
		return domain.Settings{}, err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE settings SET revision=?, document_json=?, updated_at=?
        WHERE singleton=1 AND revision=?`, settings.Revision, document, formatTime(settings.UpdatedAt), expectedRevision)
	if err != nil {
		return domain.Settings{}, fmt.Errorf("update settings: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return domain.Settings{}, err
	}
	if changed != 1 {
		return domain.Settings{}, settingsApplication.ErrRevisionConflict
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO settings_audit_events
        (revision, actor_type, actor_id, sections_json, occurred_at) VALUES (?, ?, ?, ?, ?)`,
		settings.Revision, audit.ActorType, audit.ActorID, sections, formatTime(audit.OccurredAt)); err != nil {
		return domain.Settings{}, fmt.Errorf("audit settings update: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return domain.Settings{}, err
	}
	return settings, nil
}
