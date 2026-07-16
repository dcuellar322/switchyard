-- +goose Up
CREATE TABLE system_health (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    schema_version INTEGER NOT NULL CHECK (schema_version >= 0),
    initialized_at TEXT NOT NULL
);

INSERT INTO system_health (singleton, schema_version, initialized_at)
VALUES (1, 1, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));

-- +goose Down
DROP TABLE system_health;
