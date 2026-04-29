-- +goose Up
CREATE TABLE IF NOT EXISTS user_chat_models (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  purpose TEXT NOT NULL,
  model_name TEXT NOT NULL,
  is_selected BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, purpose, model_name)
);

CREATE INDEX IF NOT EXISTS idx_user_chat_models_user_purpose
  ON user_chat_models (user_id, purpose, created_at ASC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_chat_models_selected
  ON user_chat_models (user_id, purpose)
  WHERE is_selected;

ALTER TABLE user_chat_models
  ADD COLUMN IF NOT EXISTS name TEXT,
  ADD COLUMN IF NOT EXISTS base_url TEXT,
  ADD COLUMN IF NOT EXISTS api_key_encrypted TEXT,
  ADD COLUMN IF NOT EXISTS origin TEXT,
  ADD COLUMN IF NOT EXISTS temperature DOUBLE PRECISION;

UPDATE user_chat_models
SET
  name = COALESCE(name, ''),
  base_url = COALESCE(base_url, ''),
  api_key_encrypted = COALESCE(api_key_encrypted, ''),
  origin = COALESCE(origin, 'custom'),
  temperature = COALESCE(temperature, 0.05);

ALTER TABLE user_chat_models
  ALTER COLUMN name SET DEFAULT '',
  ALTER COLUMN name SET NOT NULL,
  ALTER COLUMN base_url SET DEFAULT '',
  ALTER COLUMN base_url SET NOT NULL,
  ALTER COLUMN api_key_encrypted SET DEFAULT '',
  ALTER COLUMN api_key_encrypted SET NOT NULL,
  ALTER COLUMN origin SET DEFAULT 'custom',
  ALTER COLUMN origin SET NOT NULL,
  ALTER COLUMN temperature SET DEFAULT 0.05,
  ALTER COLUMN temperature SET NOT NULL;

INSERT INTO user_chat_models (id, user_id, purpose, model_name, is_selected, created_at, updated_at)
SELECT users.id || ':general:default', users.id, 'general', 'gemma3n:e4b', TRUE, NOW(), NOW()
FROM users
WHERE NOT EXISTS (
  SELECT 1
  FROM user_chat_models
  WHERE user_id = users.id AND purpose = 'general'
);

INSERT INTO user_chat_models (id, user_id, purpose, model_name, is_selected, created_at, updated_at)
SELECT users.id || ':knowledge:default', users.id, 'knowledge', 'qwen3:8b', TRUE, NOW(), NOW()
FROM users
WHERE NOT EXISTS (
  SELECT 1
  FROM user_chat_models
  WHERE user_id = users.id AND purpose = 'knowledge'
);

-- +goose Down
DROP TABLE IF EXISTS user_chat_models;
