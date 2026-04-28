ALTER TABLE user_chat_models
  ADD COLUMN IF NOT EXISTS name TEXT NOT NULL DEFAULT '';

ALTER TABLE user_chat_models
  ADD COLUMN IF NOT EXISTS base_url TEXT NOT NULL DEFAULT '';

ALTER TABLE user_chat_models
  ADD COLUMN IF NOT EXISTS api_key_encrypted TEXT NOT NULL DEFAULT '';

UPDATE user_chat_models
SET name = CASE
  WHEN purpose = 'knowledge' THEN 'Qwen3 8B'
  ELSE 'Gemma 3n E4B'
END
WHERE name = '';

UPDATE user_chat_models
SET base_url = 'http://127.0.0.1:11434/v1'
WHERE base_url = '';

UPDATE user_chat_models
SET api_key_encrypted = 'ollama'
WHERE api_key_encrypted = '';

ALTER TABLE user_chat_models
  DROP CONSTRAINT IF EXISTS user_chat_models_user_id_purpose_model_name_key;

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_chat_models_user_purpose_name
  ON user_chat_models (user_id, purpose, lower(name));

---- create above / drop below ----

DROP INDEX IF EXISTS idx_user_chat_models_user_purpose_name;

ALTER TABLE user_chat_models
  DROP COLUMN IF EXISTS api_key_encrypted;

ALTER TABLE user_chat_models
  DROP COLUMN IF EXISTS base_url;

ALTER TABLE user_chat_models
  DROP COLUMN IF EXISTS name;
