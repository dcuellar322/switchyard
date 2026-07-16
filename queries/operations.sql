-- name: CreateOperation :execrows
INSERT INTO operations (
    id, project_id, kind, state, idempotency_key, input_json,
    requested_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (project_id, idempotency_key) DO NOTHING;

-- name: GetOperation :one
SELECT * FROM operations WHERE id = ?;

-- name: GetOperationByIdempotency :one
SELECT * FROM operations WHERE project_id = ? AND idempotency_key = ?;

-- name: ListRecoverableOperations :many
SELECT * FROM operations
WHERE state IN ('queued', 'running')
ORDER BY requested_at, id;

-- name: UpdateOperationState :execrows
UPDATE operations
SET state = ?, error_code = ?, error_message = ?, started_at = ?,
    finished_at = ?, updated_at = ?
WHERE id = ? AND state = ?;

-- name: RequestOperationCancellation :execrows
UPDATE operations
SET cancellation_requested = 1, updated_at = ?
WHERE id = ? AND state IN ('queued', 'running') AND cancellation_requested = 0;

-- name: CreateOperationStep :exec
INSERT INTO operation_steps (
    operation_id, name, state, message, occurred_at
) VALUES (?, ?, ?, ?, ?);

-- name: ListOperationSteps :many
SELECT * FROM operation_steps WHERE operation_id = ? ORDER BY id;

-- name: CreateAuditEvent :exec
INSERT INTO audit_events (
    event_type, actor_type, actor_id, project_id, operation_id,
    idempotency_key, detail_json, occurred_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListAuditEventsForOperation :many
SELECT * FROM audit_events WHERE operation_id = ? ORDER BY id;

-- name: CreateJournalEvent :one
INSERT INTO event_journal (
    id, type, occurred_at, project_id, operation_id, payload_json
) VALUES (?, ?, ?, ?, ?, ?)
RETURNING sequence;

-- name: ListJournalEventsAfter :many
SELECT * FROM event_journal
WHERE sequence > ?
ORDER BY sequence
LIMIT ?;
