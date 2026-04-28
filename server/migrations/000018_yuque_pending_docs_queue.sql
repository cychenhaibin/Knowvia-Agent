ALTER TABLE knowledge_connection_yuque_configs
  ADD COLUMN IF NOT EXISTS pending_docs_json TEXT NOT NULL DEFAULT '[]';
