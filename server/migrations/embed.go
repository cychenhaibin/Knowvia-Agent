package migrations

import "embed"

// Files embeds the SQL migration files so runtime schema setup uses the same
// source of truth as local development.
//
//go:embed *.sql
var Files embed.FS
