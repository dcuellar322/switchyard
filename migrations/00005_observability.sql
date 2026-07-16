-- +goose Up
ALTER TABLE runs ADD COLUMN operation_id TEXT;

CREATE TABLE health_samples (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    service_id TEXT NOT NULL,
    check_id TEXT NOT NULL,
    check_type TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('healthy', 'unhealthy', 'unknown')),
    severity TEXT NOT NULL CHECK (severity IN ('info', 'warning', 'critical')),
    required INTEGER NOT NULL CHECK (required IN (0, 1)),
    latency_ms INTEGER NOT NULL CHECK (latency_ms >= 0),
    message TEXT NOT NULL,
    observed_at TEXT NOT NULL
);
CREATE INDEX health_samples_latest_idx
    ON health_samples (project_id, service_id, check_id, observed_at DESC, id DESC);

CREATE TABLE log_segments (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    service_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    operation_id TEXT,
    path TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL,
    closed_at TEXT,
    first_timestamp TEXT,
    last_timestamp TEXT,
    first_sequence INTEGER,
    last_sequence INTEGER,
    entry_count INTEGER NOT NULL DEFAULT 0 CHECK (entry_count >= 0),
    size_bytes INTEGER NOT NULL DEFAULT 0 CHECK (size_bytes >= 0),
    sha256 TEXT
);
CREATE INDEX log_segments_project_time_idx ON log_segments (project_id, created_at DESC);

CREATE TABLE log_entries (
    sequence INTEGER PRIMARY KEY AUTOINCREMENT,
    digest TEXT NOT NULL UNIQUE,
    segment_id TEXT NOT NULL REFERENCES log_segments(id) ON DELETE CASCADE,
    line_number INTEGER NOT NULL CHECK (line_number >= 1),
    project_id TEXT NOT NULL,
    service_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    operation_id TEXT,
    occurred_at TEXT NOT NULL
);
CREATE INDEX log_entries_query_idx ON log_entries (project_id, service_id, occurred_at DESC, sequence DESC);
CREATE INDEX log_entries_operation_idx ON log_entries (operation_id, sequence);

UPDATE system_health SET schema_version = 5 WHERE singleton = 1;

-- +goose Down
UPDATE system_health SET schema_version = 4 WHERE singleton = 1;
DROP TABLE log_entries;
DROP TABLE log_segments;
DROP TABLE health_samples;
ALTER TABLE runs DROP COLUMN operation_id;
