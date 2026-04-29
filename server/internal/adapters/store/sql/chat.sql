-- name: CreateSession :execrows
INSERT INTO sessions (id, user_id, access_token, refresh_token, expires_at, revoked_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: GetSessionByRefreshToken :one
SELECT id, user_id, access_token, refresh_token, expires_at, revoked_at, created_at
FROM sessions
WHERE refresh_token = $1;

-- name: RevokeSession :execrows
UPDATE sessions
SET revoked_at = NOW()
WHERE id = $1;

-- name: ListUserChatModels :many
SELECT id, user_id, purpose, origin, name, base_url, api_key_encrypted, model_name, temperature, is_selected, created_at, updated_at
FROM user_chat_models
WHERE user_id = $1
ORDER BY purpose ASC, is_selected DESC, created_at ASC, name ASC;

-- name: GetUserChatModel :one
SELECT id, user_id, purpose, origin, name, base_url, api_key_encrypted, model_name, temperature, is_selected, created_at, updated_at
FROM user_chat_models
WHERE id = $1 AND user_id = $2;

-- name: GetSelectedUserChatModel :one
SELECT id, user_id, purpose, origin, name, base_url, api_key_encrypted, model_name, temperature, is_selected, created_at, updated_at
FROM user_chat_models
WHERE user_id = $1 AND purpose = $2 AND is_selected = TRUE
LIMIT 1;

-- name: CreateUserChatModel :execrows
INSERT INTO user_chat_models (id, user_id, purpose, origin, name, base_url, api_key_encrypted, model_name, temperature, is_selected, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);

-- name: UpdateUserChatModel :execrows
UPDATE user_chat_models
SET name = $3,
    base_url = $4,
    api_key_encrypted = $5,
    model_name = $6,
    temperature = $7,
    updated_at = $8
WHERE id = $1 AND user_id = $2;

-- name: ListAlternativeUserChatModels :many
SELECT id, user_id, purpose, origin, name, base_url, api_key_encrypted, model_name, temperature, is_selected, created_at, updated_at
FROM user_chat_models
WHERE user_id = $1 AND purpose = $2 AND id <> $3
ORDER BY created_at ASC, name ASC;

-- name: DeleteUserChatModel :execrows
DELETE FROM user_chat_models
WHERE id = $1 AND user_id = $2;

-- name: CreateChatSession :execrows
INSERT INTO chat_sessions (id, user_id, title, pinned, last_message_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: GetChatSession :one
SELECT id, user_id, title, pinned, last_message_at, created_at, updated_at
FROM chat_sessions
WHERE id = $1 AND user_id = $2;

-- name: UpdateChatSession :execrows
UPDATE chat_sessions
SET user_id = $2,
    title = $3,
    pinned = $4,
    last_message_at = $5,
    created_at = $6,
    updated_at = $7
WHERE id = $1;

-- name: ListChatSessions :many
SELECT id, user_id, title, pinned, last_message_at, created_at, updated_at
FROM chat_sessions
WHERE user_id = $1
ORDER BY pinned DESC, COALESCE(last_message_at, created_at) DESC, updated_at DESC;

-- name: DeleteChatSession :execrows
DELETE FROM chat_sessions
WHERE id = $1 AND user_id = $2;

-- name: SaveChatMessage :execrows
INSERT INTO chat_messages (
	id, session_id, user_id, role, content, skill, use_knowledge,
	prompt_tokens, completion_tokens, total_tokens, created_at, completed_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (id) DO UPDATE SET
	session_id = EXCLUDED.session_id,
	user_id = EXCLUDED.user_id,
	role = EXCLUDED.role,
	content = EXCLUDED.content,
	skill = EXCLUDED.skill,
	use_knowledge = EXCLUDED.use_knowledge,
	prompt_tokens = EXCLUDED.prompt_tokens,
	completion_tokens = EXCLUDED.completion_tokens,
	total_tokens = EXCLUDED.total_tokens,
	created_at = EXCLUDED.created_at,
	completed_at = EXCLUDED.completed_at;

-- name: ListChatMessages :many
SELECT cm.id, cm.session_id, cm.user_id, cm.role, cm.content, cm.skill, cm.use_knowledge,
       cm.prompt_tokens, cm.completion_tokens, cm.total_tokens, cm.created_at, cm.completed_at
FROM chat_messages cm
INNER JOIN chat_sessions cs ON cs.id = cm.session_id
WHERE cm.session_id = $1 AND cs.user_id = $2
ORDER BY cm.created_at ASC;

-- name: DeleteChatMessageSources :execrows
DELETE FROM chat_message_sources
WHERE message_id = $1;

-- name: InsertChatMessageSource :execrows
INSERT INTO chat_message_sources (
	id, message_id, provider, connection_id, document_id, chunk_id,
	title, repo, url, snippet, matched_lines_json, score, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);

-- name: ListChatMessageSources :many
SELECT id, message_id, provider, connection_id, document_id, chunk_id,
       title, repo, url, snippet, matched_lines_json, score, created_at
FROM chat_message_sources
WHERE message_id = $1
ORDER BY created_at ASC;

-- name: EnsureUserChatModelDefault :execrows
INSERT INTO user_chat_models (id, user_id, purpose, origin, name, base_url, api_key_encrypted, model_name, temperature, is_selected, created_at, updated_at)
SELECT $1, $2, $3, 'default', $4, $5, $6, $7, $8, TRUE, $9, $9
WHERE NOT EXISTS (
	SELECT 1 FROM user_chat_models WHERE user_id = $2 AND purpose = $3
);

-- name: CountSelectedUserChatModels :one
SELECT COUNT(*)
FROM user_chat_models
WHERE user_id = $1 AND purpose = $2 AND is_selected = TRUE;

-- name: SelectFallbackUserChatModel :one
SELECT id
FROM user_chat_models
WHERE user_id = $1 AND purpose = $2
ORDER BY created_at ASC, name ASC
LIMIT 1;

-- name: UnsetSelectedUserChatModel :execrows
UPDATE user_chat_models
SET is_selected = FALSE,
    updated_at = $3
WHERE user_id = $1 AND purpose = $2 AND is_selected = TRUE;

-- name: SetSelectedUserChatModel :execrows
UPDATE user_chat_models
SET is_selected = TRUE,
    updated_at = $4
WHERE user_id = $1 AND purpose = $2 AND id = $3;
