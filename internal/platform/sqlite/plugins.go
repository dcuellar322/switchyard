package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	pluginsApplication "switchyard.dev/switchyard/internal/plugins/application"
	"switchyard.dev/switchyard/internal/plugins/domain"
)

// PluginRepository persists reviewed external-process identities and health.
type PluginRepository struct{ database *Database }

// NewPluginRepository creates the durable plugin registry adapter.
func NewPluginRepository(database *Database) *PluginRepository {
	return &PluginRepository{database: database}
}

// Reconcile marks missing packages unavailable and upserts current executable identities.
func (r *PluginRepository) Reconcile(ctx context.Context, discovered []domain.Plugin, now time.Time) error {
	tx, err := r.database.connection.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin plugin reconciliation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE plugin_registrations SET available = 0, enabled = 0,
        health = 'unknown', health_message = '', updated_at = ?`, formatTime(now)); err != nil {
		return fmt.Errorf("mark plugins unavailable: %w", err)
	}
	for _, current := range discovered {
		arguments, _ := json.Marshal(current.Arguments)
		capabilities, _ := json.Marshal(current.Capabilities)
		requested, _ := json.Marshal(current.RequestedScopes)
		_, err := tx.ExecContext(ctx, `INSERT INTO plugin_registrations
            (id, name, version, protocol_version, manifest_path, executable_path, arguments_json, fingerprint,
             capabilities_json, requested_scopes_json, available, discovered_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)
            ON CONFLICT(id) DO UPDATE SET
              name = excluded.name, version = excluded.version, protocol_version = excluded.protocol_version,
              manifest_path = excluded.manifest_path, executable_path = excluded.executable_path,
              arguments_json = excluded.arguments_json, fingerprint = excluded.fingerprint,
              capabilities_json = excluded.capabilities_json, requested_scopes_json = excluded.requested_scopes_json,
              available = 1,
              enabled = CASE WHEN plugin_registrations.trusted_fingerprint = excluded.fingerprint THEN plugin_registrations.enabled ELSE 0 END,
              granted_scopes_json = CASE WHEN plugin_registrations.trusted_fingerprint = excluded.fingerprint THEN plugin_registrations.granted_scopes_json ELSE '[]' END,
              health = CASE WHEN plugin_registrations.trusted_fingerprint = excluded.fingerprint THEN plugin_registrations.health ELSE 'unknown' END,
              health_message = CASE WHEN plugin_registrations.trusted_fingerprint = excluded.fingerprint THEN plugin_registrations.health_message ELSE '' END,
              last_error = CASE WHEN plugin_registrations.trusted_fingerprint = excluded.fingerprint THEN plugin_registrations.last_error ELSE 'Executable identity changed; review and trust the new fingerprint.' END,
              updated_at = excluded.updated_at`,
			current.ID, current.Name, current.Version, current.ProtocolVersion, current.ManifestPath, current.Executable,
			arguments, current.Fingerprint, capabilities, requested, formatTime(current.DiscoveredAt), formatTime(now))
		if err != nil {
			return fmt.Errorf("reconcile plugin %s: %w", current.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit plugin reconciliation: %w", err)
	}
	return nil
}

// List returns registrations in stable availability, enabled, and name order.
func (r *PluginRepository) List(ctx context.Context) ([]domain.Plugin, error) {
	rows, err := r.database.connection.QueryContext(ctx, `SELECT id FROM plugin_registrations
        ORDER BY available DESC, enabled DESC, name COLLATE NOCASE, id`)
	if err != nil {
		return nil, fmt.Errorf("list plugins: %w", err)
	}
	defer func() { _ = rows.Close() }()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result := make([]domain.Plugin, 0, len(ids))
	for _, id := range ids {
		current, err := r.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		result = append(result, current)
	}
	return result, nil
}

// Get rehydrates one plugin and derives its current trust state.
func (r *PluginRepository) Get(ctx context.Context, id string) (domain.Plugin, error) {
	row := r.database.connection.QueryRowContext(ctx, `SELECT id, name, version, protocol_version, manifest_path,
        executable_path, arguments_json, fingerprint, trusted_fingerprint, capabilities_json,
        requested_scopes_json, granted_scopes_json, available, enabled, health, health_message, last_error,
        discovered_at, updated_at FROM plugin_registrations WHERE id = ?`, id)
	var current domain.Plugin
	var arguments, capabilities, requested, granted, discovered, updated string
	var available, enabled int
	if err := row.Scan(&current.ID, &current.Name, &current.Version, &current.ProtocolVersion, &current.ManifestPath,
		&current.Executable, &arguments, &current.Fingerprint, &current.TrustedFingerprint, &capabilities,
		&requested, &granted, &available, &enabled, &current.Health, &current.HealthMessage, &current.LastError,
		&discovered, &updated); errors.Is(err, sql.ErrNoRows) {
		return domain.Plugin{}, pluginsApplication.ErrNotFound
	} else if err != nil {
		return domain.Plugin{}, fmt.Errorf("get plugin: %w", err)
	}
	for _, encoded := range []struct {
		raw    string
		target any
	}{{arguments, &current.Arguments}, {capabilities, &current.Capabilities}, {requested, &current.RequestedScopes}, {granted, &current.GrantedScopes}} {
		if err := json.Unmarshal([]byte(encoded.raw), encoded.target); err != nil {
			return domain.Plugin{}, fmt.Errorf("decode plugin record: %w", err)
		}
	}
	current.Available, current.Enabled = available == 1, enabled == 1
	current.DiscoveredAt, _ = parseTime(discovered)
	current.UpdatedAt, _ = parseTime(updated)
	switch current.TrustedFingerprint {
	case "":
		current.Trust = domain.TrustUntrusted
	case current.Fingerprint:
		current.Trust = domain.TrustTrusted
	default:
		current.Trust = domain.TrustChanged
	}
	if current.Arguments == nil {
		current.Arguments = []string{}
	}
	if current.Capabilities == nil {
		current.Capabilities = []string{}
	}
	if current.RequestedScopes == nil {
		current.RequestedScopes = []string{}
	}
	if current.GrantedScopes == nil {
		current.GrantedScopes = []string{}
	}
	return current, nil
}

// Trust records an exact available package fingerprint and revokes old grants.
func (r *PluginRepository) Trust(ctx context.Context, id, fingerprint string, now time.Time) error {
	result, err := r.database.connection.ExecContext(ctx, `UPDATE plugin_registrations SET trusted_fingerprint = ?,
        enabled = 0, granted_scopes_json = '[]', health = 'unknown', health_message = '', last_error = '', updated_at = ?
        WHERE id = ? AND available = 1 AND fingerprint = ?`, fingerprint, formatTime(now), id, fingerprint)
	if err != nil {
		return fmt.Errorf("trust plugin: %w", err)
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return pluginsApplication.ErrFingerprint
	}
	return nil
}

// SetEnabled updates enabled state and the complete reviewed grant set.
func (r *PluginRepository) SetEnabled(ctx context.Context, id string, enabled bool, scopes []string, now time.Time) error {
	if scopes == nil {
		scopes = []string{}
	}
	encoded, _ := json.Marshal(scopes)
	result, err := r.database.connection.ExecContext(ctx, `UPDATE plugin_registrations SET enabled = ?,
        granted_scopes_json = ?, updated_at = ? WHERE id = ?`, boolInteger(enabled), encoded, formatTime(now), id)
	if err != nil {
		return fmt.Errorf("update plugin enabled state: %w", err)
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return pluginsApplication.ErrNotFound
	}
	return nil
}

// SetHealth records the latest bounded supervision observation.
func (r *PluginRepository) SetHealth(ctx context.Context, id string, state domain.HealthState, message, lastError string, now time.Time) error {
	result, err := r.database.connection.ExecContext(ctx, `UPDATE plugin_registrations SET health = ?, health_message = ?,
        last_error = ?, updated_at = ? WHERE id = ?`, state, message, lastError, formatTime(now), id)
	if err != nil {
		return fmt.Errorf("update plugin health: %w", err)
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return pluginsApplication.ErrNotFound
	}
	return nil
}

// AppendLogs writes bounded entries and retains the newest 1,000 per plugin.
func (r *PluginRepository) AppendLogs(ctx context.Context, entries []domain.LogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	tx, err := r.database.connection.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	plugins := map[string]struct{}{}
	for _, entry := range entries {
		level := entry.Level
		if level != "debug" && level != "info" && level != "warning" && level != "error" {
			level = "info"
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO plugin_logs(plugin_id, level, message, created_at)
            VALUES (?, ?, ?, ?)`, entry.PluginID, level, entry.Message, formatTime(entry.Created)); err != nil {
			return fmt.Errorf("append plugin log: %w", err)
		}
		plugins[entry.PluginID] = struct{}{}
	}
	for pluginID := range plugins {
		if _, err := tx.ExecContext(ctx, `DELETE FROM plugin_logs WHERE plugin_id = ? AND id NOT IN
            (SELECT id FROM plugin_logs WHERE plugin_id = ? ORDER BY id DESC LIMIT 1000)`, pluginID, pluginID); err != nil {
			return fmt.Errorf("retain plugin logs: %w", err)
		}
	}
	return tx.Commit()
}

// Logs returns the newest supervision entries first.
func (r *PluginRepository) Logs(ctx context.Context, id string, limit int) ([]domain.LogEntry, error) {
	if _, err := r.Get(ctx, id); err != nil {
		return nil, err
	}
	rows, err := r.database.connection.QueryContext(ctx, `SELECT id, plugin_id, level, message, created_at
        FROM plugin_logs WHERE plugin_id = ? ORDER BY id DESC LIMIT ?`, id, limit)
	if err != nil {
		return nil, fmt.Errorf("list plugin logs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := []domain.LogEntry{}
	for rows.Next() {
		var entry domain.LogEntry
		var created string
		if err := rows.Scan(&entry.ID, &entry.PluginID, &entry.Level, &entry.Message, &created); err != nil {
			return nil, err
		}
		entry.Created, _ = parseTime(created)
		result = append(result, entry)
	}
	return result, rows.Err()
}
