-- +goose Up
CREATE TABLE port_reservations (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    project_name TEXT NOT NULL,
    service_id TEXT NOT NULL DEFAULT '',
    port_id TEXT NOT NULL,
    host TEXT NOT NULL,
    port INTEGER NOT NULL CHECK (port BETWEEN 1 AND 65535),
    target INTEGER NOT NULL DEFAULT 0 CHECK (target BETWEEN 0 AND 65535),
    protocol TEXT NOT NULL CHECK (protocol IN ('tcp', 'udp')),
    source TEXT NOT NULL CHECK (source IN ('manifest', 'manual', 'worktree')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(project_id, port_id, source)
);
CREATE INDEX port_reservations_lookup ON port_reservations(port, protocol, host);

CREATE TABLE action_audit (
    id TEXT PRIMARY KEY,
    operation_id TEXT NOT NULL REFERENCES operations(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    action_id TEXT NOT NULL,
    action_type TEXT NOT NULL,
    risk TEXT NOT NULL,
    actor_type TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    state TEXT NOT NULL,
    working_directory TEXT NOT NULL,
    started_at TEXT NOT NULL,
    finished_at TEXT,
    error_code TEXT
);
CREATE UNIQUE INDEX action_audit_operation ON action_audit(operation_id);
CREATE INDEX action_audit_project_started ON action_audit(project_id, started_at DESC);

UPDATE system_health SET schema_version = 6 WHERE singleton = 1;

-- +goose Down
DROP TABLE action_audit;
DROP TABLE port_reservations;
UPDATE system_health SET schema_version = 5 WHERE singleton = 1;
