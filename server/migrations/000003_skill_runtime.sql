-- +goose Up
CREATE TABLE IF NOT EXISTS skill_definitions (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  slug TEXT NOT NULL,
  kind TEXT NOT NULL DEFAULT 'chat_profile',
  source TEXT NOT NULL,
  repo_url TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, slug)
);

CREATE TABLE IF NOT EXISTS skill_revisions (
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
);

CREATE TABLE IF NOT EXISTS skill_installations (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  definition_id TEXT NOT NULL REFERENCES skill_definitions(id) ON DELETE CASCADE,
  current_revision_id TEXT NOT NULL REFERENCES skill_revisions(id) ON DELETE RESTRICT,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, definition_id)
);

CREATE TABLE IF NOT EXISTS skill_runtime_snapshots (
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
);

ALTER TABLE runs ADD COLUMN IF NOT EXISTS skill_installation_id TEXT NOT NULL DEFAULT '';
ALTER TABLE runs ADD COLUMN IF NOT EXISTS skill_snapshot_id TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_skill_definitions_user_updated
  ON skill_definitions(user_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_skill_installations_user_updated
  ON skill_installations(user_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_skill_snapshots_scope_created
  ON skill_runtime_snapshots(scope, scope_id, created_at DESC);

INSERT INTO skill_definitions (id, user_id, slug, kind, source, repo_url, created_at, updated_at)
SELECT
  'legacy-def:' || id,
  user_id,
  slug,
  'chat_profile',
  source,
  repo_url,
  created_at,
  updated_at
FROM skills
ON CONFLICT (id) DO NOTHING;

INSERT INTO skill_revisions (id, definition_id, version, title, description, prompt, mode, manifest_json, created_at)
SELECT
  'legacy-rev:' || id || ':1',
  'legacy-def:' || id,
  1,
  title,
  description,
  prompt,
  mode,
  '{}',
  updated_at
FROM skills
ON CONFLICT (id) DO NOTHING;

INSERT INTO skill_installations (id, user_id, definition_id, current_revision_id, enabled, created_at, updated_at)
SELECT
  id,
  user_id,
  'legacy-def:' || id,
  'legacy-rev:' || id || ':1',
  enabled,
  created_at,
  updated_at
FROM skills
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DROP INDEX IF EXISTS idx_skill_snapshots_scope_created;
DROP INDEX IF EXISTS idx_skill_installations_user_updated;
DROP INDEX IF EXISTS idx_skill_definitions_user_updated;
ALTER TABLE runs DROP COLUMN IF EXISTS skill_snapshot_id;
ALTER TABLE runs DROP COLUMN IF EXISTS skill_installation_id;
DROP TABLE IF EXISTS skill_runtime_snapshots;
DROP TABLE IF EXISTS skill_installations;
DROP TABLE IF EXISTS skill_revisions;
DROP TABLE IF EXISTS skill_definitions;
