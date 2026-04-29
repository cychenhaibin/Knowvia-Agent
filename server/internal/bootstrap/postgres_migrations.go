package bootstrap

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	migrationfiles "github.com/chenhaibin/yuque-rag/quickque-agent/server/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

type embeddedMigration struct {
	name string
	sql  string
}

func MigratePostgres(ctx context.Context, dsn string) error {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect postgres for migrations: %w", err)
	}
	defer pool.Close()
	return applyPostgresMigrations(ctx, pool)
}

func applyPostgresMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}
	migrations, err := loadEmbeddedMigrations()
	if err != nil {
		return err
	}
	applied, err := listAppliedMigrations(ctx, pool)
	if err != nil {
		return err
	}
	for _, migration := range migrations {
		if applied[migration.name] {
			continue
		}
		if err := applyMigration(ctx, pool, migration); err != nil {
			return fmt.Errorf("apply migration %s: %w", migration.name, err)
		}
	}
	return nil
}

func loadEmbeddedMigrations() ([]embeddedMigration, error) {
	names, err := fs.Glob(migrationfiles.Files, "*.sql")
	if err != nil {
		return nil, fmt.Errorf("list embedded migrations: %w", err)
	}
	sort.Strings(names)
	migrations := make([]embeddedMigration, 0, len(names))
	for _, name := range names {
		content, err := fs.ReadFile(migrationfiles.Files, name)
		if err != nil {
			return nil, fmt.Errorf("read embedded migration %s: %w", name, err)
		}
		sql := ExtractUpMigrationSQL(string(content))
		if strings.TrimSpace(sql) == "" {
			continue
		}
		migrations = append(migrations, embeddedMigration{name: name, sql: sql})
	}
	return migrations, nil
}

func ExtractUpMigrationSQL(content string) string {
	lines := strings.Split(content, "\n")
	hasExplicitUp := strings.Contains(content, "+goose Up") || strings.Contains(content, "+migrate Up")
	collecting := !hasExplicitUp
	var builder strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "-- +goose Up"), strings.HasPrefix(trimmed, "-- +migrate Up"):
			collecting = true
			continue
		case strings.HasPrefix(trimmed, "-- +goose Down"), strings.HasPrefix(trimmed, "-- +migrate Down"):
			collecting = false
			goto done
		case trimmed == "---- create above / drop below ----":
			if !hasExplicitUp {
				collecting = false
				goto done
			}
			continue
		}
		if collecting {
			builder.WriteString(line)
			builder.WriteByte('\n')
		}
	}

done:
	return strings.TrimSpace(builder.String())
}

func listAppliedMigrations(ctx context.Context, pool *pgxpool.Pool) (map[string]bool, error) {
	rows, err := pool.Query(ctx, `SELECT name FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("list applied migrations: %w", err)
	}
	defer rows.Close()

	applied := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan applied migration: %w", err)
		}
		applied[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", err)
	}
	return applied, nil
}

func applyMigration(ctx context.Context, pool *pgxpool.Pool, migration embeddedMigration) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, migration.sql); err != nil {
		return fmt.Errorf("exec migration sql: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO schema_migrations (name)
		VALUES ($1)
	`, migration.name); err != nil {
		return fmt.Errorf("record applied migration: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}
	return nil
}
