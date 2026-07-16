-- +goose Up
CREATE TABLE fleet_machines (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    endpoint TEXT NOT NULL UNIQUE,
    certificate_fingerprint TEXT NOT NULL,
    ca_certificate_path TEXT NOT NULL,
    client_certificate_path TEXT NOT NULL,
    client_key_path TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    capabilities_json TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(capabilities_json)),
    grants_json TEXT NOT NULL CHECK (json_valid(grants_json)),
    state TEXT NOT NULL CHECK (state IN ('pending', 'online', 'degraded', 'offline', 'disabled')),
    peer_id TEXT NOT NULL DEFAULT '',
    peer_version TEXT NOT NULL DEFAULT '',
    os TEXT NOT NULL DEFAULT '',
    architecture TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    last_seen_at TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX fleet_machines_peer_idx ON fleet_machines(peer_id) WHERE peer_id <> '';
CREATE INDEX fleet_machines_state_idx ON fleet_machines(enabled DESC, state, name COLLATE NOCASE, id);

CREATE TABLE fleet_snapshots (
    machine_id TEXT PRIMARY KEY REFERENCES fleet_machines(id) ON DELETE CASCADE,
    snapshot_json TEXT NOT NULL CHECK (json_valid(snapshot_json)),
    observed_at TEXT NOT NULL
);

CREATE TABLE fleet_audit_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    machine_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    actor_type TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    request_id TEXT NOT NULL DEFAULT '',
    detail TEXT NOT NULL DEFAULT '',
    occurred_at TEXT NOT NULL
);

CREATE INDEX fleet_audit_machine_idx ON fleet_audit_events(machine_id, occurred_at DESC, id DESC);

UPDATE system_health SET schema_version = 14 WHERE singleton = 1;

-- +goose Down
UPDATE system_health SET schema_version = 13 WHERE singleton = 1;
DROP TABLE fleet_audit_events;
DROP TABLE fleet_snapshots;
DROP TABLE fleet_machines;
