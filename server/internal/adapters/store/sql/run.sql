-- name: CreateRun :execrows
INSERT INTO runs (
	id, user_id, title, goal, requested_mode, knowledge_connection_ids,
	skill_installation_id, skill_snapshot_id, effective_mode, status,
	latest_artifact_id, error_message, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14);

-- name: GetRun :one
SELECT id, user_id, title, goal, requested_mode, knowledge_connection_ids,
       skill_installation_id, skill_snapshot_id, effective_mode, status,
       latest_artifact_id, error_message, created_at, updated_at
FROM runs
WHERE id = $1 AND user_id = $2;

-- name: GetRunById :one
SELECT id, user_id, title, goal, requested_mode, knowledge_connection_ids,
       skill_installation_id, skill_snapshot_id, effective_mode, status,
       latest_artifact_id, error_message, created_at, updated_at
FROM runs
WHERE id = $1;

-- name: UpdateRun :execrows
UPDATE runs
SET user_id = $2,
    title = $3,
    goal = $4,
    requested_mode = $5,
    knowledge_connection_ids = $6,
    skill_installation_id = $7,
    skill_snapshot_id = $8,
    effective_mode = $9,
    status = $10,
    latest_artifact_id = $11,
    error_message = $12,
    created_at = $13,
    updated_at = $14
WHERE id = $1;

-- name: ListRuns :many
SELECT id, user_id, title, goal, requested_mode, knowledge_connection_ids,
       skill_installation_id, skill_snapshot_id, effective_mode, status,
       latest_artifact_id, error_message, created_at, updated_at
FROM runs
WHERE user_id = $1
ORDER BY updated_at DESC;

-- name: UpsertRunStep :execrows
INSERT INTO run_steps (id, run_id, kind, label, status, summary, started_at, finished_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (id) DO UPDATE SET
	kind = EXCLUDED.kind,
	label = EXCLUDED.label,
	status = EXCLUDED.status,
	summary = EXCLUDED.summary,
	started_at = EXCLUDED.started_at,
	finished_at = EXCLUDED.finished_at,
	created_at = EXCLUDED.created_at;

-- name: ListRunSteps :many
SELECT id, run_id, kind, label, status, summary, started_at, finished_at, created_at
FROM run_steps
WHERE run_id = $1
ORDER BY created_at ASC;

-- name: SaveArtifact :execrows
INSERT INTO run_artifacts (id, run_id, kind, content_markdown, version, created_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListArtifacts :many
SELECT id, run_id, kind, content_markdown, version, created_at
FROM run_artifacts
WHERE run_id = $1
ORDER BY created_at ASC;

-- name: DeleteRunSources :execrows
DELETE FROM run_sources
WHERE run_id = $1;

-- name: InsertRunSource :execrows
INSERT INTO run_sources (id, run_id, provider, connection_id, document_id, chunk_id, title, repo, url, snippet, score, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);

-- name: ListRunSources :many
SELECT id, run_id, provider, connection_id, document_id, chunk_id, title, repo, url, snippet, score, created_at
FROM run_sources
WHERE run_id = $1
ORDER BY created_at ASC;
