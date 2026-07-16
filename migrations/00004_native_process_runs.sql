-- +goose Up
CREATE TABLE runs (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    service_id TEXT NOT NULL,
    runtime_driver TEXT NOT NULL,
    origin TEXT NOT NULL CHECK (origin IN ('switchyard', 'external')),
    started_at TEXT NOT NULL,
    ended_at TEXT,
    exit_code INTEGER,
    termination_reason TEXT NOT NULL DEFAULT '',
    identity_fingerprint TEXT NOT NULL,
    restart_count INTEGER NOT NULL DEFAULT 0 CHECK (restart_count >= 0)
);

CREATE INDEX runs_project_active_idx ON runs (project_id, ended_at, service_id);

CREATE TABLE run_processes (
    run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    pid INTEGER NOT NULL,
    process_group_id INTEGER NOT NULL,
    executable TEXT NOT NULL,
    started_at TEXT NOT NULL,
    working_directory TEXT NOT NULL,
    identity_fingerprint TEXT NOT NULL,
    observed_at TEXT NOT NULL,
    PRIMARY KEY (run_id, pid, started_at)
);

CREATE INDEX run_processes_group_idx ON run_processes (process_group_id, run_id);

UPDATE system_health SET schema_version = 4 WHERE singleton = 1;

-- +goose Down
UPDATE system_health SET schema_version = 3 WHERE singleton = 1;
DROP TABLE run_processes;
DROP TABLE runs;
