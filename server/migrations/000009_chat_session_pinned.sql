-- +goose Up
ALTER TABLE chat_sessions
  ADD COLUMN IF NOT EXISTS pinned BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_chat_sessions_user_pinned_updated
  ON chat_sessions(user_id, pinned DESC, updated_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_chat_sessions_user_pinned_updated;

ALTER TABLE chat_sessions
  DROP COLUMN IF EXISTS pinned;
