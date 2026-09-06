-- +goose Up
CREATE TABLE fluxa_credentials (
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  site TEXT NOT NULL CHECK (site IN ('paid', 'free')),
  token_ciphertext TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (user_id, site)
);

-- +goose Down
DROP TABLE fluxa_credentials;
