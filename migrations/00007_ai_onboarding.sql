-- +goose Up
CREATE TABLE manifest_ai_runs (
    operation_id TEXT PRIMARY KEY REFERENCES operations(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    source_proposal_id TEXT NOT NULL REFERENCES manifest_proposals(id) ON DELETE CASCADE,
    result_proposal_id TEXT REFERENCES manifest_proposals(id) ON DELETE SET NULL,
    provider TEXT NOT NULL,
    model TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL CHECK (state IN ('running', 'succeeded', 'failed', 'cancelled')),
    bundle_json TEXT NOT NULL,
    bundle_sha256 TEXT NOT NULL,
    limits_json TEXT NOT NULL,
    fields_json TEXT NOT NULL DEFAULT '[]',
    conflicts_json TEXT NOT NULL DEFAULT '[]',
    warnings_json TEXT NOT NULL DEFAULT '[]',
    dry_run_json TEXT NOT NULL DEFAULT '{}',
    usage_json TEXT NOT NULL DEFAULT '{}',
    error_code TEXT,
    error_message TEXT,
    started_at TEXT NOT NULL,
    finished_at TEXT
);

CREATE INDEX manifest_ai_runs_proposal_idx
    ON manifest_ai_runs (source_proposal_id, started_at DESC);

UPDATE system_health SET schema_version = 7 WHERE singleton = 1;

-- +goose Down
DROP TABLE manifest_ai_runs;
UPDATE system_health SET schema_version = 6 WHERE singleton = 1;
