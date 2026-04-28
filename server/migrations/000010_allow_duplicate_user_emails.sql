-- +goose Up
DROP INDEX IF EXISTS idx_users_email_unique;

-- +goose Down
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_unique
  ON users (lower(email))
  WHERE email <> '';
