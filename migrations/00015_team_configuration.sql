-- +goose Up
CREATE TABLE team_publishers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    public_key TEXT NOT NULL,
    trusted_at TEXT NOT NULL
);

CREATE TABLE team_bundles (
    id TEXT PRIMARY KEY,
    schema_version TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('project-template', 'policy-pack', 'plugin-registry', 'enterprise-config')),
    metadata_json TEXT NOT NULL CHECK (json_valid(metadata_json)),
    payload_json TEXT NOT NULL CHECK (json_valid(payload_json)),
    signature_json TEXT NOT NULL CHECK (json_valid(signature_json)),
    publisher_id TEXT NOT NULL REFERENCES team_publishers(id),
    installed_at TEXT NOT NULL
);

CREATE INDEX team_bundles_kind_idx ON team_bundles(kind, installed_at DESC, id);
CREATE INDEX team_bundles_publisher_idx ON team_bundles(publisher_id, id);

CREATE TABLE team_audit_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_type TEXT NOT NULL,
    actor_type TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    subject_id TEXT NOT NULL,
    detail TEXT NOT NULL DEFAULT '',
    occurred_at TEXT NOT NULL
);

CREATE INDEX team_audit_subject_idx ON team_audit_events(subject_id, occurred_at DESC, id DESC);

UPDATE system_health SET schema_version = 15 WHERE singleton = 1;

-- +goose Down
UPDATE system_health SET schema_version = 14 WHERE singleton = 1;
DROP TABLE team_audit_events;
DROP TABLE team_bundles;
DROP TABLE team_publishers;
