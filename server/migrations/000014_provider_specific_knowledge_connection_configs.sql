CREATE TABLE IF NOT EXISTS knowledge_connection_yuque_configs (
  connection_id TEXT PRIMARY KEY REFERENCES knowledge_connections(id) ON DELETE CASCADE,
  token_encrypted TEXT NOT NULL,
  group_login TEXT NOT NULL,
  namespace TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS knowledge_connection_feishu_configs (
  connection_id TEXT PRIMARY KEY REFERENCES knowledge_connections(id) ON DELETE CASCADE,
  app_id TEXT NOT NULL,
  app_secret_encrypted TEXT NOT NULL,
  document_id TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DO $$
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
END$$;

ALTER TABLE knowledge_connections DROP COLUMN IF EXISTS token_encrypted;
ALTER TABLE knowledge_connections DROP COLUMN IF EXISTS group_login;
ALTER TABLE knowledge_connections DROP COLUMN IF EXISTS namespace;
