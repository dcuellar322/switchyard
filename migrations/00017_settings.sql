-- +goose Up
CREATE TABLE settings (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    revision INTEGER NOT NULL CHECK (revision >= 1),
    document_json TEXT NOT NULL CHECK (json_valid(document_json)),
    updated_at TEXT NOT NULL
);

CREATE TABLE settings_audit_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    revision INTEGER NOT NULL,
    actor_type TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    sections_json TEXT NOT NULL CHECK (json_valid(sections_json)),
    occurred_at TEXT NOT NULL
);

CREATE INDEX settings_audit_revision_idx
ON settings_audit_events(revision DESC, id DESC);

UPDATE system_health SET schema_version = 17 WHERE singleton = 1;

-- +goose Down
UPDATE system_health SET schema_version = 16 WHERE singleton = 1;
DROP TABLE settings_audit_events;
DROP TABLE settings;
