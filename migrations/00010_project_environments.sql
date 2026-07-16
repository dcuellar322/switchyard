-- +goose Up
CREATE TABLE project_environments (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    path TEXT NOT NULL UNIQUE,
    head TEXT NOT NULL DEFAULT '',
    branch TEXT NOT NULL DEFAULT '',
    detached INTEGER NOT NULL CHECK (detached IN (0, 1)),
    bare INTEGER NOT NULL CHECK (bare IN (0, 1)),
    locked INTEGER NOT NULL CHECK (locked IN (0, 1)),
    is_primary INTEGER NOT NULL CHECK (is_primary IN (0, 1)),
    availability TEXT NOT NULL CHECK (availability IN ('available', 'unavailable')),
    unavailable_reason TEXT NOT NULL DEFAULT '',
    runtime_state TEXT NOT NULL CHECK (runtime_state IN ('registered', 'active', 'inactive', 'unavailable')),
    hostname TEXT NOT NULL,
    target TEXT NOT NULL DEFAULT '',
    compose_project_name TEXT NOT NULL UNIQUE,
    port_lease_namespace TEXT NOT NULL UNIQUE,
    port_offset INTEGER NOT NULL CHECK (port_offset BETWEEN 1 AND 65535),
    registered_at TEXT NOT NULL,
    last_observed_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX project_environments_project_idx
    ON project_environments(project_id, is_primary DESC, name, id);
CREATE INDEX project_environments_hostname_idx
    ON project_environments(hostname, runtime_state);

CREATE TABLE environment_port_leases (
    environment_id TEXT NOT NULL REFERENCES project_environments(id) ON DELETE CASCADE,
    port_id TEXT NOT NULL,
    protocol TEXT NOT NULL CHECK (protocol IN ('tcp', 'udp')),
    target_port INTEGER NOT NULL CHECK (target_port BETWEEN 1 AND 65535),
    host_port INTEGER NOT NULL CHECK (host_port BETWEEN 1 AND 65535),
    PRIMARY KEY(environment_id, port_id),
    UNIQUE(protocol, host_port)
);

UPDATE system_health SET schema_version = 10 WHERE singleton = 1;

-- +goose Down
UPDATE system_health SET schema_version = 9 WHERE singleton = 1;
DROP TABLE environment_port_leases;
DROP TABLE project_environments;
