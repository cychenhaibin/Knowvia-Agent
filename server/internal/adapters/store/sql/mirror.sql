-- name: CreateMirrorTask :execrows
INSERT INTO mirror_tasks (
	id, kind, user_id, resource_id, status, payload, attempts, last_error,
	next_retry_at, created_at, updated_at, completed_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);

-- name: UpdateMirrorTask :execrows
UPDATE mirror_tasks
SET kind = $2,
    user_id = $3,
    resource_id = $4,
    status = $5,
    payload = $6,
    attempts = $7,
    last_error = $8,
    next_retry_at = $9,
    created_at = $10,
    updated_at = $11,
    completed_at = $12
WHERE id = $1;

-- name: GetMirrorTask :one
SELECT id, kind, user_id, resource_id, status, payload, attempts, last_error,
       next_retry_at, created_at, updated_at, completed_at
FROM mirror_tasks
WHERE id = $1;

-- name: ListDueMirrorTasks :many
SELECT id, kind, user_id, resource_id, status, payload, attempts, last_error,
       next_retry_at, created_at, updated_at, completed_at
FROM mirror_tasks
WHERE status = $1 AND next_retry_at <= $2
ORDER BY next_retry_at ASC, created_at ASC
LIMIT $3;
