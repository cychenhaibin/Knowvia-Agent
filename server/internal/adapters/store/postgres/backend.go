package postgres

import (
	"context"

	basestore "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/store"
)

type Store = basestore.PostgresStore

func New(ctx context.Context, dsn string) (*Store, error) {
	return basestore.NewPostgresStore(ctx, dsn)
}
