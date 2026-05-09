package bootstrap

import (
	"strings"
	"testing"
)

func TestExtractUpMigrationSQLGooseMarkers(t *testing.T) {
	content := `-- +goose Up
CREATE TABLE demo (id TEXT PRIMARY KEY);
-- +goose Down
DROP TABLE demo;`

	got := ExtractUpMigrationSQL(content)
	if strings.Contains(got, "DROP TABLE") {
		t.Fatalf("expected down section to be excluded, got %q", got)
	}
	if !strings.Contains(got, "CREATE TABLE demo") {
		t.Fatalf("expected up section to be kept, got %q", got)
	}
}

func TestExtractUpMigrationSQLSeparator(t *testing.T) {
	content := `ALTER TABLE demo ADD COLUMN note TEXT;
---- create above / drop below ----
ALTER TABLE demo DROP COLUMN note;`

	got := ExtractUpMigrationSQL(content)
	if strings.Contains(got, "DROP COLUMN") {
		t.Fatalf("expected separator to stop extraction, got %q", got)
	}
	if !strings.Contains(got, "ADD COLUMN") {
		t.Fatalf("expected create section to be kept, got %q", got)
	}
}

func TestLoadEmbeddedMigrationsIncludesLatestMigration(t *testing.T) {
	migrations, err := loadEmbeddedMigrations()
	if err != nil {
		t.Fatalf("load embedded migrations: %v", err)
	}
	if len(migrations) == 0 {
		t.Fatal("expected embedded migrations to be discovered")
	}
	if migrations[len(migrations)-1].name != "000023_github_repo_analysis_tasks.sql" {
		t.Fatalf("expected latest migration to be github repo analysis tasks, got %s", migrations[len(migrations)-1].name)
	}
}
