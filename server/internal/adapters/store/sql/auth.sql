-- name: UpsertUser :execrows
INSERT INTO users (id, username, display_name, email, avatar_url, password_hash, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (id) DO UPDATE SET
	username = EXCLUDED.username,
	display_name = EXCLUDED.display_name,
	email = EXCLUDED.email,
	avatar_url = EXCLUDED.avatar_url,
	password_hash = EXCLUDED.password_hash;

-- name: GetUserByUsername :one
SELECT id, username, display_name, email, avatar_url, password_hash, created_at
FROM users
WHERE username = $1;

-- name: GetUserByID :one
SELECT id, username, display_name, email, avatar_url, password_hash, created_at
FROM users
WHERE id = $1;

-- name: GetUserByAuthIdentity :one
SELECT u.id, u.username, u.display_name, u.email, u.avatar_url, u.password_hash, u.created_at
FROM auth_identities ai
JOIN users u ON u.id = ai.user_id
WHERE ai.provider = $1 AND ai.provider_subject = $2;

-- name: UpsertAuthIdentity :execrows
INSERT INTO auth_identities (
	id, user_id, provider, provider_subject, email, email_verified, avatar_url, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (provider, provider_subject) DO UPDATE SET
	user_id = EXCLUDED.user_id,
	email = EXCLUDED.email,
	email_verified = EXCLUDED.email_verified,
	avatar_url = EXCLUDED.avatar_url,
	updated_at = EXCLUDED.updated_at;

-- name: UpsertFluxACredential :execrows
INSERT INTO fluxa_credentials (user_id, site, token_ciphertext, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id, site) DO UPDATE SET
	token_ciphertext = EXCLUDED.token_ciphertext,
	updated_at = EXCLUDED.updated_at;

-- name: GetFluxACredential :one
SELECT user_id, site, token_ciphertext, created_at, updated_at
FROM fluxa_credentials
WHERE user_id = $1 AND site = $2;
