-- name: GetSystemHealth :one
SELECT singleton, schema_version, initialized_at
FROM system_health
WHERE singleton = 1;
