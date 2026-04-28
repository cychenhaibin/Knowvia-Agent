package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillruntime"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

type scanner interface {
	Scan(dest ...any) error
}

type dbQuerier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func NewPostgresStore(ctx context.Context, dsn string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	store := &PostgresStore{pool: pool}
	if err := store.applySchema(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return store, nil
}

func (s *PostgresStore) Close() {
	s.pool.Close()
}

func (s *PostgresStore) applySchema(ctx context.Context) error {
	statements := []string{
		`CREATE EXTENSION IF NOT EXISTS vector`,
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			display_name TEXT NOT NULL,
			email TEXT NOT NULL DEFAULT '',
			avatar_url TEXT NOT NULL DEFAULT '',
			password_hash TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS auth_identities (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			provider TEXT NOT NULL,
			provider_subject TEXT NOT NULL,
			email TEXT NOT NULL DEFAULT '',
			email_verified BOOLEAN NOT NULL DEFAULT FALSE,
			avatar_url TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (provider, provider_subject)
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			access_token TEXT NOT NULL UNIQUE,
			refresh_token TEXT NOT NULL UNIQUE,
			expires_at TIMESTAMPTZ NOT NULL,
			revoked_at TIMESTAMPTZ NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS user_chat_models (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			purpose TEXT NOT NULL,
			origin TEXT NOT NULL DEFAULT 'custom',
			name TEXT NOT NULL,
			base_url TEXT NOT NULL DEFAULT '',
			api_key_encrypted TEXT NOT NULL DEFAULT '',
			model_name TEXT NOT NULL,
			temperature DOUBLE PRECISION NOT NULL DEFAULT 0.05,
			is_selected BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (user_id, purpose, name)
		)`,
		`ALTER TABLE user_chat_models
			ADD COLUMN IF NOT EXISTS temperature DOUBLE PRECISION NOT NULL DEFAULT 0.05`,
		`CREATE TABLE IF NOT EXISTS chat_sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			title TEXT NOT NULL,
			pinned BOOLEAN NOT NULL DEFAULT FALSE,
			last_message_at TIMESTAMPTZ NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS chat_messages (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			skill TEXT NOT NULL DEFAULT '',
			use_knowledge BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			completed_at TIMESTAMPTZ NULL
		)`,
		`CREATE TABLE IF NOT EXISTS chat_message_sources (
			id TEXT PRIMARY KEY,
			message_id TEXT NOT NULL REFERENCES chat_messages(id) ON DELETE CASCADE,
			provider TEXT NOT NULL,
			connection_id TEXT NOT NULL DEFAULT '',
			document_id TEXT NOT NULL DEFAULT '',
			chunk_id TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL,
			repo TEXT NOT NULL DEFAULT '',
			url TEXT NOT NULL DEFAULT '',
			snippet TEXT NOT NULL DEFAULT '',
			matched_lines_json TEXT NOT NULL DEFAULT '[]',
			score DOUBLE PRECISION NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`ALTER TABLE chat_message_sources
			ADD COLUMN IF NOT EXISTS matched_lines_json TEXT NOT NULL DEFAULT '[]'`,
		`CREATE TABLE IF NOT EXISTS skills (
				id TEXT PRIMARY KEY,
				user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
				slug TEXT NOT NULL,
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			prompt TEXT NOT NULL,
			mode TEXT NOT NULL DEFAULT 'answer',
			source TEXT NOT NULL,
			enabled BOOLEAN NOT NULL DEFAULT TRUE,
			repo_url TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				UNIQUE (user_id, slug)
			)`,
		`CREATE TABLE IF NOT EXISTS skill_definitions (
				id TEXT PRIMARY KEY,
				user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
				slug TEXT NOT NULL,
				kind TEXT NOT NULL DEFAULT 'chat_profile',
				source TEXT NOT NULL,
				repo_url TEXT NOT NULL DEFAULT '',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				UNIQUE (user_id, slug)
			)`,
		`CREATE TABLE IF NOT EXISTS skill_revisions (
				id TEXT PRIMARY KEY,
				definition_id TEXT NOT NULL REFERENCES skill_definitions(id) ON DELETE CASCADE,
				version INTEGER NOT NULL,
				title TEXT NOT NULL,
				description TEXT NOT NULL DEFAULT '',
				prompt TEXT NOT NULL,
				mode TEXT NOT NULL DEFAULT 'answer',
				manifest_json TEXT NOT NULL DEFAULT '{}',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				UNIQUE (definition_id, version)
			)`,
		`CREATE TABLE IF NOT EXISTS skill_installations (
				id TEXT PRIMARY KEY,
				user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
				definition_id TEXT NOT NULL REFERENCES skill_definitions(id) ON DELETE CASCADE,
				current_revision_id TEXT NOT NULL REFERENCES skill_revisions(id) ON DELETE RESTRICT,
				name TEXT NOT NULL DEFAULT '',
				is_default BOOLEAN NOT NULL DEFAULT FALSE,
				enabled BOOLEAN NOT NULL DEFAULT TRUE,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
		`CREATE TABLE IF NOT EXISTS skill_runtime_snapshots (
				id TEXT PRIMARY KEY,
				scope TEXT NOT NULL,
				scope_id TEXT NOT NULL,
				user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
				installation_id TEXT NOT NULL DEFAULT '',
				definition_id TEXT NOT NULL DEFAULT '',
				revision_id TEXT NOT NULL DEFAULT '',
				kind TEXT NOT NULL DEFAULT 'chat_profile',
				title TEXT NOT NULL DEFAULT '',
				description TEXT NOT NULL DEFAULT '',
				mode TEXT NOT NULL DEFAULT 'answer',
				prompt TEXT NOT NULL DEFAULT '',
				runtime_spec_json TEXT NOT NULL DEFAULT '{}',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				UNIQUE (scope, scope_id)
			)`,
		`CREATE TABLE IF NOT EXISTS skill_artifacts (
				id TEXT PRIMARY KEY,
				user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
				definition_id TEXT NOT NULL DEFAULT '',
				revision_id TEXT NOT NULL DEFAULT '',
				source TEXT NOT NULL,
				file_name TEXT NOT NULL,
				media_type TEXT NOT NULL DEFAULT '',
				source_url TEXT NOT NULL DEFAULT '',
				sha256 TEXT NOT NULL,
				size_bytes BIGINT NOT NULL DEFAULT 0,
				entry_path TEXT NOT NULL DEFAULT '',
				manifest_path TEXT NOT NULL DEFAULT '',
				instructions_path TEXT NOT NULL DEFAULT '',
				archive_bytes BYTEA NULL,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
		`CREATE TABLE IF NOT EXISTS skill_artifact_files (
				id TEXT PRIMARY KEY,
				artifact_id TEXT NOT NULL REFERENCES skill_artifacts(id) ON DELETE CASCADE,
				user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
				path TEXT NOT NULL,
				media_type TEXT NOT NULL DEFAULT '',
				size_bytes BIGINT NOT NULL DEFAULT 0,
				sha256 TEXT NOT NULL DEFAULT '',
				is_manifest BOOLEAN NOT NULL DEFAULT FALSE,
				is_instructions BOOLEAN NOT NULL DEFAULT FALSE,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
		`CREATE TABLE IF NOT EXISTS skill_import_jobs (
				id TEXT PRIMARY KEY,
				user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
				source TEXT NOT NULL,
				status TEXT NOT NULL,
				artifact_id TEXT NOT NULL DEFAULT '',
				definition_id TEXT NOT NULL DEFAULT '',
				revision_id TEXT NOT NULL DEFAULT '',
				installation_id TEXT NOT NULL DEFAULT '',
				error_message TEXT NOT NULL DEFAULT '',
				request_json TEXT NOT NULL DEFAULT '',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				completed_at TIMESTAMPTZ NULL
			)`,
		`CREATE TABLE IF NOT EXISTS mirror_tasks (
				id TEXT PRIMARY KEY,
			kind TEXT NOT NULL,
			user_id TEXT NOT NULL DEFAULT '',
			resource_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			payload TEXT NOT NULL DEFAULT '',
			attempts INTEGER NOT NULL DEFAULT 0,
			last_error TEXT NOT NULL DEFAULT '',
			next_retry_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			completed_at TIMESTAMPTZ NULL
		)`,
		`CREATE TABLE IF NOT EXISTS knowledge_connections (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			provider TEXT NOT NULL,
			name TEXT NOT NULL,
			sync_enabled BOOLEAN NOT NULL DEFAULT TRUE,
			last_synced_at TIMESTAMPTZ NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS knowledge_connection_yuque_configs (
			connection_id TEXT PRIMARY KEY REFERENCES knowledge_connections(id) ON DELETE CASCADE,
			token_encrypted TEXT NOT NULL,
			group_login TEXT NOT NULL,
			namespace TEXT NULL,
			pending_docs_json TEXT NOT NULL DEFAULT '[]',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`ALTER TABLE knowledge_connection_yuque_configs
			ADD COLUMN IF NOT EXISTS pending_docs_json TEXT NOT NULL DEFAULT '[]'`,
		`CREATE TABLE IF NOT EXISTS knowledge_connection_feishu_configs (
			connection_id TEXT PRIMARY KEY REFERENCES knowledge_connections(id) ON DELETE CASCADE,
			app_id TEXT NOT NULL,
			app_secret_encrypted TEXT NOT NULL,
			entry_type TEXT NOT NULL DEFAULT 'docx',
			entry_token TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`ALTER TABLE knowledge_connection_feishu_configs
			ADD COLUMN IF NOT EXISTS entry_type TEXT NOT NULL DEFAULT 'docx'`,
		`ALTER TABLE knowledge_connection_feishu_configs
			ADD COLUMN IF NOT EXISTS entry_token TEXT`,
		`DO $$
		BEGIN
			IF EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_schema = current_schema()
				  AND table_name = 'knowledge_connection_feishu_configs'
				  AND column_name = 'document_id'
			) THEN
				UPDATE knowledge_connection_feishu_configs
				SET entry_token = document_id
				WHERE (entry_token IS NULL OR entry_token = '')
				  AND document_id IS NOT NULL
				  AND document_id <> '';
			END IF;
		END$$`,
		`ALTER TABLE knowledge_connection_feishu_configs
			ALTER COLUMN entry_token SET NOT NULL`,
		`ALTER TABLE knowledge_connection_feishu_configs
			DROP COLUMN IF EXISTS document_id`,
		`DO $$
		BEGIN
			IF EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_schema = current_schema()
				  AND table_name = 'knowledge_connections'
				  AND column_name = 'token_encrypted'
			) THEN
				INSERT INTO knowledge_connection_yuque_configs (
					connection_id,
					token_encrypted,
					group_login,
					namespace,
					created_at,
					updated_at
				)
				SELECT
					id,
					token_encrypted,
					group_login,
					namespace,
					created_at,
					updated_at
				FROM knowledge_connections
				WHERE provider = 'yuque'
				ON CONFLICT (connection_id) DO UPDATE
				SET token_encrypted = EXCLUDED.token_encrypted,
				    group_login = EXCLUDED.group_login,
				    namespace = EXCLUDED.namespace,
				    updated_at = EXCLUDED.updated_at;
			END IF;
		END$$`,
		`ALTER TABLE knowledge_connections DROP COLUMN IF EXISTS token_encrypted`,
		`ALTER TABLE knowledge_connections DROP COLUMN IF EXISTS group_login`,
		`ALTER TABLE knowledge_connections DROP COLUMN IF EXISTS namespace`,
		`CREATE TABLE IF NOT EXISTS knowledge_sync_jobs (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			connection_id TEXT NOT NULL REFERENCES knowledge_connections(id) ON DELETE CASCADE,
			status TEXT NOT NULL,
			summary TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			started_at TIMESTAMPTZ NULL,
			finished_at TIMESTAMPTZ NULL
		)`,
		`CREATE TABLE IF NOT EXISTS knowledge_documents (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			connection_id TEXT NOT NULL REFERENCES knowledge_connections(id) ON DELETE CASCADE,
			repo TEXT NOT NULL,
			title TEXT NOT NULL,
			doc_ref TEXT NOT NULL,
			source_url TEXT NULL,
			body_hash TEXT NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS knowledge_document_meta (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			connection_id TEXT NOT NULL REFERENCES knowledge_connections(id) ON DELETE CASCADE,
			repo TEXT NOT NULL,
			title TEXT NOT NULL,
			doc_ref TEXT NOT NULL,
			source_url TEXT NULL,
			chunk_count INTEGER NOT NULL DEFAULT 0,
			updated_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS knowledge_source_documents (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			connection_id TEXT NOT NULL REFERENCES knowledge_connections(id) ON DELETE CASCADE,
			provider TEXT NOT NULL,
			external_id TEXT NOT NULL,
			repo TEXT NOT NULL,
			title TEXT NOT NULL,
			doc_ref TEXT NOT NULL,
			source_url TEXT NULL,
			raw_body TEXT NOT NULL,
			body_hash TEXT NOT NULL,
			source_updated_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS knowledge_chunks (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			connection_id TEXT NOT NULL REFERENCES knowledge_connections(id) ON DELETE CASCADE,
			document_id TEXT NOT NULL REFERENCES knowledge_documents(id) ON DELETE CASCADE,
			chunk_index INTEGER NOT NULL,
			content TEXT NOT NULL,
			content_hash TEXT NOT NULL,
			embedding VECTOR(1536),
			tsv TSVECTOR,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS runs (
				id TEXT PRIMARY KEY,
				user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
				title TEXT NOT NULL,
				goal TEXT NOT NULL,
				requested_mode TEXT NOT NULL,
				knowledge_connection_ids TEXT[] NOT NULL DEFAULT '{}',
				skill_installation_id TEXT NOT NULL DEFAULT '',
				skill_snapshot_id TEXT NOT NULL DEFAULT '',
				effective_mode TEXT NOT NULL DEFAULT 'auto',
				status TEXT NOT NULL,
				latest_artifact_id TEXT NOT NULL DEFAULT '',
			error_message TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS run_steps (
			id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
			kind TEXT NOT NULL,
			label TEXT NOT NULL,
			status TEXT NOT NULL,
			summary TEXT NOT NULL DEFAULT '',
			started_at TIMESTAMPTZ NULL,
			finished_at TIMESTAMPTZ NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS run_artifacts (
			id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
			kind TEXT NOT NULL,
			content_markdown TEXT NOT NULL,
			version INTEGER NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS run_sources (
			id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
			provider TEXT NOT NULL,
			connection_id TEXT NOT NULL DEFAULT '',
			document_id TEXT NOT NULL DEFAULT '',
			chunk_id TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL,
			repo TEXT NOT NULL DEFAULT '',
			url TEXT NOT NULL DEFAULT '',
			snippet TEXT NOT NULL DEFAULT '',
			score DOUBLE PRECISION NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
		`ALTER TABLE runs ADD COLUMN IF NOT EXISTS knowledge_connection_ids TEXT[] NOT NULL DEFAULT '{}'`,
		`ALTER TABLE runs ADD COLUMN IF NOT EXISTS skill_installation_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE runs ADD COLUMN IF NOT EXISTS skill_snapshot_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS email TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE skill_installations ADD COLUMN IF NOT EXISTS name TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE skill_installations ADD COLUMN IF NOT EXISTS is_default BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE user_chat_models ADD COLUMN IF NOT EXISTS origin TEXT NOT NULL DEFAULT 'custom'`,
		`ALTER TABLE user_chat_models ADD COLUMN IF NOT EXISTS name TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE user_chat_models ADD COLUMN IF NOT EXISTS base_url TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE user_chat_models ADD COLUMN IF NOT EXISTS api_key_encrypted TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE run_sources ADD COLUMN IF NOT EXISTS connection_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE run_sources ADD COLUMN IF NOT EXISTS document_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE run_sources ADD COLUMN IF NOT EXISTS chunk_id TEXT NOT NULL DEFAULT ''`,
		`UPDATE user_chat_models
			SET name = CASE
				WHEN purpose = 'knowledge' THEN 'Qwen3 8B'
				ELSE 'Gemma 3n E4B'
			END
			WHERE name = ''`,
		`UPDATE user_chat_models
			SET base_url = 'http://127.0.0.1:11434/v1'
			WHERE base_url = ''`,
		`UPDATE user_chat_models
			SET api_key_encrypted = 'ollama'
			WHERE api_key_encrypted = ''`,
		`UPDATE user_chat_models
			SET origin = 'default'
			WHERE origin = 'custom'
			  AND name IN ('Gemma 3n E4B', 'Qwen3 8B')
			  AND base_url = 'http://127.0.0.1:11434/v1'
			  AND api_key_encrypted = 'ollama'
			  AND (
				(name = 'Gemma 3n E4B' AND model_name = 'gemma3n:e4b')
				OR (name = 'Qwen3 8B' AND model_name = 'qwen3:8b')
			  )`,
		`ALTER TABLE user_chat_models DROP CONSTRAINT IF EXISTS user_chat_models_user_id_purpose_model_name_key`,
		`ALTER TABLE skill_installations DROP CONSTRAINT IF EXISTS skill_installations_user_id_definition_id_key`,
		`UPDATE skill_installations si
			SET name = sr.title
			FROM skill_revisions sr
			WHERE si.current_revision_id = sr.id
			  AND si.name = ''`,
		`UPDATE skill_installations si
			SET is_default = TRUE
			WHERE NOT EXISTS (
				SELECT 1
				FROM skill_installations existing
				WHERE existing.user_id = si.user_id
				  AND existing.definition_id = si.definition_id
				  AND existing.is_default = TRUE
			)
			  AND si.id = (
				SELECT candidate.id
				FROM skill_installations candidate
				WHERE candidate.user_id = si.user_id
				  AND candidate.definition_id = si.definition_id
				ORDER BY candidate.updated_at DESC, candidate.created_at ASC, candidate.id ASC
				LIMIT 1
			  )`,
		`CREATE INDEX IF NOT EXISTS idx_runs_user_updated ON runs(user_id, updated_at DESC)`,
		`DROP INDEX IF EXISTS idx_users_email_unique`,
		`CREATE INDEX IF NOT EXISTS idx_auth_identities_user_provider ON auth_identities(user_id, provider)`,
		`ALTER TABLE chat_sessions ADD COLUMN IF NOT EXISTS pinned BOOLEAN NOT NULL DEFAULT FALSE`,
		`CREATE INDEX IF NOT EXISTS idx_chat_sessions_user_updated ON chat_sessions(user_id, updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_sessions_user_pinned_updated ON chat_sessions(user_id, pinned DESC, updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_user_chat_models_user_purpose ON user_chat_models(user_id, purpose, created_at ASC)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_user_chat_models_user_purpose_name ON user_chat_models(user_id, purpose, lower(name))`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_user_chat_models_selected ON user_chat_models(user_id, purpose) WHERE is_selected`,
		`CREATE INDEX IF NOT EXISTS idx_chat_messages_session_created ON chat_messages(session_id, created_at ASC)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_message_sources_message_created ON chat_message_sources(message_id, created_at ASC)`,
		`CREATE INDEX IF NOT EXISTS idx_skills_user_updated ON skills(user_id, updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_skill_definitions_user_updated ON skill_definitions(user_id, updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_skill_installations_user_updated ON skill_installations(user_id, updated_at DESC)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_skill_installations_user_definition_default
			ON skill_installations(user_id, definition_id)
			WHERE is_default = TRUE`,
		`CREATE INDEX IF NOT EXISTS idx_skill_snapshots_scope_created ON skill_runtime_snapshots(scope, scope_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_skill_artifacts_user_created ON skill_artifacts(user_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_skill_artifact_files_artifact_path ON skill_artifact_files(artifact_id, path ASC)`,
		`CREATE INDEX IF NOT EXISTS idx_skill_import_jobs_user_updated ON skill_import_jobs(user_id, updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_mirror_tasks_status_retry ON mirror_tasks(status, next_retry_at ASC)`,
		`CREATE INDEX IF NOT EXISTS idx_steps_run_created ON run_steps(run_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_artifacts_run_created ON run_artifacts(run_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_sources_run_created ON run_sources(run_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_doc_meta_connection_updated ON knowledge_document_meta(connection_id, updated_at DESC)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_source_docs_connection_external ON knowledge_source_documents(connection_id, external_id)`,
		`CREATE INDEX IF NOT EXISTS idx_source_docs_connection_updated ON knowledge_source_documents(connection_id, source_updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_chunks_connection ON knowledge_chunks(connection_id)`,
		`CREATE INDEX IF NOT EXISTS idx_chunks_tsv ON knowledge_chunks USING GIN(tsv)`,
	}
	for _, statement := range statements {
		if _, err := s.pool.Exec(ctx, statement); err != nil {
			return fmt.Errorf("apply schema: %w", err)
		}
	}
	if err := s.backfillLegacySkills(ctx); err != nil {
		return fmt.Errorf("backfill legacy skills: %w", err)
	}
	return nil
}

func (s *PostgresStore) backfillLegacySkills(ctx context.Context) error {
	var existing int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(1) FROM skill_installations`).Scan(&existing); err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, slug, title, description, prompt, mode, source, enabled, repo_url, created_at, updated_at
		FROM skills
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type legacySkill struct {
		ID          string
		UserID      string
		Slug        string
		Title       string
		Description string
		Prompt      string
		Mode        string
		Source      string
		Enabled     bool
		RepoURL     string
		CreatedAt   time.Time
		UpdatedAt   time.Time
	}
	items := []legacySkill{}
	for rows.Next() {
		var item legacySkill
		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.Slug,
			&item.Title,
			&item.Description,
			&item.Prompt,
			&item.Mode,
			&item.Source,
			&item.Enabled,
			&item.RepoURL,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	for _, item := range items {
		definitionID := "legacy-def:" + item.ID
		revisionID := "legacy-rev:" + item.ID + ":1"
		if _, err := s.pool.Exec(ctx, `
			INSERT INTO skill_definitions (id, user_id, slug, kind, source, repo_url, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (id) DO NOTHING
		`, definitionID, item.UserID, item.Slug, string(domain.SkillKindChatProfile), item.Source, item.RepoURL, item.CreatedAt, item.UpdatedAt); err != nil {
			return err
		}
		if _, err := s.pool.Exec(ctx, `
			INSERT INTO skill_revisions (id, definition_id, version, title, description, prompt, mode, manifest_json, created_at)
			VALUES ($1, $2, 1, $3, $4, $5, $6, '{}', $7)
			ON CONFLICT (id) DO NOTHING
		`, revisionID, definitionID, item.Title, item.Description, item.Prompt, item.Mode, item.UpdatedAt); err != nil {
			return err
		}
		if _, err := s.pool.Exec(ctx, `
			INSERT INTO skill_installations (id, user_id, definition_id, current_revision_id, name, is_default, enabled, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, TRUE, $6, $7, $8)
			ON CONFLICT (id) DO NOTHING
		`, item.ID, item.UserID, definitionID, revisionID, item.Title, item.Enabled, item.CreatedAt, item.UpdatedAt); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresStore) UpsertUser(ctx context.Context, user domain.User) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO users (id, username, display_name, email, avatar_url, password_hash, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			username = EXCLUDED.username,
			display_name = EXCLUDED.display_name,
			email = EXCLUDED.email,
			avatar_url = EXCLUDED.avatar_url,
			password_hash = EXCLUDED.password_hash
	`, user.ID, user.Username, user.DisplayName, user.Email, user.AvatarURL, user.PasswordHash, user.CreatedAt)
	return err
}

func (s *PostgresStore) GetUserByUsername(ctx context.Context, username string) (domain.User, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, username, display_name, email, avatar_url, password_hash, created_at
		FROM users
		WHERE username = $1
	`, username)
	return scanUser(row)
}

func (s *PostgresStore) GetUserByID(ctx context.Context, userID string) (domain.User, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, username, display_name, email, avatar_url, password_hash, created_at
		FROM users
		WHERE id = $1
	`, userID)
	return scanUser(row)
}

func (s *PostgresStore) GetUserByAuthIdentity(ctx context.Context, provider domain.AuthProvider, subject string) (domain.User, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT u.id, u.username, u.display_name, u.email, u.avatar_url, u.password_hash, u.created_at
		FROM auth_identities ai
		JOIN users u ON u.id = ai.user_id
		WHERE ai.provider = $1 AND ai.provider_subject = $2
	`, string(provider), strings.TrimSpace(subject))
	return scanUser(row)
}

func (s *PostgresStore) UpsertAuthIdentity(ctx context.Context, identity domain.AuthIdentity) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO auth_identities (
			id, user_id, provider, provider_subject, email, email_verified, avatar_url, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (provider, provider_subject) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			email = EXCLUDED.email,
			email_verified = EXCLUDED.email_verified,
			avatar_url = EXCLUDED.avatar_url,
			updated_at = EXCLUDED.updated_at
	`, identity.ID, identity.UserID, string(identity.Provider), identity.ProviderSubject, identity.Email, identity.EmailVerified, identity.AvatarURL, identity.CreatedAt, identity.UpdatedAt)
	return err
}

func (s *PostgresStore) CreateSession(ctx context.Context, session domain.Session) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO sessions (id, user_id, access_token, refresh_token, expires_at, revoked_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, session.ID, session.UserID, session.AccessToken, session.RefreshToken, session.ExpiresAt, session.RevokedAt, session.CreatedAt)
	return err
}

func (s *PostgresStore) GetSessionByRefreshToken(ctx context.Context, refreshToken string) (domain.Session, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, user_id, access_token, refresh_token, expires_at, revoked_at, created_at
		FROM sessions
		WHERE refresh_token = $1
	`, refreshToken)
	return scanSession(row)
}

func (s *PostgresStore) RevokeSession(ctx context.Context, sessionID string) error {
	commandTag, err := s.pool.Exec(ctx, `
		UPDATE sessions
		SET revoked_at = NOW()
		WHERE id = $1
	`, sessionID)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) EnsureUserChatModelDefaults(ctx context.Context, userID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := ensureUserChatModelDefaultsTx(ctx, tx, userID, time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) ListUserChatModels(ctx context.Context, userID string) ([]domain.UserChatModel, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, purpose, origin, name, base_url, api_key_encrypted, model_name, temperature, is_selected, created_at, updated_at
		FROM user_chat_models
		WHERE user_id = $1
		ORDER BY purpose ASC, is_selected DESC, created_at ASC, name ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.UserChatModel, 0)
	for rows.Next() {
		item, err := scanUserChatModel(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) GetUserChatModel(ctx context.Context, userID, modelID string) (domain.UserChatModel, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, user_id, purpose, origin, name, base_url, api_key_encrypted, model_name, temperature, is_selected, created_at, updated_at
		FROM user_chat_models
		WHERE id = $1 AND user_id = $2
	`, modelID, userID)
	return scanUserChatModel(row)
}

func (s *PostgresStore) GetSelectedUserChatModel(ctx context.Context, userID string, purpose domain.ChatModelPurpose) (domain.UserChatModel, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, user_id, purpose, origin, name, base_url, api_key_encrypted, model_name, temperature, is_selected, created_at, updated_at
		FROM user_chat_models
		WHERE user_id = $1 AND purpose = $2 AND is_selected = TRUE
		LIMIT 1
	`, userID, string(purpose))
	return scanUserChatModel(row)
}

func (s *PostgresStore) CreateUserChatModel(ctx context.Context, model domain.UserChatModel) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO user_chat_models (id, user_id, purpose, origin, name, base_url, api_key_encrypted, model_name, temperature, is_selected, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, model.ID, model.UserID, string(model.Purpose), string(model.Origin), model.Name, model.BaseURL, model.APIKey, model.ModelName, model.Temperature, model.IsSelected, model.CreatedAt, model.UpdatedAt)
	return normalizeError(err)
}

func (s *PostgresStore) UpdateUserChatModel(ctx context.Context, model domain.UserChatModel) error {
	commandTag, err := s.pool.Exec(ctx, `
		UPDATE user_chat_models
		SET name = $3,
		    base_url = $4,
		    api_key_encrypted = $5,
		    model_name = $6,
		    temperature = $7,
		    updated_at = $8
		WHERE id = $1 AND user_id = $2
	`, model.ID, model.UserID, model.Name, model.BaseURL, model.APIKey, model.ModelName, model.Temperature, model.UpdatedAt)
	if err != nil {
		return normalizeError(err)
	}
	if commandTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) SelectUserChatModel(ctx context.Context, userID, modelID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	model, err := getUserChatModelTx(ctx, tx, userID, modelID)
	if err != nil {
		return err
	}
	if err := setSelectedUserChatModelTx(ctx, tx, userID, model.Purpose, modelID, time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) DeleteUserChatModel(ctx context.Context, userID, modelID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	model, err := getUserChatModelTx(ctx, tx, userID, modelID)
	if err != nil {
		return err
	}
	if model.Origin == domain.ChatModelOriginDefault {
		return ErrConflict
	}
	rows, err := tx.Query(ctx, `
		SELECT id, user_id, purpose, origin, name, base_url, api_key_encrypted, model_name, temperature, is_selected, created_at, updated_at
		FROM user_chat_models
		WHERE user_id = $1 AND purpose = $2 AND id <> $3
		ORDER BY created_at ASC, name ASC
	`, userID, string(model.Purpose), modelID)
	if err != nil {
		return err
	}
	remaining := make([]domain.UserChatModel, 0)
	for rows.Next() {
		item, scanErr := scanUserChatModel(rows)
		if scanErr != nil {
			rows.Close()
			return scanErr
		}
		remaining = append(remaining, item)
	}
	rows.Close()
	if len(remaining) == 0 {
		return ErrConflict
	}

	if model.IsSelected {
		fallbackID := remaining[0].ID
		if err := setSelectedUserChatModelTx(
			ctx,
			tx,
			userID,
			model.Purpose,
			fallbackID,
			time.Now().UTC(),
		); err != nil {
			return err
		}
	}

	commandTag, err := tx.Exec(ctx, `
		DELETE FROM user_chat_models
		WHERE id = $1 AND user_id = $2
	`, modelID, userID)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) CreateChatSession(ctx context.Context, session domain.ChatSession) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO chat_sessions (id, user_id, title, pinned, last_message_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, session.ID, session.UserID, session.Title, session.Pinned, session.LastMessageAt, session.CreatedAt, session.UpdatedAt)
	return err
}

func (s *PostgresStore) GetChatSession(ctx context.Context, userID, sessionID string) (domain.ChatSession, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, user_id, title, pinned, last_message_at, created_at, updated_at
		FROM chat_sessions
		WHERE id = $1 AND user_id = $2
	`, sessionID, userID)
	return scanChatSession(row)
}

func (s *PostgresStore) UpdateChatSession(ctx context.Context, session domain.ChatSession) error {
	commandTag, err := s.pool.Exec(ctx, `
		UPDATE chat_sessions
		SET user_id = $2,
		    title = $3,
		    pinned = $4,
		    last_message_at = $5,
		    created_at = $6,
		    updated_at = $7
		WHERE id = $1
	`, session.ID, session.UserID, session.Title, session.Pinned, session.LastMessageAt, session.CreatedAt, session.UpdatedAt)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) ListChatSessions(ctx context.Context, userID string) ([]domain.ChatSession, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, title, pinned, last_message_at, created_at, updated_at
		FROM chat_sessions
		WHERE user_id = $1
		ORDER BY pinned DESC, COALESCE(last_message_at, created_at) DESC, updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions := []domain.ChatSession{}
	for rows.Next() {
		session, err := scanChatSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

func (s *PostgresStore) DeleteChatSession(ctx context.Context, userID, sessionID string) error {
	commandTag, err := s.pool.Exec(ctx, `
		DELETE FROM chat_sessions
		WHERE id = $1 AND user_id = $2
	`, sessionID, userID)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) SaveChatMessage(ctx context.Context, message domain.ChatMessage) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO chat_messages (
			id, session_id, user_id, role, content, skill, use_knowledge, created_at, completed_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			session_id = EXCLUDED.session_id,
			user_id = EXCLUDED.user_id,
			role = EXCLUDED.role,
			content = EXCLUDED.content,
			skill = EXCLUDED.skill,
			use_knowledge = EXCLUDED.use_knowledge,
			created_at = EXCLUDED.created_at,
			completed_at = EXCLUDED.completed_at
	`, message.ID, message.SessionID, message.UserID, string(message.Role), message.Content,
		message.Skill, message.UseKnowledge, message.CreatedAt, message.CompletedAt)
	return err
}

func (s *PostgresStore) ListChatMessages(ctx context.Context, userID, sessionID string) ([]domain.ChatMessage, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT cm.id, cm.session_id, cm.user_id, cm.role, cm.content, cm.skill, cm.use_knowledge, cm.created_at, cm.completed_at
		FROM chat_messages cm
		INNER JOIN chat_sessions cs ON cs.id = cm.session_id
		WHERE cm.session_id = $1 AND cs.user_id = $2
		ORDER BY cm.created_at ASC
	`, sessionID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := []domain.ChatMessage{}
	for rows.Next() {
		message, err := scanChatMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func (s *PostgresStore) SaveChatMessageSources(ctx context.Context, messageID string, sources []domain.ChatMessageSource) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM chat_message_sources WHERE message_id = $1`, messageID); err != nil {
		return err
	}
	for _, source := range sources {
		matchedLinesJSON, err := json.Marshal(source.MatchedLines)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO chat_message_sources (
				id, message_id, provider, connection_id, document_id, chunk_id,
				title, repo, url, snippet, matched_lines_json, score, created_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		`, source.ID, source.MessageID, string(source.Provider), source.ConnectionID,
			source.DocumentID, source.ChunkID, source.Title, source.Repo, source.URL,
			source.Snippet, string(matchedLinesJSON), source.Score, source.CreatedAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) ListChatMessageSources(ctx context.Context, messageID string) ([]domain.ChatMessageSource, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, message_id, provider, connection_id, document_id, chunk_id,
		       title, repo, url, snippet, matched_lines_json, score, created_at
		FROM chat_message_sources
		WHERE message_id = $1
		ORDER BY created_at ASC
	`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sources := []domain.ChatMessageSource{}
	for rows.Next() {
		source, err := scanChatMessageSource(rows)
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	return sources, rows.Err()
}

func (s *PostgresStore) CreateSkill(ctx context.Context, skill domain.Skill) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	manifestJSON, err := skillruntime.EncodeRevisionManifest(skill)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO skill_definitions (
			id, user_id, slug, kind, source, repo_url, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, skill.DefinitionID, skill.UserID, skill.Slug, string(skill.Kind), string(skill.Source), skill.RepoURL, skill.CreatedAt, skill.UpdatedAt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO skill_revisions (
			id, definition_id, version, title, description, prompt, mode, manifest_json, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, skill.RevisionID, skill.DefinitionID, skill.Version, skill.Title, skill.Description, skill.Prompt, skill.Mode, manifestJSON, skill.UpdatedAt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO skill_installations (
			id, user_id, definition_id, current_revision_id, name, is_default, enabled, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, TRUE, $6, $7, $8)
	`, skill.ID, skill.UserID, skill.DefinitionID, skill.RevisionID, skill.Title, skill.Enabled, skill.CreatedAt, skill.UpdatedAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) CreateSkillDefinitionRevision(ctx context.Context, definition domain.SkillDefinition, revision domain.SkillRevision) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	manifestJSON := revision.ManifestJSON
	if strings.TrimSpace(manifestJSON) == "" {
		manifestJSON = "{}"
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO skill_definitions (
			id, user_id, slug, kind, source, repo_url, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, definition.ID, definition.UserID, definition.Slug, string(definition.Kind), string(definition.Source), definition.RepoURL, definition.CreatedAt, definition.UpdatedAt); err != nil {
		return normalizeError(err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO skill_revisions (
			id, definition_id, version, title, description, prompt, mode, manifest_json, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, revision.ID, revision.DefinitionID, revision.Version, revision.Title, revision.Description, revision.Prompt, revision.Mode, manifestJSON, revision.CreatedAt); err != nil {
		return normalizeError(err)
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) CreateSkillInstallation(ctx context.Context, installation domain.SkillInstallation) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var revisionTitle string
	if err := tx.QueryRow(ctx, `
		SELECT sr.title
		FROM skill_definitions sd
		JOIN skill_revisions sr ON sr.id = $2 AND sr.definition_id = sd.id
		WHERE sd.id = $1 AND sd.user_id = $3
	`, installation.DefinitionID, installation.CurrentRevisionID, installation.UserID).Scan(&revisionTitle); err != nil {
		return normalizeError(err)
	}
	if strings.TrimSpace(installation.Name) == "" {
		installation.Name = revisionTitle
	}
	var existingCount int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(1)
		FROM skill_installations
		WHERE user_id = $1 AND definition_id = $2
	`, installation.UserID, installation.DefinitionID).Scan(&existingCount); err != nil {
		return err
	}
	if existingCount == 0 {
		installation.IsDefault = true
	}
	if installation.IsDefault {
		if _, err := tx.Exec(ctx, `
			UPDATE skill_installations
			SET is_default = FALSE
			WHERE user_id = $1 AND definition_id = $2 AND is_default = TRUE
		`, installation.UserID, installation.DefinitionID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO skill_installations (
			id, user_id, definition_id, current_revision_id, name, is_default, enabled, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, installation.ID, installation.UserID, installation.DefinitionID, installation.CurrentRevisionID, installation.Name, installation.IsDefault, installation.Enabled, installation.CreatedAt, installation.UpdatedAt); err != nil {
		return normalizeError(err)
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) UpdateSkill(ctx context.Context, skill domain.Skill) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	manifestJSON, err := skillruntime.EncodeRevisionManifest(skill)
	if err != nil {
		return err
	}

	commandTag, err := tx.Exec(ctx, `
		UPDATE skill_definitions
		SET slug = $2,
		    kind = $3,
		    source = $4,
		    repo_url = $5,
		    updated_at = $6
		WHERE id = $1 AND user_id = $7
	`, skill.DefinitionID, skill.Slug, string(skill.Kind), string(skill.Source), skill.RepoURL, skill.UpdatedAt, skill.UserID)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO skill_revisions (
			id, definition_id, version, title, description, prompt, mode, manifest_json, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, skill.RevisionID, skill.DefinitionID, skill.Version, skill.Title, skill.Description, skill.Prompt, skill.Mode, manifestJSON, skill.UpdatedAt); err != nil {
		return err
	}
	commandTag, err = tx.Exec(ctx, `
		UPDATE skill_installations
		SET current_revision_id = $2,
		    enabled = $3,
		    updated_at = $4
		WHERE id = $1 AND user_id = $5
	`, skill.ID, skill.RevisionID, skill.Enabled, skill.UpdatedAt, skill.UserID)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) GetSkill(ctx context.Context, userID, skillID string) (domain.Skill, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT si.id, si.user_id, sd.id, sr.id, sr.version, sd.slug, sd.kind,
		       sr.title, sr.description, sr.prompt, sr.mode, sr.manifest_json, sd.source, si.enabled,
		       sd.repo_url, si.created_at, si.updated_at
		FROM skill_installations si
		JOIN skill_definitions sd ON sd.id = si.definition_id
		JOIN skill_revisions sr ON sr.id = si.current_revision_id
		WHERE si.id = $1 AND si.user_id = $2
	`, skillID, userID)
	return scanSkill(row)
}

func (s *PostgresStore) ListSkills(ctx context.Context, userID string) ([]domain.Skill, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT si.id, si.user_id, sd.id, sr.id, sr.version, sd.slug, sd.kind,
		       sr.title, sr.description, sr.prompt, sr.mode, sr.manifest_json, sd.source, si.enabled,
		       sd.repo_url, si.created_at, si.updated_at
		FROM skill_installations si
		JOIN skill_definitions sd ON sd.id = si.definition_id
		JOIN skill_revisions sr ON sr.id = si.current_revision_id
		WHERE si.user_id = $1
		ORDER BY si.updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	skills := []domain.Skill{}
	for rows.Next() {
		skill, err := scanSkill(rows)
		if err != nil {
			return nil, err
		}
		skills = append(skills, skill)
	}
	return skills, rows.Err()
}

func (s *PostgresStore) GetSkillInstallationRecord(ctx context.Context, userID, installationID string) (domain.SkillInstallationRecord, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT si.id, si.user_id, si.definition_id, si.current_revision_id, si.name, si.is_default, si.enabled, si.created_at, si.updated_at,
		       sd.id, sd.user_id, sd.slug, sd.kind, sd.source, sd.repo_url, sd.created_at, sd.updated_at,
		       sr.id, sr.definition_id, sr.version, sr.title, sr.description, sr.prompt, sr.mode, sr.manifest_json, sr.created_at
		FROM skill_installations si
		JOIN skill_definitions sd ON sd.id = si.definition_id
		JOIN skill_revisions sr ON sr.id = si.current_revision_id
		WHERE si.id = $1 AND si.user_id = $2
	`, installationID, userID)
	return scanSkillInstallationRecord(row)
}

func (s *PostgresStore) GetDefaultSkillInstallationRecord(ctx context.Context, userID, definitionID string) (domain.SkillInstallationRecord, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT si.id, si.user_id, si.definition_id, si.current_revision_id, si.name, si.is_default, si.enabled, si.created_at, si.updated_at,
		       sd.id, sd.user_id, sd.slug, sd.kind, sd.source, sd.repo_url, sd.created_at, sd.updated_at,
		       sr.id, sr.definition_id, sr.version, sr.title, sr.description, sr.prompt, sr.mode, sr.manifest_json, sr.created_at
		FROM skill_installations si
		JOIN skill_definitions sd ON sd.id = si.definition_id
		JOIN skill_revisions sr ON sr.id = si.current_revision_id
		WHERE si.user_id = $1 AND si.definition_id = $2
		ORDER BY si.is_default DESC, si.updated_at DESC
		LIMIT 1
	`, userID, definitionID)
	return scanSkillInstallationRecord(row)
}

func (s *PostgresStore) ListSkillInstallations(ctx context.Context, userID string) ([]domain.SkillInstallationRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT si.id, si.user_id, si.definition_id, si.current_revision_id, si.name, si.is_default, si.enabled, si.created_at, si.updated_at,
		       sd.id, sd.user_id, sd.slug, sd.kind, sd.source, sd.repo_url, sd.created_at, sd.updated_at,
		       sr.id, sr.definition_id, sr.version, sr.title, sr.description, sr.prompt, sr.mode, sr.manifest_json, sr.created_at
		FROM skill_installations si
		JOIN skill_definitions sd ON sd.id = si.definition_id
		JOIN skill_revisions sr ON sr.id = si.current_revision_id
		WHERE si.user_id = $1
		ORDER BY si.updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.SkillInstallationRecord{}
	for rows.Next() {
		item, err := scanSkillInstallationRecord(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) UpdateSkillInstallation(ctx context.Context, installation domain.SkillInstallation) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var definitionID string
	var revisionTitle string
	var currentDefault bool
	if err := tx.QueryRow(ctx, `
		SELECT si.definition_id, sr.title, si.is_default
		FROM skill_installations si
		JOIN skill_revisions sr ON sr.id = $2 AND sr.definition_id = si.definition_id
		WHERE si.id = $1 AND si.user_id = $3
	`, installation.ID, installation.CurrentRevisionID, installation.UserID).Scan(&definitionID, &revisionTitle, &currentDefault); err != nil {
		return normalizeError(err)
	}
	if strings.TrimSpace(installation.Name) == "" {
		installation.Name = revisionTitle
	}
	var promoteSiblingID sql.NullString
	if currentDefault && !installation.IsDefault {
		if err := tx.QueryRow(ctx, `
			SELECT id
			FROM skill_installations
			WHERE user_id = $1 AND definition_id = $2 AND id <> $3
			ORDER BY updated_at DESC, created_at ASC, id ASC
			LIMIT 1
		`, installation.UserID, definitionID, installation.ID).Scan(&promoteSiblingID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if !promoteSiblingID.Valid {
			installation.IsDefault = true
		}
	}
	if installation.IsDefault {
		if _, err := tx.Exec(ctx, `
			UPDATE skill_installations
			SET is_default = FALSE
			WHERE user_id = $1 AND definition_id = $2 AND id <> $3 AND is_default = TRUE
		`, installation.UserID, definitionID, installation.ID); err != nil {
			return err
		}
	}
	commandTag, err := tx.Exec(ctx, `
		UPDATE skill_installations si
		SET current_revision_id = $2,
		    name = $3,
		    is_default = $4,
		    enabled = $5,
		    updated_at = $6
		WHERE si.id = $1
		  AND si.user_id = $7
		  AND EXISTS (
			SELECT 1
			FROM skill_revisions sr
			WHERE sr.id = $2
			  AND sr.definition_id = si.definition_id
		  )
	`, installation.ID, installation.CurrentRevisionID, installation.Name, installation.IsDefault, installation.Enabled, installation.UpdatedAt, installation.UserID)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if promoteSiblingID.Valid {
		if _, err := tx.Exec(ctx, `
			UPDATE skill_installations
			SET is_default = TRUE
			WHERE id = $1
		`, promoteSiblingID.String); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) ListSkillDefinitions(ctx context.Context, userID string) ([]domain.SkillDefinition, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, slug, kind, source, repo_url, created_at, updated_at
		FROM skill_definitions
		WHERE user_id = $1
		ORDER BY updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.SkillDefinition{}
	for rows.Next() {
		item, err := scanSkillDefinition(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) GetSkillDefinitionDetails(ctx context.Context, userID, definitionID string) (domain.SkillDefinitionDetails, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, user_id, slug, kind, source, repo_url, created_at, updated_at
		FROM skill_definitions
		WHERE id = $1 AND user_id = $2
	`, definitionID, userID)
	definition, err := scanSkillDefinition(row)
	if err != nil {
		return domain.SkillDefinitionDetails{}, err
	}

	revisions, err := s.ListSkillRevisions(ctx, userID, definitionID)
	if err != nil {
		return domain.SkillDefinitionDetails{}, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, definition_id, current_revision_id, name, is_default, enabled, created_at, updated_at
		FROM skill_installations
		WHERE user_id = $1 AND definition_id = $2
		ORDER BY is_default DESC, updated_at DESC
	`, userID, definitionID)
	if err != nil {
		return domain.SkillDefinitionDetails{}, err
	}
	defer rows.Close()

	installations := make([]domain.SkillInstallation, 0)
	for rows.Next() {
		item, err := scanSkillInstallation(rows)
		if err != nil {
			return domain.SkillDefinitionDetails{}, err
		}
		installations = append(installations, item)
	}
	if err := rows.Err(); err != nil {
		return domain.SkillDefinitionDetails{}, err
	}

	return domain.SkillDefinitionDetails{
		Definition:    definition,
		Revisions:     revisions,
		Installations: installations,
	}, nil
}

func (s *PostgresStore) ListSkillRevisions(ctx context.Context, userID, definitionID string) ([]domain.SkillRevision, error) {
	var exists int
	if err := s.pool.QueryRow(ctx, `
		SELECT 1
		FROM skill_definitions
		WHERE id = $1 AND user_id = $2
	`, definitionID, userID).Scan(&exists); err != nil {
		return nil, normalizeError(err)
	}
	rows, err := s.pool.Query(ctx, `
		SELECT sr.id, sr.definition_id, sr.version, sr.title, sr.description, sr.prompt, sr.mode, sr.manifest_json, sr.created_at
		FROM skill_revisions sr
		JOIN skill_definitions sd ON sd.id = sr.definition_id
		WHERE sr.definition_id = $1 AND sd.user_id = $2
		ORDER BY sr.version DESC, sr.created_at DESC
	`, definitionID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.SkillRevision{}
	for rows.Next() {
		item, err := scanSkillRevision(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) DeleteSkill(ctx context.Context, userID, skillID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var definitionID string
	var deletedDefault bool
	if err := tx.QueryRow(ctx, `
		SELECT definition_id, is_default
		FROM skill_installations
		WHERE id = $1 AND user_id = $2
	`, skillID, userID).Scan(&definitionID, &deletedDefault); err != nil {
		return normalizeError(err)
	}
	commandTag, err := tx.Exec(ctx, `
		DELETE FROM skill_installations
		WHERE id = $1 AND user_id = $2
	`, skillID, userID)
	if err != nil {
		return normalizeError(err)
	}
	if commandTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if deletedDefault {
		if _, err := tx.Exec(ctx, `
			UPDATE skill_installations
			SET is_default = TRUE
			WHERE id = (
				SELECT id
				FROM skill_installations
				WHERE user_id = $1 AND definition_id = $2
				ORDER BY updated_at DESC, created_at ASC, id ASC
				LIMIT 1
			)
		`, userID, definitionID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) CreateSkillRuntimeSnapshot(ctx context.Context, snapshot domain.SkillRuntimeSnapshot) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO skill_runtime_snapshots (
			id, scope, scope_id, user_id, installation_id, definition_id, revision_id,
			kind, title, description, mode, prompt, runtime_spec_json, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`, snapshot.ID, string(snapshot.Scope), snapshot.ScopeID, snapshot.UserID, snapshot.InstallationID,
		snapshot.DefinitionID, snapshot.RevisionID, string(snapshot.Kind), snapshot.Title, snapshot.Description,
		snapshot.Mode, snapshot.Prompt, snapshot.RuntimeSpecJSON, snapshot.CreatedAt)
	return err
}

func (s *PostgresStore) CreateSkillArtifact(ctx context.Context, artifact domain.SkillArtifact) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO skill_artifacts (
			id, user_id, definition_id, revision_id, source, file_name, media_type, source_url,
			sha256, size_bytes, entry_path, manifest_path, instructions_path, archive_bytes, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`, artifact.ID, artifact.UserID, artifact.DefinitionID, artifact.RevisionID, string(artifact.Source),
		artifact.FileName, artifact.MediaType, artifact.SourceURL, artifact.SHA256, artifact.SizeBytes,
		artifact.EntryPath, artifact.ManifestPath, artifact.InstructionsPath, artifact.ArchiveBytes, artifact.CreatedAt)
	return normalizeError(err)
}

func (s *PostgresStore) GetSkillArtifact(ctx context.Context, userID, artifactID string) (domain.SkillArtifact, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, user_id, definition_id, revision_id, source, file_name, media_type, source_url,
		       sha256, size_bytes, entry_path, manifest_path, instructions_path, archive_bytes, created_at
		FROM skill_artifacts
		WHERE id = $1 AND user_id = $2
	`, artifactID, userID)
	return scanSkillArtifact(row)
}

func (s *PostgresStore) ReplaceSkillArtifactFiles(ctx context.Context, artifactID string, files []domain.SkillArtifactFile) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var userID string
	if err := tx.QueryRow(ctx, `SELECT user_id FROM skill_artifacts WHERE id = $1`, artifactID).Scan(&userID); err != nil {
		return normalizeError(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM skill_artifact_files WHERE artifact_id = $1`, artifactID); err != nil {
		return err
	}
	for _, file := range files {
		if _, err := tx.Exec(ctx, `
			INSERT INTO skill_artifact_files (
				id, artifact_id, user_id, path, media_type, size_bytes, sha256,
				is_manifest, is_instructions, created_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, file.ID, artifactID, userID, file.Path, file.MediaType, file.SizeBytes, file.SHA256,
			file.IsManifest, file.IsInstructions, file.CreatedAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) ListSkillArtifactFiles(ctx context.Context, userID, artifactID string) ([]domain.SkillArtifactFile, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT saff.id, saff.artifact_id, saff.user_id, saff.path, saff.media_type, saff.size_bytes,
		       saff.sha256, saff.is_manifest, saff.is_instructions, saff.created_at
		FROM skill_artifact_files saff
		JOIN skill_artifacts sa ON sa.id = saff.artifact_id
		WHERE saff.artifact_id = $1 AND sa.user_id = $2
		ORDER BY saff.path ASC
	`, artifactID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.SkillArtifactFile{}
	for rows.Next() {
		item, err := scanSkillArtifactFile(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		if _, err := s.GetSkillArtifact(ctx, userID, artifactID); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (s *PostgresStore) CreateSkillImportJob(ctx context.Context, job domain.SkillImportJob) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO skill_import_jobs (
			id, user_id, source, status, artifact_id, definition_id, revision_id, installation_id,
			error_message, request_json, created_at, updated_at, completed_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, job.ID, job.UserID, string(job.Source), string(job.Status), job.ArtifactID, job.DefinitionID,
		job.RevisionID, job.InstallationID, job.ErrorMessage, job.RequestJSON, job.CreatedAt, job.UpdatedAt, job.CompletedAt)
	return normalizeError(err)
}

func (s *PostgresStore) UpdateSkillImportJob(ctx context.Context, job domain.SkillImportJob) error {
	commandTag, err := s.pool.Exec(ctx, `
		UPDATE skill_import_jobs
		SET source = $2,
		    status = $3,
		    artifact_id = $4,
		    definition_id = $5,
		    revision_id = $6,
		    installation_id = $7,
		    error_message = $8,
		    request_json = $9,
		    updated_at = $10,
		    completed_at = $11
		WHERE id = $1 AND user_id = $12
	`, job.ID, string(job.Source), string(job.Status), job.ArtifactID, job.DefinitionID,
		job.RevisionID, job.InstallationID, job.ErrorMessage, job.RequestJSON, job.UpdatedAt, job.CompletedAt, job.UserID)
	if err != nil {
		return normalizeError(err)
	}
	if commandTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) GetSkillImportJob(ctx context.Context, userID, jobID string) (domain.SkillImportJob, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, user_id, source, status, artifact_id, definition_id, revision_id, installation_id,
		       error_message, request_json, created_at, updated_at, completed_at
		FROM skill_import_jobs
		WHERE id = $1 AND user_id = $2
	`, jobID, userID)
	return scanSkillImportJob(row)
}

func (s *PostgresStore) ListSkillImportJobs(ctx context.Context, userID string) ([]domain.SkillImportJob, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, source, status, artifact_id, definition_id, revision_id, installation_id,
		       error_message, request_json, created_at, updated_at, completed_at
		FROM skill_import_jobs
		WHERE user_id = $1
		ORDER BY updated_at DESC, created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.SkillImportJob{}
	for rows.Next() {
		item, err := scanSkillImportJob(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) CreateMirrorTask(ctx context.Context, task domain.MirrorTask) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO mirror_tasks (
			id, kind, user_id, resource_id, status, payload, attempts, last_error,
			next_retry_at, created_at, updated_at, completed_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, task.ID, string(task.Kind), task.UserID, task.ResourceID, string(task.Status), task.Payload,
		task.Attempts, task.LastError, task.NextRetryAt, task.CreatedAt, task.UpdatedAt, task.CompletedAt)
	return err
}

func (s *PostgresStore) UpdateMirrorTask(ctx context.Context, task domain.MirrorTask) error {
	commandTag, err := s.pool.Exec(ctx, `
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
		WHERE id = $1
	`, task.ID, string(task.Kind), task.UserID, task.ResourceID, string(task.Status), task.Payload,
		task.Attempts, task.LastError, task.NextRetryAt, task.CreatedAt, task.UpdatedAt, task.CompletedAt)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) GetMirrorTask(ctx context.Context, taskID string) (domain.MirrorTask, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, kind, user_id, resource_id, status, payload, attempts, last_error,
		       next_retry_at, created_at, updated_at, completed_at
		FROM mirror_tasks
		WHERE id = $1
	`, taskID)
	return scanMirrorTask(row)
}

func (s *PostgresStore) ListDueMirrorTasks(ctx context.Context, dueBefore time.Time, limit int) ([]domain.MirrorTask, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, kind, user_id, resource_id, status, payload, attempts, last_error,
		       next_retry_at, created_at, updated_at, completed_at
		FROM mirror_tasks
		WHERE status = $1 AND next_retry_at <= $2
		ORDER BY next_retry_at ASC, created_at ASC
		LIMIT $3
	`, string(domain.MirrorTaskPending), dueBefore, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := []domain.MirrorTask{}
	for rows.Next() {
		task, err := scanMirrorTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (s *PostgresStore) CreateRun(ctx context.Context, run domain.Run) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO runs (
			id, user_id, title, goal, requested_mode, knowledge_connection_ids,
			skill_installation_id, skill_snapshot_id, effective_mode, status,
			latest_artifact_id, error_message, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`, run.ID, run.UserID, run.Title, run.Goal, string(run.RequestedMode),
		run.KnowledgeConnectionIDs, run.SkillInstallationID, run.SkillSnapshotID,
		string(run.EffectiveMode), string(run.Status), run.LatestArtifactID,
		run.ErrorMessage, run.CreatedAt, run.UpdatedAt)
	return err
}

func (s *PostgresStore) GetRun(ctx context.Context, userID, runID string) (domain.Run, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, user_id, title, goal, requested_mode, knowledge_connection_ids,
		       skill_installation_id, skill_snapshot_id, effective_mode, status,
		       latest_artifact_id, error_message, created_at, updated_at
		FROM runs
		WHERE id = $1 AND user_id = $2
	`, runID, userID)
	return scanRun(row)
}

func (s *PostgresStore) GetRunByID(ctx context.Context, runID string) (domain.Run, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, user_id, title, goal, requested_mode, knowledge_connection_ids,
		       skill_installation_id, skill_snapshot_id, effective_mode, status,
		       latest_artifact_id, error_message, created_at, updated_at
		FROM runs
		WHERE id = $1
	`, runID)
	return scanRun(row)
}

func (s *PostgresStore) UpdateRun(ctx context.Context, run domain.Run) error {
	commandTag, err := s.pool.Exec(ctx, `
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
		WHERE id = $1
	`, run.ID, run.UserID, run.Title, run.Goal, string(run.RequestedMode),
		run.KnowledgeConnectionIDs, run.SkillInstallationID, run.SkillSnapshotID,
		string(run.EffectiveMode), string(run.Status), run.LatestArtifactID,
		run.ErrorMessage, run.CreatedAt, run.UpdatedAt)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) ListRuns(ctx context.Context, userID string) ([]domain.Run, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, title, goal, requested_mode, knowledge_connection_ids,
		       skill_installation_id, skill_snapshot_id, effective_mode, status,
		       latest_artifact_id, error_message, created_at, updated_at
		FROM runs
		WHERE user_id = $1
		ORDER BY updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	runs := []domain.Run{}
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (s *PostgresStore) UpsertRunStep(ctx context.Context, step domain.RunStep) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO run_steps (id, run_id, kind, label, status, summary, started_at, finished_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			kind = EXCLUDED.kind,
			label = EXCLUDED.label,
			status = EXCLUDED.status,
			summary = EXCLUDED.summary,
			started_at = EXCLUDED.started_at,
			finished_at = EXCLUDED.finished_at,
			created_at = EXCLUDED.created_at
	`, step.ID, step.RunID, string(step.Kind), step.Label, string(step.Status), step.Summary, step.StartedAt, step.FinishedAt, step.CreatedAt)
	return err
}

func (s *PostgresStore) ListRunSteps(ctx context.Context, runID string) ([]domain.RunStep, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, run_id, kind, label, status, summary, started_at, finished_at, created_at
		FROM run_steps
		WHERE run_id = $1
		ORDER BY created_at ASC
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	steps := []domain.RunStep{}
	for rows.Next() {
		step, err := scanRunStep(rows)
		if err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	return steps, rows.Err()
}

func (s *PostgresStore) SaveArtifact(ctx context.Context, artifact domain.RunArtifact) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO run_artifacts (id, run_id, kind, content_markdown, version, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, artifact.ID, artifact.RunID, string(artifact.Kind), artifact.ContentMarkdown, artifact.Version, artifact.CreatedAt)
	return err
}

func (s *PostgresStore) ListArtifacts(ctx context.Context, runID string) ([]domain.RunArtifact, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, run_id, kind, content_markdown, version, created_at
		FROM run_artifacts
		WHERE run_id = $1
		ORDER BY created_at ASC
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	artifacts := []domain.RunArtifact{}
	for rows.Next() {
		artifact, err := scanArtifact(rows)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, rows.Err()
}

func (s *PostgresStore) SaveSources(ctx context.Context, runID string, sources []domain.RunSource) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM run_sources WHERE run_id = $1`, runID); err != nil {
		return err
	}
	for _, source := range sources {
		if _, err := tx.Exec(ctx, `
			INSERT INTO run_sources (id, run_id, provider, connection_id, document_id, chunk_id, title, repo, url, snippet, score, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		`, source.ID, source.RunID, string(source.Provider), source.ConnectionID, source.DocumentID, source.ChunkID,
			source.Title, source.Repo, source.URL, source.Snippet, source.Score, source.CreatedAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) ListSources(ctx context.Context, runID string) ([]domain.RunSource, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, run_id, provider, connection_id, document_id, chunk_id, title, repo, url, snippet, score, created_at
		FROM run_sources
		WHERE run_id = $1
		ORDER BY created_at ASC
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sources := []domain.RunSource{}
	for rows.Next() {
		source, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	return sources, rows.Err()
}

func (s *PostgresStore) CreateKnowledgeConnection(ctx context.Context, connection domain.KnowledgeConnection) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		INSERT INTO knowledge_connections (
			id, user_id, provider, name, sync_enabled, last_synced_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, connection.ID, connection.UserID, string(connection.Provider), connection.Name,
		connection.SyncEnabled, connection.LastSyncedAt, connection.CreatedAt, connection.UpdatedAt); err != nil {
		return err
	}
	if err := upsertKnowledgeConnectionConfigs(ctx, tx, connection); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) UpdateKnowledgeConnection(ctx context.Context, connection domain.KnowledgeConnection) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	commandTag, err := tx.Exec(ctx, `
		UPDATE knowledge_connections
		SET user_id = $2,
		    provider = $3,
		    name = $4,
		    sync_enabled = $5,
		    last_synced_at = $6,
		    created_at = $7,
		    updated_at = $8
		WHERE id = $1
	`, connection.ID, connection.UserID, string(connection.Provider), connection.Name,
		connection.SyncEnabled, connection.LastSyncedAt, connection.CreatedAt, connection.UpdatedAt)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err := upsertKnowledgeConnectionConfigs(ctx, tx, connection); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) GetKnowledgeConnection(ctx context.Context, userID, connectionID string) (domain.KnowledgeConnection, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, user_id, provider, name, sync_enabled, last_synced_at, created_at, updated_at
		FROM knowledge_connections
		WHERE id = $1 AND user_id = $2
	`, connectionID, userID)
	connection, err := scanConnection(row)
	if err != nil {
		return domain.KnowledgeConnection{}, err
	}
	if err := loadKnowledgeConnectionConfigs(ctx, s.pool, &connection); err != nil {
		return domain.KnowledgeConnection{}, err
	}
	return connection, nil
}

func (s *PostgresStore) GetKnowledgeConnectionByID(ctx context.Context, connectionID string) (domain.KnowledgeConnection, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, user_id, provider, name, sync_enabled, last_synced_at, created_at, updated_at
		FROM knowledge_connections
		WHERE id = $1
	`, connectionID)
	connection, err := scanConnection(row)
	if err != nil {
		return domain.KnowledgeConnection{}, err
	}
	if err := loadKnowledgeConnectionConfigs(ctx, s.pool, &connection); err != nil {
		return domain.KnowledgeConnection{}, err
	}
	return connection, nil
}

func (s *PostgresStore) ListKnowledgeConnections(ctx context.Context, userID string) ([]domain.KnowledgeConnection, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, provider, name, sync_enabled, last_synced_at, created_at, updated_at
		FROM knowledge_connections
		WHERE user_id = $1
		ORDER BY updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	connections := []domain.KnowledgeConnection{}
	for rows.Next() {
		connection, err := scanConnection(rows)
		if err != nil {
			return nil, err
		}
		if err := loadKnowledgeConnectionConfigs(ctx, s.pool, &connection); err != nil {
			return nil, err
		}
		connections = append(connections, connection)
	}
	return connections, rows.Err()
}

func (s *PostgresStore) DeleteKnowledgeConnection(ctx context.Context, userID, connectionID string) error {
	commandTag, err := s.pool.Exec(ctx, `
		DELETE FROM knowledge_connections
		WHERE id = $1 AND user_id = $2
	`, connectionID, userID)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) CreateKnowledgeSyncJob(ctx context.Context, job domain.KnowledgeSyncJob) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO knowledge_sync_jobs (id, user_id, connection_id, status, summary, created_at, started_at, finished_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, job.ID, job.UserID, job.ConnectionID, string(job.Status), job.Summary, job.CreatedAt, job.StartedAt, job.FinishedAt)
	return err
}

func (s *PostgresStore) UpdateKnowledgeSyncJob(ctx context.Context, job domain.KnowledgeSyncJob) error {
	commandTag, err := s.pool.Exec(ctx, `
		UPDATE knowledge_sync_jobs
		SET user_id = $2,
		    connection_id = $3,
		    status = $4,
		    summary = $5,
		    created_at = $6,
		    started_at = $7,
		    finished_at = $8
		WHERE id = $1
	`, job.ID, job.UserID, job.ConnectionID, string(job.Status), job.Summary, job.CreatedAt, job.StartedAt, job.FinishedAt)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) GetKnowledgeSyncJob(ctx context.Context, jobID string) (domain.KnowledgeSyncJob, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, user_id, connection_id, status, summary, created_at, started_at, finished_at
		FROM knowledge_sync_jobs
		WHERE id = $1
	`, jobID)
	return scanSyncJob(row)
}

func (s *PostgresStore) ListKnowledgeSyncJobs(ctx context.Context, userID, connectionID string) ([]domain.KnowledgeSyncJob, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, connection_id, status, summary, created_at, started_at, finished_at
		FROM knowledge_sync_jobs
		WHERE user_id = $1 AND connection_id = $2
		ORDER BY created_at DESC
	`, userID, connectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := []domain.KnowledgeSyncJob{}
	for rows.Next() {
		job, err := scanSyncJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (s *PostgresStore) ListKnowledgeMetadata(ctx context.Context, connectionID string) ([]domain.KnowledgeDocumentMeta, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, connection_id, repo, title, doc_ref, source_url, chunk_count, updated_at, created_at
		FROM knowledge_document_meta
		WHERE connection_id = $1
		ORDER BY repo ASC, updated_at DESC, title ASC
	`, connectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	docs := []domain.KnowledgeDocumentMeta{}
	for rows.Next() {
		var doc domain.KnowledgeDocumentMeta
		if err := rows.Scan(
			&doc.ID,
			&doc.UserID,
			&doc.ConnectionID,
			&doc.Repo,
			&doc.Title,
			&doc.DocRef,
			&doc.SourceURL,
			&doc.ChunkCount,
			&doc.UpdatedAt,
			&doc.CreatedAt,
		); err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

func (s *PostgresStore) ReplaceKnowledgeMetadata(ctx context.Context, connectionID string, docs []domain.KnowledgeDocumentMeta) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM knowledge_document_meta WHERE connection_id = $1`, connectionID); err != nil {
		return err
	}
	for _, doc := range docs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO knowledge_document_meta (
				id, user_id, connection_id, repo, title, doc_ref, source_url, chunk_count, updated_at, created_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, doc.ID, doc.UserID, doc.ConnectionID, doc.Repo, doc.Title, doc.DocRef,
			nullableString(doc.SourceURL), doc.ChunkCount, doc.UpdatedAt, doc.CreatedAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) ListKnowledgeSourceDocuments(ctx context.Context, connectionID string) ([]domain.KnowledgeSourceDocument, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, connection_id, provider, external_id, repo, title, doc_ref, source_url, raw_body, body_hash, source_updated_at, created_at, updated_at
		FROM knowledge_source_documents
		WHERE connection_id = $1
		ORDER BY source_updated_at DESC, title ASC
	`, connectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	docs := []domain.KnowledgeSourceDocument{}
	for rows.Next() {
		var doc domain.KnowledgeSourceDocument
		var providerName string
		if err := rows.Scan(
			&doc.ID,
			&doc.UserID,
			&doc.ConnectionID,
			&providerName,
			&doc.ExternalID,
			&doc.Repo,
			&doc.Title,
			&doc.DocRef,
			&doc.SourceURL,
			&doc.RawBody,
			&doc.BodyHash,
			&doc.SourceUpdatedAt,
			&doc.CreatedAt,
			&doc.UpdatedAt,
		); err != nil {
			return nil, err
		}
		doc.Provider = domain.Provider(providerName)
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

func (s *PostgresStore) ReplaceKnowledgeSourceDocuments(ctx context.Context, connectionID string, docs []domain.KnowledgeSourceDocument) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM knowledge_source_documents WHERE connection_id = $1`, connectionID); err != nil {
		return err
	}
	for _, doc := range docs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO knowledge_source_documents (
				id, user_id, connection_id, provider, external_id, repo, title, doc_ref, source_url, raw_body, body_hash, source_updated_at, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		`, doc.ID, doc.UserID, doc.ConnectionID, string(doc.Provider), doc.ExternalID, doc.Repo, doc.Title, doc.DocRef,
			nullableString(doc.SourceURL), doc.RawBody, doc.BodyHash, doc.SourceUpdatedAt, doc.CreatedAt, doc.UpdatedAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) GetKnowledgeCorpus(ctx context.Context, connectionID string) ([]domain.KnowledgeDocument, []domain.KnowledgeChunk, error) {
	docRows, err := s.pool.Query(ctx, `
		SELECT id, user_id, connection_id, repo, title, doc_ref, source_url, body_hash, updated_at, created_at
		FROM knowledge_documents
		WHERE connection_id = $1
		ORDER BY created_at ASC, id ASC
	`, connectionID)
	if err != nil {
		return nil, nil, err
	}
	defer docRows.Close()

	docs := []domain.KnowledgeDocument{}
	for docRows.Next() {
		var doc domain.KnowledgeDocument
		if err := docRows.Scan(
			&doc.ID,
			&doc.UserID,
			&doc.ConnectionID,
			&doc.Repo,
			&doc.Title,
			&doc.DocRef,
			&doc.SourceURL,
			&doc.BodyHash,
			&doc.UpdatedAt,
			&doc.CreatedAt,
		); err != nil {
			return nil, nil, err
		}
		docs = append(docs, doc)
	}
	if err := docRows.Err(); err != nil {
		return nil, nil, err
	}

	chunkRows, err := s.pool.Query(ctx, `
		SELECT id, user_id, connection_id, document_id, chunk_index, content, content_hash, created_at
		FROM knowledge_chunks
		WHERE connection_id = $1
		ORDER BY created_at ASC, document_id ASC, chunk_index ASC
	`, connectionID)
	if err != nil {
		return nil, nil, err
	}
	defer chunkRows.Close()

	chunks := []domain.KnowledgeChunk{}
	for chunkRows.Next() {
		var chunk domain.KnowledgeChunk
		if err := chunkRows.Scan(
			&chunk.ID,
			&chunk.UserID,
			&chunk.ConnectionID,
			&chunk.DocumentID,
			&chunk.ChunkIndex,
			&chunk.Content,
			&chunk.ContentHash,
			&chunk.CreatedAt,
		); err != nil {
			return nil, nil, err
		}
		chunks = append(chunks, chunk)
	}
	if err := chunkRows.Err(); err != nil {
		return nil, nil, err
	}

	return docs, chunks, nil
}

func (s *PostgresStore) ReplaceKnowledgeCorpus(ctx context.Context, connectionID string, docs []domain.KnowledgeDocument, chunks []domain.KnowledgeChunk) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM knowledge_documents WHERE connection_id = $1`, connectionID); err != nil {
		return err
	}
	for _, doc := range docs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO knowledge_documents (
				id, user_id, connection_id, repo, title, doc_ref, source_url, body_hash, updated_at, created_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, doc.ID, doc.UserID, doc.ConnectionID, doc.Repo, doc.Title, doc.DocRef,
			nullableString(doc.SourceURL), doc.BodyHash, doc.UpdatedAt, doc.CreatedAt); err != nil {
			return err
		}
	}
	for _, chunk := range chunks {
		if _, err := tx.Exec(ctx, `
			INSERT INTO knowledge_chunks (
				id, user_id, connection_id, document_id, chunk_index, content, content_hash, tsv, created_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, to_tsvector('simple', $6), $8)
		`, chunk.ID, chunk.UserID, chunk.ConnectionID, chunk.DocumentID,
			chunk.ChunkIndex, chunk.Content, chunk.ContentHash, chunk.CreatedAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) SearchKnowledge(ctx context.Context, userID string, connectionIDs []string, query string, limit int) ([]domain.KnowledgeHit, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []domain.KnowledgeHit{}, nil
	}

	sqlText := `
			SELECT kd.connection_id, kd.id, kch.id, kc.provider, kd.title, kd.repo, COALESCE(kd.source_url, ''), kch.content
			FROM knowledge_chunks kch
			INNER JOIN knowledge_documents kd ON kd.id = kch.document_id
			INNER JOIN knowledge_connections kc ON kc.id = kd.connection_id
		WHERE kc.user_id = $1
	`
	args := []any{userID}
	if len(connectionIDs) > 0 {
		sqlText += ` AND kd.connection_id = ANY($2)`
		args = append(args, connectionIDs)
	}
	sqlText += ` ORDER BY kd.updated_at DESC, kch.chunk_index ASC LIMIT 4000`

	rows, err := s.pool.Query(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tokens := tokenize(query)
	loweredQuery := strings.ToLower(query)
	hits := []domain.KnowledgeHit{}
	for rows.Next() {
		var hit domain.KnowledgeHit
		var content string
		if err := rows.Scan(&hit.ConnectionID, &hit.DocumentID, &hit.ChunkID, &hit.Provider, &hit.Title, &hit.Repo, &hit.URL, &content); err != nil {
			return nil, err
		}
		hit.Score = boostedScore(hit.Title, hit.Repo, content, tokens)
		if hit.Score == 0 &&
			!strings.Contains(strings.ToLower(content), loweredQuery) &&
			!strings.Contains(strings.ToLower(hit.Title), loweredQuery) &&
			!strings.Contains(strings.ToLower(hit.Repo), loweredQuery) {
			continue
		}
		if hit.Score == 0 {
			hit.Score = 1
		}
		hit.Snippet = snippet(content, tokens, 220)
		hits = append(hits, hit)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].Title < hits[j].Title
		}
		return hits[i].Score > hits[j].Score
	})
	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

func scanUser(scan scanner) (domain.User, error) {
	var user domain.User
	if err := scan.Scan(&user.ID, &user.Username, &user.DisplayName, &user.Email, &user.AvatarURL, &user.PasswordHash, &user.CreatedAt); err != nil {
		return domain.User{}, normalizeError(err)
	}
	return user, nil
}

func scanSession(scan scanner) (domain.Session, error) {
	var session domain.Session
	var revokedAt sql.NullTime
	if err := scan.Scan(
		&session.ID,
		&session.UserID,
		&session.AccessToken,
		&session.RefreshToken,
		&session.ExpiresAt,
		&revokedAt,
		&session.CreatedAt,
	); err != nil {
		return domain.Session{}, normalizeError(err)
	}
	session.RevokedAt = nullTimePtr(revokedAt)
	return session, nil
}

func scanChatSession(scan scanner) (domain.ChatSession, error) {
	var session domain.ChatSession
	var pinned bool
	var lastMessageAt sql.NullTime
	if err := scan.Scan(
		&session.ID,
		&session.UserID,
		&session.Title,
		&pinned,
		&lastMessageAt,
		&session.CreatedAt,
		&session.UpdatedAt,
	); err != nil {
		return domain.ChatSession{}, normalizeError(err)
	}
	session.Pinned = pinned
	session.LastMessageAt = nullTimePtr(lastMessageAt)
	return session, nil
}

func scanChatMessage(scan scanner) (domain.ChatMessage, error) {
	var message domain.ChatMessage
	var role string
	var completedAt sql.NullTime
	if err := scan.Scan(
		&message.ID,
		&message.SessionID,
		&message.UserID,
		&role,
		&message.Content,
		&message.Skill,
		&message.UseKnowledge,
		&message.CreatedAt,
		&completedAt,
	); err != nil {
		return domain.ChatMessage{}, normalizeError(err)
	}
	message.Role = domain.ChatRole(role)
	message.CompletedAt = nullTimePtr(completedAt)
	return message, nil
}

func scanChatMessageSource(scan scanner) (domain.ChatMessageSource, error) {
	var source domain.ChatMessageSource
	var provider string
	var matchedLinesJSON string
	if err := scan.Scan(
		&source.ID,
		&source.MessageID,
		&provider,
		&source.ConnectionID,
		&source.DocumentID,
		&source.ChunkID,
		&source.Title,
		&source.Repo,
		&source.URL,
		&source.Snippet,
		&matchedLinesJSON,
		&source.Score,
		&source.CreatedAt,
	); err != nil {
		return domain.ChatMessageSource{}, normalizeError(err)
	}
	source.Provider = domain.Provider(provider)
	if strings.TrimSpace(matchedLinesJSON) != "" {
		_ = json.Unmarshal([]byte(matchedLinesJSON), &source.MatchedLines)
	}
	return source, nil
}

func scanSkill(scan scanner) (domain.Skill, error) {
	var skill domain.Skill
	var manifestJSON string
	var source string
	var kind string
	if err := scan.Scan(
		&skill.ID,
		&skill.UserID,
		&skill.DefinitionID,
		&skill.RevisionID,
		&skill.Version,
		&skill.Slug,
		&kind,
		&skill.Title,
		&skill.Description,
		&skill.Prompt,
		&skill.Mode,
		&manifestJSON,
		&source,
		&skill.Enabled,
		&skill.RepoURL,
		&skill.CreatedAt,
		&skill.UpdatedAt,
	); err != nil {
		return domain.Skill{}, normalizeError(err)
	}
	skill.Source = domain.SkillSource(source)
	skill.Kind = domain.SkillKind(kind)
	if err := skillruntime.ApplyRevisionManifest(&skill, manifestJSON); err != nil {
		return domain.Skill{}, err
	}
	return skill, nil
}

func scanSkillDefinition(scan scanner) (domain.SkillDefinition, error) {
	var definition domain.SkillDefinition
	var kind string
	var source string
	if err := scan.Scan(
		&definition.ID,
		&definition.UserID,
		&definition.Slug,
		&kind,
		&source,
		&definition.RepoURL,
		&definition.CreatedAt,
		&definition.UpdatedAt,
	); err != nil {
		return domain.SkillDefinition{}, normalizeError(err)
	}
	definition.Kind = domain.SkillKind(kind)
	definition.Source = domain.SkillSource(source)
	return definition, nil
}

func scanSkillRevision(scan scanner) (domain.SkillRevision, error) {
	var revision domain.SkillRevision
	if err := scan.Scan(
		&revision.ID,
		&revision.DefinitionID,
		&revision.Version,
		&revision.Title,
		&revision.Description,
		&revision.Prompt,
		&revision.Mode,
		&revision.ManifestJSON,
		&revision.CreatedAt,
	); err != nil {
		return domain.SkillRevision{}, normalizeError(err)
	}
	if err := skillruntime.ApplyRevisionManifestToRevision(&revision, revision.ManifestJSON); err != nil {
		return domain.SkillRevision{}, err
	}
	return revision, nil
}

func scanSkillInstallation(scan scanner) (domain.SkillInstallation, error) {
	var installation domain.SkillInstallation
	if err := scan.Scan(
		&installation.ID,
		&installation.UserID,
		&installation.DefinitionID,
		&installation.CurrentRevisionID,
		&installation.Name,
		&installation.IsDefault,
		&installation.Enabled,
		&installation.CreatedAt,
		&installation.UpdatedAt,
	); err != nil {
		return domain.SkillInstallation{}, normalizeError(err)
	}
	return installation, nil
}

func scanSkillInstallationRecord(scan scanner) (domain.SkillInstallationRecord, error) {
	var installation domain.SkillInstallation
	var definition domain.SkillDefinition
	var revision domain.SkillRevision
	var definitionKind string
	var definitionSource string
	var manifestJSON string
	if err := scan.Scan(
		&installation.ID,
		&installation.UserID,
		&installation.DefinitionID,
		&installation.CurrentRevisionID,
		&installation.Name,
		&installation.IsDefault,
		&installation.Enabled,
		&installation.CreatedAt,
		&installation.UpdatedAt,
		&definition.ID,
		&definition.UserID,
		&definition.Slug,
		&definitionKind,
		&definitionSource,
		&definition.RepoURL,
		&definition.CreatedAt,
		&definition.UpdatedAt,
		&revision.ID,
		&revision.DefinitionID,
		&revision.Version,
		&revision.Title,
		&revision.Description,
		&revision.Prompt,
		&revision.Mode,
		&manifestJSON,
		&revision.CreatedAt,
	); err != nil {
		return domain.SkillInstallationRecord{}, normalizeError(err)
	}
	definition.Kind = domain.SkillKind(definitionKind)
	definition.Source = domain.SkillSource(definitionSource)
	revision.ManifestJSON = manifestJSON
	if err := skillruntime.ApplyRevisionManifestToRevision(&revision, manifestJSON); err != nil {
		return domain.SkillInstallationRecord{}, err
	}
	return domain.SkillInstallationRecord{
		Installation:    installation,
		Definition:      definition,
		CurrentRevision: revision,
	}, nil
}

func scanSkillArtifact(scan scanner) (domain.SkillArtifact, error) {
	var artifact domain.SkillArtifact
	var source string
	var archiveBytes []byte
	if err := scan.Scan(
		&artifact.ID,
		&artifact.UserID,
		&artifact.DefinitionID,
		&artifact.RevisionID,
		&source,
		&artifact.FileName,
		&artifact.MediaType,
		&artifact.SourceURL,
		&artifact.SHA256,
		&artifact.SizeBytes,
		&artifact.EntryPath,
		&artifact.ManifestPath,
		&artifact.InstructionsPath,
		&archiveBytes,
		&artifact.CreatedAt,
	); err != nil {
		return domain.SkillArtifact{}, normalizeError(err)
	}
	artifact.Source = domain.SkillSource(source)
	artifact.ArchiveBytes = append([]byte(nil), archiveBytes...)
	return artifact, nil
}

func scanSkillArtifactFile(scan scanner) (domain.SkillArtifactFile, error) {
	var file domain.SkillArtifactFile
	if err := scan.Scan(
		&file.ID,
		&file.ArtifactID,
		&file.UserID,
		&file.Path,
		&file.MediaType,
		&file.SizeBytes,
		&file.SHA256,
		&file.IsManifest,
		&file.IsInstructions,
		&file.CreatedAt,
	); err != nil {
		return domain.SkillArtifactFile{}, normalizeError(err)
	}
	return file, nil
}

func scanSkillImportJob(scan scanner) (domain.SkillImportJob, error) {
	var job domain.SkillImportJob
	var source string
	var status string
	var completedAt sql.NullTime
	if err := scan.Scan(
		&job.ID,
		&job.UserID,
		&source,
		&status,
		&job.ArtifactID,
		&job.DefinitionID,
		&job.RevisionID,
		&job.InstallationID,
		&job.ErrorMessage,
		&job.RequestJSON,
		&job.CreatedAt,
		&job.UpdatedAt,
		&completedAt,
	); err != nil {
		return domain.SkillImportJob{}, normalizeError(err)
	}
	job.Source = domain.SkillSource(source)
	job.Status = domain.SkillImportStatus(status)
	job.CompletedAt = nullTimePtr(completedAt)
	return job, nil
}

func scanMirrorTask(scan scanner) (domain.MirrorTask, error) {
	var task domain.MirrorTask
	var kind string
	var status string
	if err := scan.Scan(
		&task.ID,
		&kind,
		&task.UserID,
		&task.ResourceID,
		&status,
		&task.Payload,
		&task.Attempts,
		&task.LastError,
		&task.NextRetryAt,
		&task.CreatedAt,
		&task.UpdatedAt,
		&task.CompletedAt,
	); err != nil {
		return domain.MirrorTask{}, normalizeError(err)
	}
	task.Kind = domain.MirrorTaskKind(kind)
	task.Status = domain.MirrorTaskStatus(status)
	return task, nil
}

func scanRun(scan scanner) (domain.Run, error) {
	var run domain.Run
	var requestedMode, effectiveMode, status string
	var latestArtifactID, errorMessage string
	var knowledgeConnectionIDs []string
	var skillInstallationID, skillSnapshotID string
	if err := scan.Scan(
		&run.ID,
		&run.UserID,
		&run.Title,
		&run.Goal,
		&requestedMode,
		&knowledgeConnectionIDs,
		&skillInstallationID,
		&skillSnapshotID,
		&effectiveMode,
		&status,
		&latestArtifactID,
		&errorMessage,
		&run.CreatedAt,
		&run.UpdatedAt,
	); err != nil {
		return domain.Run{}, normalizeError(err)
	}
	run.RequestedMode = domain.RunMode(requestedMode)
	run.KnowledgeConnectionIDs = append([]string(nil), knowledgeConnectionIDs...)
	run.SkillInstallationID = skillInstallationID
	run.SkillSnapshotID = skillSnapshotID
	run.EffectiveMode = domain.RunMode(effectiveMode)
	run.Status = domain.RunStatus(status)
	run.LatestArtifactID = latestArtifactID
	run.ErrorMessage = errorMessage
	return run, nil
}

func scanRunStep(scan scanner) (domain.RunStep, error) {
	var step domain.RunStep
	var kind, status string
	var startedAt, finishedAt sql.NullTime
	if err := scan.Scan(
		&step.ID,
		&step.RunID,
		&kind,
		&step.Label,
		&status,
		&step.Summary,
		&startedAt,
		&finishedAt,
		&step.CreatedAt,
	); err != nil {
		return domain.RunStep{}, normalizeError(err)
	}
	step.Kind = domain.StepKind(kind)
	step.Status = domain.StepStatus(status)
	step.StartedAt = nullTimePtr(startedAt)
	step.FinishedAt = nullTimePtr(finishedAt)
	return step, nil
}

func scanArtifact(scan scanner) (domain.RunArtifact, error) {
	var artifact domain.RunArtifact
	var kind string
	if err := scan.Scan(
		&artifact.ID,
		&artifact.RunID,
		&kind,
		&artifact.ContentMarkdown,
		&artifact.Version,
		&artifact.CreatedAt,
	); err != nil {
		return domain.RunArtifact{}, normalizeError(err)
	}
	artifact.Kind = domain.ArtifactKind(kind)
	return artifact, nil
}

func scanSource(scan scanner) (domain.RunSource, error) {
	var source domain.RunSource
	var provider string
	if err := scan.Scan(
		&source.ID,
		&source.RunID,
		&provider,
		&source.ConnectionID,
		&source.DocumentID,
		&source.ChunkID,
		&source.Title,
		&source.Repo,
		&source.URL,
		&source.Snippet,
		&source.Score,
		&source.CreatedAt,
	); err != nil {
		return domain.RunSource{}, normalizeError(err)
	}
	source.Provider = domain.Provider(provider)
	return source, nil
}

func scanUserChatModel(scan scanner) (domain.UserChatModel, error) {
	var model domain.UserChatModel
	var purpose string
	var origin string
	if err := scan.Scan(
		&model.ID,
		&model.UserID,
		&purpose,
		&origin,
		&model.Name,
		&model.BaseURL,
		&model.APIKey,
		&model.ModelName,
		&model.Temperature,
		&model.IsSelected,
		&model.CreatedAt,
		&model.UpdatedAt,
	); err != nil {
		return domain.UserChatModel{}, normalizeError(err)
	}
	model.Purpose = domain.ChatModelPurpose(purpose)
	model.Origin = domain.ChatModelOrigin(origin)
	return model, nil
}

func scanConnection(scan scanner) (domain.KnowledgeConnection, error) {
	var connection domain.KnowledgeConnection
	var provider string
	var lastSyncedAt sql.NullTime
	if err := scan.Scan(
		&connection.ID,
		&connection.UserID,
		&provider,
		&connection.Name,
		&connection.SyncEnabled,
		&lastSyncedAt,
		&connection.CreatedAt,
		&connection.UpdatedAt,
	); err != nil {
		return domain.KnowledgeConnection{}, normalizeError(err)
	}
	connection.Provider = domain.Provider(provider)
	connection.LastSyncedAt = nullTimePtr(lastSyncedAt)
	return connection, nil
}

func upsertKnowledgeConnectionConfigs(ctx context.Context, q dbQuerier, connection domain.KnowledgeConnection) error {
	switch connection.Provider {
	case domain.ProviderYuque:
		if connection.Yuque == nil {
			return errors.New("yuque config is required")
		}
		config := connection.Yuque
		if _, err := q.Exec(ctx, `
			INSERT INTO knowledge_connection_yuque_configs (
				connection_id, token_encrypted, group_login, namespace, pending_docs_json, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (connection_id) DO UPDATE
			SET token_encrypted = EXCLUDED.token_encrypted,
			    group_login = EXCLUDED.group_login,
			    namespace = EXCLUDED.namespace,
			    pending_docs_json = EXCLUDED.pending_docs_json,
			    updated_at = EXCLUDED.updated_at
		`, connection.ID, config.Token, config.GroupLogin, nullableString(config.Namespace), marshalKnowledgeConnectionYuquePendingDocs(config.PendingDocs), config.CreatedAt, config.UpdatedAt); err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `DELETE FROM knowledge_connection_feishu_configs WHERE connection_id = $1`, connection.ID); err != nil {
			return err
		}
	case domain.ProviderFeishu:
		if connection.Feishu == nil {
			return errors.New("feishu config is required")
		}
		config := connection.Feishu
		if _, err := q.Exec(ctx, `
			INSERT INTO knowledge_connection_feishu_configs (
				connection_id, app_id, app_secret_encrypted, entry_type, entry_token, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (connection_id) DO UPDATE
			SET app_id = EXCLUDED.app_id,
			    app_secret_encrypted = EXCLUDED.app_secret_encrypted,
			    entry_type = EXCLUDED.entry_type,
			    entry_token = EXCLUDED.entry_token,
			    updated_at = EXCLUDED.updated_at
		`, connection.ID, config.AppID, config.AppSecret, config.EntryType, config.EntryToken, config.CreatedAt, config.UpdatedAt); err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `DELETE FROM knowledge_connection_yuque_configs WHERE connection_id = $1`, connection.ID); err != nil {
			return err
		}
	default:
		if _, err := q.Exec(ctx, `DELETE FROM knowledge_connection_yuque_configs WHERE connection_id = $1`, connection.ID); err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `DELETE FROM knowledge_connection_feishu_configs WHERE connection_id = $1`, connection.ID); err != nil {
			return err
		}
	}
	return nil
}

func loadKnowledgeConnectionConfigs(ctx context.Context, q dbQuerier, connection *domain.KnowledgeConnection) error {
	connection.Yuque = nil
	connection.Feishu = nil

	switch connection.Provider {
	case domain.ProviderYuque:
		var namespace sql.NullString
		var pendingDocsJSON string
		config := domain.KnowledgeConnectionYuqueConfig{}
		err := q.QueryRow(ctx, `
			SELECT token_encrypted, group_login, namespace, pending_docs_json, created_at, updated_at
			FROM knowledge_connection_yuque_configs
			WHERE connection_id = $1
		`, connection.ID).Scan(
			&config.Token,
			&config.GroupLogin,
			&namespace,
			&pendingDocsJSON,
			&config.CreatedAt,
			&config.UpdatedAt,
		)
		if err != nil {
			return normalizeError(err)
		}
		config.Namespace = nullString(namespace)
		config.PendingDocs = unmarshalKnowledgeConnectionYuquePendingDocs(pendingDocsJSON)
		connection.Yuque = &config
	case domain.ProviderFeishu:
		config := domain.KnowledgeConnectionFeishuConfig{}
		err := q.QueryRow(ctx, `
			SELECT app_id, app_secret_encrypted, entry_type, entry_token, created_at, updated_at
			FROM knowledge_connection_feishu_configs
			WHERE connection_id = $1
		`, connection.ID).Scan(
			&config.AppID,
			&config.AppSecret,
			&config.EntryType,
			&config.EntryToken,
			&config.CreatedAt,
			&config.UpdatedAt,
		)
		if err != nil {
			return normalizeError(err)
		}
		connection.Feishu = &config
	}

	return nil
}

func scanSyncJob(scan scanner) (domain.KnowledgeSyncJob, error) {
	var job domain.KnowledgeSyncJob
	var status string
	var startedAt, finishedAt sql.NullTime
	if err := scan.Scan(
		&job.ID,
		&job.UserID,
		&job.ConnectionID,
		&status,
		&job.Summary,
		&job.CreatedAt,
		&startedAt,
		&finishedAt,
	); err != nil {
		return domain.KnowledgeSyncJob{}, normalizeError(err)
	}
	job.Status = domain.SyncJobStatus(status)
	job.StartedAt = nullTimePtr(startedAt)
	job.FinishedAt = nullTimePtr(finishedAt)
	return job, nil
}

func normalizeError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrConflict
	}
	return err
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nullString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func marshalKnowledgeConnectionYuquePendingDocs(docs []domain.KnowledgeConnectionYuquePendingDoc) string {
	if len(docs) == 0 {
		return "[]"
	}
	encoded, err := json.Marshal(docs)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

func unmarshalKnowledgeConnectionYuquePendingDocs(raw string) []domain.KnowledgeConnectionYuquePendingDoc {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var docs []domain.KnowledgeConnectionYuquePendingDoc
	if err := json.Unmarshal([]byte(raw), &docs); err != nil {
		return nil
	}
	if len(docs) == 0 {
		return nil
	}
	return docs
}

func ensureUserChatModelDefaultsTx(ctx context.Context, tx pgx.Tx, userID string, now time.Time) error {
	defaults := []struct {
		purpose domain.ChatModelPurpose
		name    string
		runtime domain.ChatRuntimeConfig
	}{
		func() struct {
			purpose domain.ChatModelPurpose
			name    string
			runtime domain.ChatRuntimeConfig
		} {
			name, runtime := domain.DefaultChatModelConfigForPurpose(domain.ChatModelPurposeGeneral)
			return struct {
				purpose domain.ChatModelPurpose
				name    string
				runtime domain.ChatRuntimeConfig
			}{purpose: domain.ChatModelPurposeGeneral, name: name, runtime: runtime}
		}(),
		func() struct {
			purpose domain.ChatModelPurpose
			name    string
			runtime domain.ChatRuntimeConfig
		} {
			name, runtime := domain.DefaultChatModelConfigForPurpose(domain.ChatModelPurposeKnowledge)
			return struct {
				purpose domain.ChatModelPurpose
				name    string
				runtime domain.ChatRuntimeConfig
			}{purpose: domain.ChatModelPurposeKnowledge, name: name, runtime: runtime}
		}(),
	}

	for _, item := range defaults {
		if _, err := tx.Exec(ctx, `
				INSERT INTO user_chat_models (id, user_id, purpose, origin, name, base_url, api_key_encrypted, model_name, temperature, is_selected, created_at, updated_at)
				SELECT $1, $2, $3, 'default', $4, $5, $6, $7, $8, TRUE, $9, $9
				WHERE NOT EXISTS (
					SELECT 1 FROM user_chat_models WHERE user_id = $2 AND purpose = $3
				)
			`, uuid.NewString(), userID, string(item.purpose), item.name, item.runtime.BaseURL, item.runtime.APIKey, item.runtime.ModelName, item.runtime.Temperature, now); err != nil {
			return err
		}

		var selectedCount int
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM user_chat_models
			WHERE user_id = $1 AND purpose = $2 AND is_selected = TRUE
		`, userID, string(item.purpose)).Scan(&selectedCount); err != nil {
			return err
		}
		if selectedCount == 1 {
			continue
		}

		var fallbackID string
		if err := tx.QueryRow(ctx, `
			SELECT id
			FROM user_chat_models
			WHERE user_id = $1 AND purpose = $2
			ORDER BY created_at ASC, name ASC
			LIMIT 1
		`, userID, string(item.purpose)).Scan(&fallbackID); err != nil {
			return normalizeError(err)
		}
		if err := setSelectedUserChatModelTx(ctx, tx, userID, item.purpose, fallbackID, now); err != nil {
			return err
		}
	}
	return nil
}

func setSelectedUserChatModelTx(
	ctx context.Context,
	tx pgx.Tx,
	userID string,
	purpose domain.ChatModelPurpose,
	modelID string,
	now time.Time,
) error {
	if _, err := tx.Exec(ctx, `
		UPDATE user_chat_models
		SET is_selected = FALSE,
		    updated_at = $3
		WHERE user_id = $1 AND purpose = $2 AND is_selected = TRUE
	`, userID, string(purpose), now); err != nil {
		return err
	}

	commandTag, err := tx.Exec(ctx, `
		UPDATE user_chat_models
		SET is_selected = TRUE,
		    updated_at = $4
		WHERE user_id = $1 AND purpose = $2 AND id = $3
	`, userID, string(purpose), modelID, now)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func getUserChatModelTx(ctx context.Context, tx pgx.Tx, userID, modelID string) (domain.UserChatModel, error) {
	row := tx.QueryRow(ctx, `
		SELECT id, user_id, purpose, origin, name, base_url, api_key_encrypted, model_name, temperature, is_selected, created_at, updated_at
		FROM user_chat_models
		WHERE id = $1 AND user_id = $2
	`, modelID, userID)
	return scanUserChatModel(row)
}
