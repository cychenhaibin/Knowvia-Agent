-- +goose Up
ALTER TABLE skill_installations
  DROP CONSTRAINT IF EXISTS skill_installations_user_id_definition_id_key;

-- +goose Down
ALTER TABLE skill_installations
  ADD CONSTRAINT skill_installations_user_id_definition_id_key UNIQUE (user_id, definition_id);
