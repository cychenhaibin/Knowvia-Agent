package store

import (
	"context"
	"time"

	sqldb "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/db"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *PostgresStore) EnsureUserChatModelDefaults(ctx context.Context, userID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := ensureUserChatModelDefaultsTx(ctx, tx, userID, time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func ensureUserChatModelDefaultsTx(ctx context.Context, tx pgx.Tx, userID string, now time.Time) error {
	queries := sQ(tx)
	defaults := []struct {
		purpose domain.ChatModelPurpose
		name    string
		runtime domain.ChatRuntimeConfig
	}{
		func() struct {
			purpose domain.ChatModelPurpose
			name    string
			runtime domain.ChatRuntimeConfig
		} {
			name, runtime := domain.DefaultChatModelConfigForPurpose(domain.ChatModelPurposeGeneral)
			return struct {
				purpose domain.ChatModelPurpose
				name    string
				runtime domain.ChatRuntimeConfig
			}{purpose: domain.ChatModelPurposeGeneral, name: name, runtime: runtime}
		}(),
		func() struct {
			purpose domain.ChatModelPurpose
			name    string
			runtime domain.ChatRuntimeConfig
		} {
			name, runtime := domain.DefaultChatModelConfigForPurpose(domain.ChatModelPurposeKnowledge)
			return struct {
				purpose domain.ChatModelPurpose
				name    string
				runtime domain.ChatRuntimeConfig
			}{purpose: domain.ChatModelPurposeKnowledge, name: name, runtime: runtime}
		}(),
	}

	for _, item := range defaults {
		if _, err := queries.EnsureUserChatModelDefault(ctx, sqldb.EnsureUserChatModelDefaultParams{
			ID:              uuid.NewString(),
			UserID:          userID,
			Purpose:         string(item.purpose),
			Name:            item.name,
			BaseUrl:         item.runtime.BaseURL,
			ApiKeyEncrypted: item.runtime.APIKey,
			ModelName:       item.runtime.ModelName,
			Temperature:     item.runtime.Temperature,
			CreatedAt:       pgTimestamptz(now),
		}); err != nil {
			return err
		}

		selectedCount, err := queries.CountSelectedUserChatModels(ctx, sqldb.CountSelectedUserChatModelsParams{
			UserID:  userID,
			Purpose: string(item.purpose),
		})
		if err != nil {
			return err
		}
		if selectedCount == 1 {
			continue
		}

		fallbackID, err := queries.SelectFallbackUserChatModel(ctx, sqldb.SelectFallbackUserChatModelParams{
			UserID:  userID,
			Purpose: string(item.purpose),
		})
		if err != nil {
			return normalizeError(err)
		}
		if err := setSelectedUserChatModelTx(ctx, tx, userID, item.purpose, fallbackID, now); err != nil {
			return err
		}
	}
	return nil
}
