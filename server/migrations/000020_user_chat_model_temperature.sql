ALTER TABLE user_chat_models
  ADD COLUMN IF NOT EXISTS temperature DOUBLE PRECISION NOT NULL DEFAULT 0.05;

UPDATE user_chat_models
SET temperature = 0.05
WHERE temperature IS NULL;

---- create above / drop below ----

ALTER TABLE user_chat_models
  DROP COLUMN IF EXISTS temperature;
