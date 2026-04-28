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
