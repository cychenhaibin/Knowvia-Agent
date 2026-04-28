ALTER TABLE user_chat_models
  ADD COLUMN IF NOT EXISTS origin TEXT NOT NULL DEFAULT 'custom';

UPDATE user_chat_models
SET origin = 'default'
WHERE origin = 'custom'
  AND name IN ('Gemma 3n E4B', 'Qwen3 8B')
  AND base_url = 'http://127.0.0.1:11434/v1'
  AND api_key_encrypted = 'ollama'
  AND (
    (name = 'Gemma 3n E4B' AND model_name = 'gemma3n:e4b')
    OR (name = 'Qwen3 8B' AND model_name = 'qwen3:8b')
  );

---- create above / drop below ----

ALTER TABLE user_chat_models
  DROP COLUMN IF EXISTS origin;
