-- +goose Up
CREATE TABLE operations (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN (
        'queued', 'running', 'succeeded', 'failed', 'cancelled', 'partially_succeeded'
    )),
    idempotency_key TEXT NOT NULL,
    input_json TEXT NOT NULL DEFAULT '{}',
    error_code TEXT,
    error_message TEXT,
    cancellation_requested INTEGER NOT NULL DEFAULT 0 CHECK (cancellation_requested IN (0, 1)),
    requested_at TEXT NOT NULL,
    started_at TEXT,
    finished_at TEXT,
    updated_at TEXT NOT NULL,
    UNIQUE (project_id, idempotency_key)
);

CREATE INDEX operations_project_requested_idx
    ON operations (project_id, requested_at DESC);
CREATE INDEX operations_state_requested_idx
    ON operations (state, requested_at);

CREATE TABLE operation_steps (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    operation_id TEXT NOT NULL REFERENCES operations(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('running', 'succeeded', 'failed', 'cancelled')),
    message TEXT NOT NULL DEFAULT '',
    occurred_at TEXT NOT NULL
);

CREATE INDEX operation_steps_operation_idx
    ON operation_steps (operation_id, id);

CREATE TABLE event_journal (
    sequence INTEGER PRIMARY KEY AUTOINCREMENT,
    id TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL,
    occurred_at TEXT NOT NULL,
    project_id TEXT,
    operation_id TEXT,
    payload_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX event_journal_operation_idx
    ON event_journal (operation_id, sequence);

CREATE TABLE audit_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_type TEXT NOT NULL,
    actor_type TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    project_id TEXT,
    operation_id TEXT,
    idempotency_key TEXT,
    detail_json TEXT NOT NULL DEFAULT '{}',
    occurred_at TEXT NOT NULL
);

CREATE INDEX audit_events_occurred_idx ON audit_events (occurred_at DESC);
CREATE INDEX audit_events_operation_idx ON audit_events (operation_id, id);

UPDATE system_health SET schema_version = 2 WHERE singleton = 1;

-- +goose Down
UPDATE system_health SET schema_version = 1 WHERE singleton = 1;
DROP TABLE audit_events;
DROP TABLE event_journal;
DROP TABLE operation_steps;
DROP TABLE operations;
