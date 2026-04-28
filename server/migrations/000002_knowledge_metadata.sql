-- +goose Up
CREATE TABLE IF NOT EXISTS knowledge_document_meta (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  connection_id TEXT NOT NULL REFERENCES knowledge_connections(id) ON DELETE CASCADE,
  repo TEXT NOT NULL,
  title TEXT NOT NULL,
  doc_ref TEXT NOT NULL,
  source_url TEXT NULL,
  chunk_count INTEGER NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_doc_meta_connection_updated
  ON knowledge_document_meta(connection_id, updated_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_doc_meta_connection_updated;
DROP TABLE IF EXISTS knowledge_document_meta;
