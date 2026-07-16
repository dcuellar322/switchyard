-- +goose Up
CREATE TABLE terminal_sessions (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    environment_id TEXT NOT NULL DEFAULT '',
    kind TEXT NOT NULL CHECK (kind IN ('shell', 'service', 'database', 'agent', 'action')),
    display_name TEXT NOT NULL,
    owner_type TEXT NOT NULL,
    owner_id TEXT NOT NULL,
    provider TEXT NOT NULL DEFAULT '',
    service_id TEXT NOT NULL DEFAULT '',
    action_id TEXT NOT NULL DEFAULT '',
    working_directory TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('starting', 'active', 'exited', 'terminated', 'expired', 'interrupted', 'failed')),
    persistence_policy TEXT NOT NULL CHECK (persistence_policy = 'detach_until_idle_timeout'),
    capture_policy TEXT NOT NULL CHECK (capture_policy = 'user_visible_terminal_output_only'),
    output_bytes INTEGER NOT NULL DEFAULT 0 CHECK (output_bytes >= 0),
    output_truncated INTEGER NOT NULL DEFAULT 0 CHECK (output_truncated IN (0, 1)),
    last_output_at TEXT,
    exit_code INTEGER,
    created_at TEXT NOT NULL,
    last_attached_at TEXT,
    detached_at TEXT,
    finished_at TEXT,
    error_code TEXT NOT NULL DEFAULT ''
);

CREATE INDEX terminal_sessions_project_created_idx
    ON terminal_sessions(project_id, created_at DESC, id);
CREATE INDEX terminal_sessions_owner_status_idx
    ON terminal_sessions(owner_type, owner_id, status, created_at DESC);

CREATE TABLE terminal_session_audits (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES terminal_sessions(id) ON DELETE CASCADE,
    event TEXT NOT NULL,
    actor_type TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    detail_json TEXT NOT NULL CHECK (json_valid(detail_json)),
    occurred_at TEXT NOT NULL
);

CREATE INDEX terminal_session_audits_session_idx
    ON terminal_session_audits(session_id, occurred_at, id);

UPDATE system_health SET schema_version = 11 WHERE singleton = 1;

-- +goose Down
UPDATE system_health SET schema_version = 10 WHERE singleton = 1;
DROP TABLE terminal_session_audits;
DROP TABLE terminal_sessions;
