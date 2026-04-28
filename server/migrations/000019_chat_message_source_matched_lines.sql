ALTER TABLE chat_message_sources
  ADD COLUMN IF NOT EXISTS matched_lines_json TEXT NOT NULL DEFAULT '[]';
