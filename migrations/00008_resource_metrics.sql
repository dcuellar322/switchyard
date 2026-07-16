-- +goose Up
CREATE TABLE resource_metric_samples (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    service_id TEXT NOT NULL DEFAULT '',
    sampled_at TEXT NOT NULL,
    resolution_seconds INTEGER NOT NULL CHECK (resolution_seconds IN (0, 60, 900)),
    sample_count INTEGER NOT NULL CHECK (sample_count > 0),
    cpu_percent REAL NOT NULL CHECK (cpu_percent >= 0),
    cpu_max_percent REAL NOT NULL CHECK (cpu_max_percent >= 0),
    cpu_available INTEGER NOT NULL CHECK (cpu_available IN (0, 1)),
    memory_bytes INTEGER NOT NULL CHECK (memory_bytes >= 0),
    memory_max_bytes INTEGER NOT NULL CHECK (memory_max_bytes >= 0),
    memory_limit INTEGER NOT NULL CHECK (memory_limit >= 0),
    memory_available INTEGER NOT NULL CHECK (memory_available IN (0, 1)),
    network_rx_bytes INTEGER NOT NULL CHECK (network_rx_bytes >= 0),
    network_tx_bytes INTEGER NOT NULL CHECK (network_tx_bytes >= 0),
    network_available INTEGER NOT NULL CHECK (network_available IN (0, 1)),
    disk_read_bytes INTEGER NOT NULL CHECK (disk_read_bytes >= 0),
    disk_write_bytes INTEGER NOT NULL CHECK (disk_write_bytes >= 0),
    disk_available INTEGER NOT NULL CHECK (disk_available IN (0, 1)),
    process_count INTEGER NOT NULL CHECK (process_count >= 0),
    restart_count INTEGER NOT NULL CHECK (restart_count >= 0),
    health_latency_ms INTEGER NOT NULL CHECK (health_latency_ms >= 0),
    health_available INTEGER NOT NULL CHECK (health_available IN (0, 1)),
    storage_bytes INTEGER,
    storage_classification TEXT NOT NULL CHECK (storage_classification IN ('exclusive', 'shared', 'estimated', 'unknown')),
    partial INTEGER NOT NULL CHECK (partial IN (0, 1)),
    UNIQUE (project_id, service_id, resolution_seconds, sampled_at)
);
CREATE INDEX resource_metric_history_idx
    ON resource_metric_samples (project_id, service_id, resolution_seconds, sampled_at, id);
CREATE INDEX resource_metric_retention_idx
    ON resource_metric_samples (resolution_seconds, sampled_at, id);

UPDATE system_health SET schema_version = 8 WHERE singleton = 1;

-- +goose Down
UPDATE system_health SET schema_version = 7 WHERE singleton = 1;
DROP TABLE resource_metric_samples;
