-- +goose Up
CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    slug TEXT NOT NULL,
    display_name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    trust_state TEXT NOT NULL CHECK (trust_state IN ('pending', 'trusted', 'rejected')),
    primary_location TEXT NOT NULL UNIQUE,
    manifest_revision INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX projects_slug_idx ON projects (slug);

CREATE TABLE project_locations (
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    path TEXT NOT NULL UNIQUE,
    is_primary INTEGER NOT NULL CHECK (is_primary IN (0, 1)),
    PRIMARY KEY (project_id, path)
);

CREATE TABLE project_tags (
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    tag TEXT NOT NULL,
    PRIMARY KEY (project_id, tag)
);

CREATE TABLE manifest_proposals (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    scanner_version TEXT NOT NULL,
    schema_version TEXT NOT NULL,
    candidate_json TEXT NOT NULL,
    confidence_json TEXT NOT NULL,
    unresolved_json TEXT NOT NULL,
    validation_json TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('proposed', 'accepted', 'rejected', 'superseded')),
    created_at TEXT NOT NULL
);

CREATE INDEX manifest_proposals_project_idx ON manifest_proposals (project_id, created_at DESC);

CREATE TABLE discovery_evidence (
    id TEXT PRIMARY KEY,
    proposal_id TEXT NOT NULL REFERENCES manifest_proposals(id) ON DELETE CASCADE,
    scanner TEXT NOT NULL,
    kind TEXT NOT NULL,
    source_path TEXT NOT NULL,
    start_line INTEGER NOT NULL,
    end_line INTEGER NOT NULL,
    confidence REAL NOT NULL,
    data_json TEXT NOT NULL,
    warnings_json TEXT NOT NULL
);

CREATE INDEX discovery_evidence_proposal_idx ON discovery_evidence (proposal_id, source_path, start_line);

CREATE TABLE manifest_snapshots (
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    revision INTEGER NOT NULL,
    proposal_id TEXT NOT NULL REFERENCES manifest_proposals(id),
    manifest_json TEXT NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (project_id, revision)
);

UPDATE system_health SET schema_version = 3 WHERE singleton = 1;

-- +goose Down
UPDATE system_health SET schema_version = 2 WHERE singleton = 1;
DROP TABLE manifest_snapshots;
DROP TABLE discovery_evidence;
DROP TABLE manifest_proposals;
DROP TABLE project_tags;
DROP TABLE project_locations;
DROP TABLE projects;
