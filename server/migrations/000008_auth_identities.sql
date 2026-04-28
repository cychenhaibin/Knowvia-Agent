-- +goose Up
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS email TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS avatar_url TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS auth_identities (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider TEXT NOT NULL,
  provider_subject TEXT NOT NULL,
  email TEXT NOT NULL DEFAULT '',
  email_verified BOOLEAN NOT NULL DEFAULT FALSE,
  avatar_url TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (provider, provider_subject)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_unique
  ON users (lower(email))
  WHERE email <> '';

CREATE INDEX IF NOT EXISTS idx_auth_identities_user_provider
  ON auth_identities (user_id, provider);

-- +goose Down
DROP INDEX IF EXISTS idx_auth_identities_user_provider;
DROP INDEX IF EXISTS idx_users_email_unique;
DROP TABLE IF EXISTS auth_identities;
ALTER TABLE users
  DROP COLUMN IF EXISTS avatar_url,
  DROP COLUMN IF EXISTS email;
