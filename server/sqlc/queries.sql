-- name: ListRunsByUser :many
SELECT id, user_id, title, goal, requested_mode, knowledge_connection_ids,
       effective_mode, status, latest_artifact_id, error_message, created_at,
       updated_at
FROM runs
WHERE user_id = $1
ORDER BY updated_at DESC;

-- name: ListKnowledgeConnectionsByUser :many
SELECT id, user_id, provider, name, group_login, namespace, sync_enabled,
       last_synced_at, created_at, updated_at
FROM knowledge_connections
WHERE user_id = $1
ORDER BY updated_at DESC;
