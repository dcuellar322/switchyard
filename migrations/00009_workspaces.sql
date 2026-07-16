-- +goose Up
CREATE TABLE workspaces (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    default_failure_policy TEXT NOT NULL CHECK (default_failure_policy IN ('rollback', 'continue')),
    default_profile_id TEXT NOT NULL DEFAULT '',
    revision INTEGER NOT NULL CHECK (revision > 0),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX workspaces_name_idx ON workspaces (name COLLATE NOCASE, id);

CREATE TABLE workspace_projects (
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('application', 'dependency', 'tooling')),
    sort_order INTEGER NOT NULL CHECK (sort_order >= 0),
    health_gate INTEGER NOT NULL CHECK (health_gate IN (0, 1)),
    health_timeout_seconds INTEGER NOT NULL CHECK (health_timeout_seconds >= 0),
    PRIMARY KEY (workspace_id, project_id)
);
CREATE INDEX workspace_projects_project_idx ON workspace_projects (project_id, workspace_id);

CREATE TABLE workspace_dependencies (
    workspace_id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    depends_on_project_id TEXT NOT NULL,
    PRIMARY KEY (workspace_id, project_id, depends_on_project_id),
    FOREIGN KEY (workspace_id, project_id)
        REFERENCES workspace_projects(workspace_id, project_id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, depends_on_project_id)
        REFERENCES workspace_projects(workspace_id, project_id) ON DELETE CASCADE,
    CHECK (project_id <> depends_on_project_id)
);
CREATE INDEX workspace_dependencies_target_idx
    ON workspace_dependencies (workspace_id, depends_on_project_id, project_id);

CREATE TABLE workspace_profiles (
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    max_parallel INTEGER NOT NULL CHECK (max_parallel BETWEEN 1 AND 64),
    low_memory INTEGER NOT NULL CHECK (low_memory IN (0, 1)),
    memory_budget_bytes INTEGER NOT NULL CHECK (memory_budget_bytes >= 0),
    PRIMARY KEY (workspace_id, id)
);

CREATE TABLE workspace_profile_projects (
    workspace_id TEXT NOT NULL,
    profile_id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    sort_order INTEGER NOT NULL CHECK (sort_order >= 0),
    PRIMARY KEY (workspace_id, profile_id, project_id),
    FOREIGN KEY (workspace_id, profile_id)
        REFERENCES workspace_profiles(workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, project_id)
        REFERENCES workspace_projects(workspace_id, project_id) ON DELETE CASCADE
);

CREATE TABLE workspace_recipes (
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    id TEXT NOT NULL,
    name TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('open_url', 'open_terminal', 'open_editor', 'start_agent')),
    project_id TEXT,
    target TEXT NOT NULL DEFAULT '',
    arguments_json TEXT NOT NULL DEFAULT '[]',
    sort_order INTEGER NOT NULL CHECK (sort_order >= 0),
    PRIMARY KEY (workspace_id, id),
    FOREIGN KEY (workspace_id, project_id)
        REFERENCES workspace_projects(workspace_id, project_id) ON DELETE CASCADE
);

CREATE TABLE workspace_runs (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('start', 'stop')),
    state TEXT NOT NULL CHECK (state IN ('running', 'succeeded', 'partially_succeeded', 'failed', 'cancelled')),
    failure_policy TEXT NOT NULL CHECK (failure_policy IN ('rollback', 'continue')),
    profile_id TEXT NOT NULL DEFAULT '',
    remove_data INTEGER NOT NULL CHECK (remove_data IN (0, 1)),
    error_message TEXT NOT NULL DEFAULT '',
    started_at TEXT NOT NULL,
    finished_at TEXT
);
CREATE INDEX workspace_runs_workspace_started_idx
    ON workspace_runs (workspace_id, started_at DESC, id DESC);

CREATE TABLE workspace_run_projects (
    run_id TEXT NOT NULL REFERENCES workspace_runs(id) ON DELETE CASCADE,
    project_id TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('application', 'dependency', 'tooling')),
    state TEXT NOT NULL CHECK (state IN (
        'queued', 'blocked', 'starting', 'checking_health', 'running', 'start_failed',
        'stopping', 'stopped', 'stop_failed', 'rolling_back', 'rolled_back',
        'rollback_failed', 'cancelled'
    )),
    message TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL CHECK (sort_order >= 0),
    started_at TEXT,
    finished_at TEXT,
    PRIMARY KEY (run_id, project_id)
);

ALTER TABLE operations ADD COLUMN workspace_id TEXT REFERENCES workspaces(id) ON DELETE SET NULL;
ALTER TABLE audit_events ADD COLUMN workspace_id TEXT REFERENCES workspaces(id) ON DELETE SET NULL;
CREATE INDEX operations_workspace_requested_idx
    ON operations (workspace_id, requested_at DESC) WHERE workspace_id IS NOT NULL;
CREATE INDEX audit_events_workspace_idx
    ON audit_events (workspace_id, occurred_at DESC) WHERE workspace_id IS NOT NULL;

UPDATE system_health SET schema_version = 9 WHERE singleton = 1;

-- +goose Down
UPDATE system_health SET schema_version = 8 WHERE singleton = 1;
DROP INDEX audit_events_workspace_idx;
DROP INDEX operations_workspace_requested_idx;
ALTER TABLE audit_events DROP COLUMN workspace_id;
ALTER TABLE operations DROP COLUMN workspace_id;
DROP TABLE workspace_run_projects;
DROP TABLE workspace_runs;
DROP TABLE workspace_recipes;
DROP TABLE workspace_profile_projects;
DROP TABLE workspace_profiles;
DROP TABLE workspace_dependencies;
DROP TABLE workspace_projects;
DROP TABLE workspaces;
