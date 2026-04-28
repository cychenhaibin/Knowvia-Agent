CREATE TABLE IF NOT EXISTS knowledge_source_documents (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  connection_id TEXT NOT NULL REFERENCES knowledge_connections(id) ON DELETE CASCADE,
  provider TEXT NOT NULL,
  external_id TEXT NOT NULL,
  repo TEXT NOT NULL,
  title TEXT NOT NULL,
  doc_ref TEXT NOT NULL,
  source_url TEXT NULL,
  raw_body TEXT NOT NULL,
  body_hash TEXT NOT NULL,
  source_updated_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_source_docs_connection_external
  ON knowledge_source_documents(connection_id, external_id);

CREATE INDEX IF NOT EXISTS idx_source_docs_connection_updated
  ON knowledge_source_documents(connection_id, source_updated_at DESC);
