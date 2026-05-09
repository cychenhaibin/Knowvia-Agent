-- +goose Up
ALTER TABLE runs
  ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'research',
  ADD COLUMN IF NOT EXISTS source_url TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS task_session_id TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS task_prompt TEXT NOT NULL DEFAULT '';

ALTER TABLE chat_sessions
  ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'chat',
  ADD COLUMN IF NOT EXISTS run_id TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_runs_user_kind_updated
  ON runs(user_id, kind, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_chat_sessions_user_kind_updated
  ON chat_sessions(user_id, kind, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_chat_sessions_run
  ON chat_sessions(run_id)
  WHERE run_id <> '';

-- +goose Down
DROP INDEX IF EXISTS idx_chat_sessions_run;
DROP INDEX IF EXISTS idx_chat_sessions_user_kind_updated;
DROP INDEX IF EXISTS idx_runs_user_kind_updated;

ALTER TABLE chat_sessions
  DROP COLUMN IF EXISTS run_id,
  DROP COLUMN IF EXISTS kind;

ALTER TABLE runs
  DROP COLUMN IF EXISTS task_prompt,
  DROP COLUMN IF EXISTS task_session_id,
  DROP COLUMN IF EXISTS source_url,
  DROP COLUMN IF EXISTS kind;
