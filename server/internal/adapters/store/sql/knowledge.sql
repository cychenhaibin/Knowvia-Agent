-- name: CreateConnection :execrows
INSERT INTO knowledge_connections (
	id, user_id, provider, name, sync_enabled, last_synced_at, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: UpdateConnection :execrows
UPDATE knowledge_connections
SET user_id = $2,
    provider = $3,
    name = $4,
    sync_enabled = $5,
    last_synced_at = $6,
    created_at = $7,
    updated_at = $8
WHERE id = $1;

-- name: GetConnection :one
SELECT id, user_id, provider, name, sync_enabled, last_synced_at, created_at, updated_at
FROM knowledge_connections
WHERE id = $1 AND user_id = $2;

-- name: GetConnectionById :one
SELECT id, user_id, provider, name, sync_enabled, last_synced_at, created_at, updated_at
FROM knowledge_connections
WHERE id = $1;

-- name: ListConnections :many
SELECT id, user_id, provider, name, sync_enabled, last_synced_at, created_at, updated_at
FROM knowledge_connections
WHERE user_id = $1
ORDER BY updated_at DESC;

-- name: DeleteConnection :execrows
DELETE FROM knowledge_connections
WHERE id = $1 AND user_id = $2;

-- name: CreateSyncJob :execrows
INSERT INTO knowledge_sync_jobs (id, user_id, connection_id, status, summary, created_at, started_at, finished_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: UpdateSyncJob :execrows
UPDATE knowledge_sync_jobs
SET user_id = $2,
    connection_id = $3,
    status = $4,
    summary = $5,
    created_at = $6,
    started_at = $7,
    finished_at = $8
WHERE id = $1;

-- name: GetSyncJob :one
SELECT id, user_id, connection_id, status, summary, created_at, started_at, finished_at
FROM knowledge_sync_jobs
WHERE id = $1;

-- name: ListSyncJobs :many
SELECT id, user_id, connection_id, status, summary, created_at, started_at, finished_at
FROM knowledge_sync_jobs
WHERE user_id = $1 AND connection_id = $2
ORDER BY created_at DESC;

-- name: ListMetadata :many
SELECT id, user_id, connection_id, repo, title, doc_ref, source_url, chunk_count, updated_at, created_at
FROM knowledge_document_meta
WHERE connection_id = $1
ORDER BY repo ASC, updated_at DESC, title ASC;

-- name: DeleteMetadata :execrows
DELETE FROM knowledge_document_meta
WHERE connection_id = $1;

-- name: InsertMetadata :execrows
INSERT INTO knowledge_document_meta (
	id, user_id, connection_id, repo, title, doc_ref, source_url, chunk_count, updated_at, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: ListSourceDocuments :many
SELECT id, user_id, connection_id, provider, external_id, repo, title, doc_ref, source_url, raw_body, body_hash, source_updated_at, created_at, updated_at
FROM knowledge_source_documents
WHERE connection_id = $1
ORDER BY source_updated_at DESC, title ASC;

-- name: DeleteSourceDocuments :execrows
DELETE FROM knowledge_source_documents
WHERE connection_id = $1;

-- name: InsertSourceDocument :execrows
INSERT INTO knowledge_source_documents (
	id, user_id, connection_id, provider, external_id, repo, title, doc_ref, source_url, raw_body, body_hash, source_updated_at, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14);

-- name: ListCorpusDocuments :many
SELECT id, user_id, connection_id, repo, title, doc_ref, source_url, body_hash, updated_at, created_at
FROM knowledge_documents
WHERE connection_id = $1
ORDER BY created_at ASC, id ASC;

-- name: ListCorpusChunks :many
SELECT id, user_id, connection_id, document_id, chunk_index, content, content_hash, created_at
FROM knowledge_chunks
WHERE connection_id = $1
ORDER BY created_at ASC, document_id ASC, chunk_index ASC;

-- name: DeleteCorpusDocuments :execrows
DELETE FROM knowledge_documents
WHERE connection_id = $1;

-- name: InsertCorpusDocument :execrows
INSERT INTO knowledge_documents (
	id, user_id, connection_id, repo, title, doc_ref, source_url, body_hash, updated_at, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: InsertCorpusChunk :execrows
INSERT INTO knowledge_chunks (
	id, user_id, connection_id, document_id, chunk_index, content, content_hash, tsv, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, to_tsvector('simple', $6), $8);

-- name: SearchKnowledgeAllConnections :many
SELECT kd.connection_id, kd.id, kch.id, kc.provider, kd.title, kd.repo, COALESCE(kd.source_url, ''), kch.content
FROM knowledge_chunks kch
INNER JOIN knowledge_documents kd ON kd.id = kch.document_id
INNER JOIN knowledge_connections kc ON kc.id = kd.connection_id
WHERE kc.user_id = $1
ORDER BY kd.updated_at DESC, kch.chunk_index ASC
LIMIT 4000;

-- name: SearchKnowledgeSelectedConnections :many
SELECT kd.connection_id, kd.id, kch.id, kc.provider, kd.title, kd.repo, COALESCE(kd.source_url, ''), kch.content
FROM knowledge_chunks kch
INNER JOIN knowledge_documents kd ON kd.id = kch.document_id
INNER JOIN knowledge_connections kc ON kc.id = kd.connection_id
WHERE kc.user_id = $1
  AND kd.connection_id = ANY($2::text[])
ORDER BY kd.updated_at DESC, kch.chunk_index ASC
LIMIT 4000;

-- name: UpsertYuqueConnectionConfig :execrows
INSERT INTO knowledge_connection_yuque_configs (
	connection_id, token_encrypted, group_login, namespace, pending_docs_json, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (connection_id) DO UPDATE
SET token_encrypted = EXCLUDED.token_encrypted,
    group_login = EXCLUDED.group_login,
    namespace = EXCLUDED.namespace,
    pending_docs_json = EXCLUDED.pending_docs_json,
    updated_at = EXCLUDED.updated_at;

-- name: DeleteFeishuConnectionConfig :execrows
DELETE FROM knowledge_connection_feishu_configs
WHERE connection_id = $1;

-- name: UpsertFeishuConnectionConfig :execrows
INSERT INTO knowledge_connection_feishu_configs (
	connection_id, app_id, app_secret_encrypted, entry_type, entry_token, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (connection_id) DO UPDATE
SET app_id = EXCLUDED.app_id,
    app_secret_encrypted = EXCLUDED.app_secret_encrypted,
    entry_type = EXCLUDED.entry_type,
    entry_token = EXCLUDED.entry_token,
    updated_at = EXCLUDED.updated_at;

-- name: DeleteYuqueConnectionConfig :execrows
DELETE FROM knowledge_connection_yuque_configs
WHERE connection_id = $1;

-- name: GetYuqueConnectionConfig :one
SELECT token_encrypted, group_login, namespace, pending_docs_json, created_at, updated_at
FROM knowledge_connection_yuque_configs
WHERE connection_id = $1;

-- name: GetFeishuConnectionConfig :one
SELECT app_id, app_secret_encrypted, entry_type, entry_token, created_at, updated_at
FROM knowledge_connection_feishu_configs
WHERE connection_id = $1;
