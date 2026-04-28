ALTER TABLE knowledge_connection_feishu_configs
  ADD COLUMN IF NOT EXISTS entry_type TEXT NOT NULL DEFAULT 'docx';

ALTER TABLE knowledge_connection_feishu_configs
  ADD COLUMN IF NOT EXISTS entry_token TEXT;

UPDATE knowledge_connection_feishu_configs
SET entry_token = document_id
WHERE (entry_token IS NULL OR entry_token = '')
  AND document_id IS NOT NULL
  AND document_id <> '';

ALTER TABLE knowledge_connection_feishu_configs
  ALTER COLUMN entry_token SET NOT NULL;

ALTER TABLE knowledge_connection_feishu_configs
  DROP COLUMN IF EXISTS document_id;
