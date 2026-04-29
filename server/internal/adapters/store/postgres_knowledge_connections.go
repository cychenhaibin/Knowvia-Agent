package store

import (
	"context"
	"errors"

	sqldb "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/db"
	"github.com/jackc/pgx/v5"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *PostgresStore) CreateKnowledgeConnection(ctx context.Context, connection domain.KnowledgeConnection) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	queries := s.queries.WithTx(tx)
	if _, err := queries.CreateConnection(ctx, sqldb.CreateConnectionParams{
		ID:           connection.ID,
		UserID:       connection.UserID,
		Provider:     string(connection.Provider),
		Name:         connection.Name,
		SyncEnabled:  connection.SyncEnabled,
		LastSyncedAt: pgNullableTimestamptz(connection.LastSyncedAt),
		CreatedAt:    pgTimestamptz(connection.CreatedAt),
		UpdatedAt:    pgTimestamptz(connection.UpdatedAt),
	}); err != nil {
		return err
	}
	if err := upsertKnowledgeConnectionConfigs(ctx, queries, connection); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) UpdateKnowledgeConnection(ctx context.Context, connection domain.KnowledgeConnection) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	queries := s.queries.WithTx(tx)
	rowsAffected, err := queries.UpdateConnection(ctx, sqldb.UpdateConnectionParams{
		ID:           connection.ID,
		UserID:       connection.UserID,
		Provider:     string(connection.Provider),
		Name:         connection.Name,
		SyncEnabled:  connection.SyncEnabled,
		LastSyncedAt: pgNullableTimestamptz(connection.LastSyncedAt),
		CreatedAt:    pgTimestamptz(connection.CreatedAt),
		UpdatedAt:    pgTimestamptz(connection.UpdatedAt),
	})
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	if err := upsertKnowledgeConnectionConfigs(ctx, queries, connection); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) GetKnowledgeConnection(ctx context.Context, userID, connectionID string) (domain.KnowledgeConnection, error) {
	row, err := s.queries.GetConnection(ctx, sqldb.GetConnectionParams{ID: connectionID, UserID: userID})
	if err != nil {
		return domain.KnowledgeConnection{}, normalizeError(err)
	}
	connection := mapDBKnowledgeConnection(row)
	if err := loadKnowledgeConnectionConfigs(ctx, s.queries, &connection); err != nil {
		return domain.KnowledgeConnection{}, err
	}
	return connection, nil
}

func (s *PostgresStore) GetKnowledgeConnectionByID(ctx context.Context, connectionID string) (domain.KnowledgeConnection, error) {
	row, err := s.queries.GetConnectionById(ctx, connectionID)
	if err != nil {
		return domain.KnowledgeConnection{}, normalizeError(err)
	}
	connection := mapDBKnowledgeConnection(row)
	if err := loadKnowledgeConnectionConfigs(ctx, s.queries, &connection); err != nil {
		return domain.KnowledgeConnection{}, err
	}
	return connection, nil
}

func (s *PostgresStore) ListKnowledgeConnections(ctx context.Context, userID string) ([]domain.KnowledgeConnection, error) {
	rows, err := s.queries.ListConnections(ctx, userID)
	if err != nil {
		return nil, err
	}
	connections := make([]domain.KnowledgeConnection, 0, len(rows))
	for _, row := range rows {
		connection := mapDBKnowledgeConnection(row)
		if err := loadKnowledgeConnectionConfigs(ctx, s.queries, &connection); err != nil {
			return nil, err
		}
		connections = append(connections, connection)
	}
	return connections, nil
}

