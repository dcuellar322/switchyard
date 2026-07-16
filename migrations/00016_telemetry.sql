-- +goose Up
CREATE TABLE telemetry_settings (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    enabled INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0, 1)),
    endpoint TEXT NOT NULL DEFAULT '',
    installation_id TEXT NOT NULL DEFAULT '',
    last_sent_at TEXT,
    last_error TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL
);

INSERT INTO telemetry_settings(singleton, enabled, endpoint, installation_id, updated_at)
VALUES (1, 0, '', '', strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));

CREATE TABLE telemetry_counters (
    name TEXT PRIMARY KEY,
    value INTEGER NOT NULL DEFAULT 0 CHECK (value >= 0),
    updated_at TEXT NOT NULL
);

CREATE TABLE telemetry_audit_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_type TEXT NOT NULL,
    actor_type TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    detail TEXT NOT NULL DEFAULT '',
    occurred_at TEXT NOT NULL
);

UPDATE system_health SET schema_version = 16 WHERE singleton = 1;

-- +goose Down
UPDATE system_health SET schema_version = 15 WHERE singleton = 1;
DROP TABLE telemetry_audit_events;
DROP TABLE telemetry_counters;
DROP TABLE telemetry_settings;
