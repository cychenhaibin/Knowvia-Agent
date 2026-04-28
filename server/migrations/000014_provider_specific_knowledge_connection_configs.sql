CREATE TABLE IF NOT EXISTS knowledge_connection_yuque_configs (
  connection_id TEXT PRIMARY KEY REFERENCES knowledge_connections(id) ON DELETE CASCADE,
  token_encrypted TEXT NOT NULL,
  group_login TEXT NOT NULL,
  namespace TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

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
  AND NOT EXISTS (
    SELECT 1
    FROM knowledge_connection_yuque_configs cfg
    WHERE cfg.connection_id = knowledge_connections.id
  );

CREATE TABLE IF NOT EXISTS knowledge_connection_feishu_configs (
  connection_id TEXT PRIMARY KEY REFERENCES knowledge_connections(id) ON DELETE CASCADE,
  app_id TEXT NOT NULL,
  app_secret_encrypted TEXT NOT NULL,
  document_id TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE knowledge_connections DROP COLUMN IF EXISTS token_encrypted;
ALTER TABLE knowledge_connections DROP COLUMN IF EXISTS group_login;
ALTER TABLE knowledge_connections DROP COLUMN IF EXISTS namespace;