func (s *PostgresStore) DeleteKnowledgeConnection(ctx context.Context, userID, connectionID string) error {
	rowsAffected, err := s.queries.DeleteConnection(ctx, sqldb.DeleteConnectionParams{
		ID:     connectionID,
		UserID: userID,
	})
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func upsertKnowledgeConnectionConfigs(ctx context.Context, queries *sqldb.Queries, connection domain.KnowledgeConnection) error {
	switch connection.Provider {
	case domain.ProviderYuque:
		if connection.Yuque == nil {
			return errors.New("yuque config is required")
		}
		config := connection.Yuque
		if _, err := queries.UpsertYuqueConnectionConfig(ctx, sqldb.UpsertYuqueConnectionConfigParams{
			ConnectionID:    connection.ID,
			TokenEncrypted:  config.Token,
			GroupLogin:      config.GroupLogin,
			Namespace:       pgNullableText(config.Namespace),
			PendingDocsJson: marshalKnowledgeConnectionYuquePendingDocs(config.PendingDocs),
			CreatedAt:       pgTimestamptz(config.CreatedAt),
			UpdatedAt:       pgTimestamptz(config.UpdatedAt),
		}); err != nil {
			return err
		}
		if _, err := queries.DeleteFeishuConnectionConfig(ctx, connection.ID); err != nil {
			return err
		}
	case domain.ProviderFeishu:
		if connection.Feishu == nil {
			return errors.New("feishu config is required")
		}
		config := connection.Feishu
		if _, err := queries.UpsertFeishuConnectionConfig(ctx, sqldb.UpsertFeishuConnectionConfigParams{
			ConnectionID:       connection.ID,
			AppID:              config.AppID,
			AppSecretEncrypted: config.AppSecret,
			EntryType:          config.EntryType,
			EntryToken:         config.EntryToken,
			CreatedAt:          pgTimestamptz(config.CreatedAt),
			UpdatedAt:          pgTimestamptz(config.UpdatedAt),
		}); err != nil {
			return err
		}
		if _, err := queries.DeleteYuqueConnectionConfig(ctx, connection.ID); err != nil {
			return err
		}
	default:
		if _, err := queries.DeleteYuqueConnectionConfig(ctx, connection.ID); err != nil {
			return err
		}
		if _, err := queries.DeleteFeishuConnectionConfig(ctx, connection.ID); err != nil {
			return err
		}
	}
	return nil
}

func loadKnowledgeConnectionConfigs(ctx context.Context, queries *sqldb.Queries, connection *domain.KnowledgeConnection) error {
	connection.Yuque = nil
	connection.Feishu = nil

	switch connection.Provider {
	case domain.ProviderYuque:
		row, err := queries.GetYuqueConnectionConfig(ctx, connection.ID)
		if err != nil {
			return normalizeError(err)
		}
		connection.Yuque = &domain.KnowledgeConnectionYuqueConfig{
			Token:       row.TokenEncrypted,
			GroupLogin:  row.GroupLogin,
			Namespace:   pgTextValue(row.Namespace),
			PendingDocs: unmarshalKnowledgeConnectionYuquePendingDocs(row.PendingDocsJson),
			CreatedAt:   pgTimestamptzValue(row.CreatedAt),
			UpdatedAt:   pgTimestamptzValue(row.UpdatedAt),
		}
	case domain.ProviderFeishu:
		row, err := queries.GetFeishuConnectionConfig(ctx, connection.ID)
		if err != nil {
			return normalizeError(err)
		}
		connection.Feishu = &domain.KnowledgeConnectionFeishuConfig{
			AppID:      row.AppID,
			AppSecret:  row.AppSecretEncrypted,
			EntryType:  row.EntryType,
			EntryToken: row.EntryToken,
			CreatedAt:  pgTimestamptzValue(row.CreatedAt),
			UpdatedAt:  pgTimestamptzValue(row.UpdatedAt),
		}
	}

	return nil
}

func mapDBKnowledgeConnection(connection sqldb.KnowledgeConnection) domain.KnowledgeConnection {
	return domain.KnowledgeConnection{
		ID:           connection.ID,
		UserID:       connection.UserID,
		Provider:     domain.Provider(connection.Provider),
		Name:         connection.Name,
		SyncEnabled:  connection.SyncEnabled,
		LastSyncedAt: pgNullableTimestamptzPtr(connection.LastSyncedAt),
		CreatedAt:    pgTimestamptzValue(connection.CreatedAt),
		UpdatedAt:    pgTimestamptzValue(connection.UpdatedAt),
	}
}
