-- +goose Up
ALTER TABLE skill_installations
  ADD COLUMN IF NOT EXISTS name TEXT NOT NULL DEFAULT '';

ALTER TABLE skill_installations
  ADD COLUMN IF NOT EXISTS is_default BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE skill_installations si
SET name = sr.title
FROM skill_revisions sr
WHERE si.current_revision_id = sr.id
  AND si.name = '';

UPDATE skill_installations si
SET is_default = TRUE
WHERE NOT EXISTS (
  SELECT 1
  FROM skill_installations existing
  WHERE existing.user_id = si.user_id
    AND existing.definition_id = si.definition_id
    AND existing.is_default = TRUE
)
  AND si.id = (
    SELECT candidate.id
    FROM skill_installations candidate
    WHERE candidate.user_id = si.user_id
      AND candidate.definition_id = si.definition_id
    ORDER BY candidate.updated_at DESC, candidate.created_at ASC, candidate.id ASC
    LIMIT 1
  );

CREATE UNIQUE INDEX IF NOT EXISTS idx_skill_installations_user_definition_default
  ON skill_installations(user_id, definition_id)
  WHERE is_default = TRUE;

-- +goose Down
DROP INDEX IF EXISTS idx_skill_installations_user_definition_default;

ALTER TABLE skill_installations
  DROP COLUMN IF EXISTS is_default;

ALTER TABLE skill_installations
  DROP COLUMN IF EXISTS name;
