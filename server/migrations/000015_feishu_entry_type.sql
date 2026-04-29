ALTER TABLE knowledge_connection_feishu_configs
  ADD COLUMN IF NOT EXISTS entry_type TEXT NOT NULL DEFAULT 'docx';

ALTER TABLE knowledge_connection_feishu_configs
  ADD COLUMN IF NOT EXISTS entry_token TEXT;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = current_schema()
      AND table_name = 'knowledge_connection_feishu_configs'
      AND column_name = 'document_id'
  ) THEN
    UPDATE knowledge_connection_feishu_configs
    SET entry_token = document_id
    WHERE (entry_token IS NULL OR entry_token = '')
      AND document_id IS NOT NULL
      AND document_id <> '';
  END IF;
END$$;

ALTER TABLE knowledge_connection_feishu_configs
  ALTER COLUMN entry_token SET NOT NULL;

ALTER TABLE knowledge_connection_feishu_configs
  DROP COLUMN IF EXISTS document_id;
