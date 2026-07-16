-- +goose Up
CREATE TABLE plugin_registrations (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    protocol_version TEXT NOT NULL,
    manifest_path TEXT NOT NULL UNIQUE,
    executable_path TEXT NOT NULL,
    arguments_json TEXT NOT NULL CHECK (json_valid(arguments_json)),
    fingerprint TEXT NOT NULL,
    trusted_fingerprint TEXT NOT NULL DEFAULT '',
    capabilities_json TEXT NOT NULL CHECK (json_valid(capabilities_json)),
    requested_scopes_json TEXT NOT NULL CHECK (json_valid(requested_scopes_json)),
    granted_scopes_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(granted_scopes_json)),
    available INTEGER NOT NULL DEFAULT 1 CHECK (available IN (0, 1)),
    enabled INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0, 1)),
    health TEXT NOT NULL DEFAULT 'unknown' CHECK (health IN ('unknown', 'healthy', 'degraded', 'unhealthy')),
    health_message TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    discovered_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX plugin_registrations_state_idx
    ON plugin_registrations(available, enabled, name COLLATE NOCASE, id);

CREATE TABLE plugin_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    plugin_id TEXT NOT NULL REFERENCES plugin_registrations(id) ON DELETE CASCADE,
    level TEXT NOT NULL CHECK (level IN ('debug', 'info', 'warning', 'error')),
    message TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX plugin_logs_plugin_created_idx
    ON plugin_logs(plugin_id, created_at DESC, id DESC);

UPDATE system_health SET schema_version = 12 WHERE singleton = 1;

-- +goose Down
UPDATE system_health SET schema_version = 11 WHERE singleton = 1;
DROP TABLE plugin_logs;
DROP TABLE plugin_registrations;
