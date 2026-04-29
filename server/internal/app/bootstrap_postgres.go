package app

import (
	"context"
	"fmt"

	postgresbackend "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/store/postgres"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/bootstrap"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/config"
)

func bootstrapPostgresServices(ctx context.Context, cfg config.Config) (*Services, error) {
	if err := bootstrap.MigratePostgres(ctx, cfg.PostgresDSN); err != nil {
		return nil, fmt.Errorf("migrate postgres store: %w", err)
	}
	postgresStore, err := postgresbackend.New(ctx, cfg.PostgresDSN)
	if err != nil {
		return nil, fmt.Errorf("connect store backend postgres: %w", err)
	}
	return bootstrapServices(
		cfg,
		postgresStore.Close,
		postgresStore,
		postgresStore,
		postgresStore,
		postgresStore,
		postgresStore,
		postgresStore,
		postgresStore,
		postgresStore,
	), nil
}
