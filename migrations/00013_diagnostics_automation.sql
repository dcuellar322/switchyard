-- +goose Up
CREATE TABLE diagnoses (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    provider TEXT NOT NULL DEFAULT '',
    bundle_sha256 TEXT NOT NULL,
    diagnosis_json TEXT NOT NULL CHECK (json_valid(diagnosis_json)),
    generated_at TEXT NOT NULL
);

CREATE INDEX diagnoses_project_generated_idx
    ON diagnoses(project_id, generated_at DESC, id DESC);

CREATE TABLE diagnostic_feedback (
    id TEXT PRIMARY KEY,
    diagnosis_id TEXT NOT NULL REFERENCES diagnoses(id) ON DELETE CASCADE,
    hypothesis_id TEXT NOT NULL,
    verdict TEXT NOT NULL CHECK (verdict IN ('accurate', 'false_positive')),
    note TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE INDEX diagnostic_feedback_diagnosis_idx
    ON diagnostic_feedback(diagnosis_id, created_at, id);

CREATE TABLE automation_recipes (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    trigger_code TEXT NOT NULL,
    action_id TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0, 1)),
    cooldown_seconds INTEGER NOT NULL CHECK (cooldown_seconds BETWEEN 60 AND 86400),
    max_runs_per_day INTEGER NOT NULL CHECK (max_runs_per_day BETWEEN 1 AND 20),
    last_run_at TEXT,
    runs_today INTEGER NOT NULL DEFAULT 0 CHECK (runs_today >= 0),
    runs_day TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX automation_recipes_project_idx
    ON automation_recipes(project_id, enabled DESC, name COLLATE NOCASE, id);

CREATE TABLE diagnostic_notifications (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    code TEXT NOT NULL,
    title TEXT NOT NULL,
    detail TEXT NOT NULL,
    occurrences INTEGER NOT NULL DEFAULT 1 CHECK (occurrences > 0),
    first_seen_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    acknowledged_at TEXT,
    UNIQUE(project_id, code)
);

CREATE INDEX diagnostic_notifications_state_idx
    ON diagnostic_notifications(acknowledged_at, last_seen_at DESC, id);

UPDATE system_health SET schema_version = 13 WHERE singleton = 1;

-- +goose Down
UPDATE system_health SET schema_version = 12 WHERE singleton = 1;
DROP TABLE diagnostic_notifications;
DROP TABLE automation_recipes;
DROP TABLE diagnostic_feedback;
DROP TABLE diagnoses;
