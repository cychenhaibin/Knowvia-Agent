-- +migrate Up
ALTER TABLE run_sources ADD COLUMN IF NOT EXISTS connection_id TEXT NOT NULL DEFAULT '';
ALTER TABLE run_sources ADD COLUMN IF NOT EXISTS document_id TEXT NOT NULL DEFAULT '';
ALTER TABLE run_sources ADD COLUMN IF NOT EXISTS chunk_id TEXT NOT NULL DEFAULT '';

-- +migrate Down
ALTER TABLE run_sources DROP COLUMN IF EXISTS chunk_id;
ALTER TABLE run_sources DROP COLUMN IF EXISTS document_id;
ALTER TABLE run_sources DROP COLUMN IF EXISTS connection_id;
