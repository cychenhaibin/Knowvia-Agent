package store

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/bootstrap"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newContractPostgresStore(t *testing.T) contractStore {
	t.Helper()

	baseDSN := strings.TrimSpace(os.Getenv("QQA_TEST_POSTGRES_DSN"))
	if baseDSN == "" {
		t.Skip("QQA_TEST_POSTGRES_DSN is not set; skipping postgres store contract tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	adminPool, err := pgxpool.New(ctx, baseDSN)
	if err != nil {
		t.Fatalf("connect admin postgres pool: %v", err)
	}

	schemaName := fmt.Sprintf("contract_%d", time.Now().UnixNano())
	if _, err := adminPool.Exec(ctx, fmt.Sprintf(`CREATE SCHEMA %s`, schemaName)); err != nil {
		adminPool.Close()
		t.Fatalf("create isolated schema: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = adminPool.Exec(cleanupCtx, fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE`, schemaName))
		adminPool.Close()
	})

	cfg, err := pgxpool.ParseConfig(baseDSN)
	if err != nil {
		t.Fatalf("parse postgres dsn: %v", err)
	}
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schemaName + ",public"
	schemaDSN := cfg.ConnString()

	if err := bootstrap.MigratePostgres(ctx, schemaDSN); err != nil {
		t.Fatalf("migrate isolated schema: %v", err)
	}

	s, err := NewPostgresStore(ctx, schemaDSN)
	if err != nil {
		t.Fatalf("create postgres store: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}
