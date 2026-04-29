package store

import (
	"context"
	"time"

	sqldb "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/db"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *PostgresStore) ListUserChatModels(ctx context.Context, userID string) ([]domain.UserChatModel, error) {
	rows, err := s.queries.ListUserChatModels(ctx, userID)
	if err != nil {
		return nil, err
	}

	items := make([]domain.UserChatModel, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapDBUserChatModel(
			row.ID, row.UserID, row.Purpose, row.Origin, row.Name, row.BaseUrl,
			row.ApiKeyEncrypted, row.ModelName, row.Temperature, row.IsSelected,
			row.CreatedAt, row.UpdatedAt,
		))
	}
	return items, nil
}

func (s *PostgresStore) GetUserChatModel(ctx context.Context, userID, modelID string) (domain.UserChatModel, error) {
	row, err := s.queries.GetUserChatModel(ctx, sqldb.GetUserChatModelParams{
		ID:     modelID,
		UserID: userID,
	})
	if err != nil {
		return domain.UserChatModel{}, normalizeError(err)
	}
	return mapDBUserChatModel(
		row.ID, row.UserID, row.Purpose, row.Origin, row.Name, row.BaseUrl,
		row.ApiKeyEncrypted, row.ModelName, row.Temperature, row.IsSelected,
		row.CreatedAt, row.UpdatedAt,
	), nil
}

func (s *PostgresStore) GetSelectedUserChatModel(ctx context.Context, userID string, purpose domain.ChatModelPurpose) (domain.UserChatModel, error) {
	row, err := s.queries.GetSelectedUserChatModel(ctx, sqldb.GetSelectedUserChatModelParams{
		UserID:  userID,
		Purpose: string(purpose),
	})
	if err != nil {
		return domain.UserChatModel{}, normalizeError(err)
	}
	return mapDBUserChatModel(
		row.ID, row.UserID, row.Purpose, row.Origin, row.Name, row.BaseUrl,
		row.ApiKeyEncrypted, row.ModelName, row.Temperature, row.IsSelected,
		row.CreatedAt, row.UpdatedAt,
	), nil
}

func (s *PostgresStore) CreateUserChatModel(ctx context.Context, model domain.UserChatModel) error {
	_, err := s.queries.CreateUserChatModel(ctx, sqldb.CreateUserChatModelParams{
		ID:              model.ID,
		UserID:          model.UserID,
		Purpose:         string(model.Purpose),
		Origin:          string(model.Origin),
		Name:            model.Name,
		BaseUrl:         model.BaseURL,
		ApiKeyEncrypted: model.APIKey,
		ModelName:       model.ModelName,
		Temperature:     model.Temperature,
		IsSelected:      model.IsSelected,
		CreatedAt:       pgTimestamptz(model.CreatedAt),
		UpdatedAt:       pgTimestamptz(model.UpdatedAt),
	})
	return normalizeError(err)
}

func (s *PostgresStore) UpdateUserChatModel(ctx context.Context, model domain.UserChatModel) error {
	rowsAffected, err := s.queries.UpdateUserChatModel(ctx, sqldb.UpdateUserChatModelParams{
		ID:              model.ID,
		UserID:          model.UserID,
		Name:            model.Name,
		BaseUrl:         model.BaseURL,
		ApiKeyEncrypted: model.APIKey,
		ModelName:       model.ModelName,
		Temperature:     model.Temperature,
		UpdatedAt:       pgTimestamptz(model.UpdatedAt),
	})
	if err != nil {
		return normalizeError(err)
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) SelectUserChatModel(ctx context.Context, userID, modelID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	model, err := getUserChatModelTx(ctx, tx, userID, modelID)
	if err != nil {
		return err
	}
	if err := setSelectedUserChatModelTx(ctx, tx, userID, model.Purpose, modelID, time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) DeleteUserChatModel(ctx context.Context, userID, modelID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	model, err := getUserChatModelTx(ctx, tx, userID, modelID)
	if err != nil {
		return err
	}
	if model.Origin == domain.ChatModelOriginDefault {
		return ErrConflict
	}
	queries := s.queries.WithTx(tx)
	rows, err := queries.ListAlternativeUserChatModels(ctx, sqldb.ListAlternativeUserChatModelsParams{
		UserID:  userID,
		Purpose: string(model.Purpose),
		ID:      modelID,
	})
	if err != nil {
		return err
	}
	remaining := make([]domain.UserChatModel, 0, len(rows))
	for _, row := range rows {
		remaining = append(remaining, mapDBUserChatModel(
			row.ID, row.UserID, row.Purpose, row.Origin, row.Name, row.BaseUrl,
			row.ApiKeyEncrypted, row.ModelName, row.Temperature, row.IsSelected,
			row.CreatedAt, row.UpdatedAt,
		))
	}
	if len(remaining) == 0 {
		return ErrConflict
	}

	if model.IsSelected {
		fallbackID := remaining[0].ID
		if err := setSelectedUserChatModelTx(
			ctx,
			tx,
			userID,
			model.Purpose,
			fallbackID,
			time.Now().UTC(),
		); err != nil {
			return err
		}
	}

	rowsAffected, err := queries.DeleteUserChatModel(ctx, sqldb.DeleteUserChatModelParams{
		ID:     modelID,
		UserID: userID,
	})
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}

func setSelectedUserChatModelTx(
	ctx context.Context,
	tx pgx.Tx,
	userID string,
	purpose domain.ChatModelPurpose,
	modelID string,
	now time.Time,
) error {
	queries := sQ(tx)
	if _, err := queries.UnsetSelectedUserChatModel(ctx, sqldb.UnsetSelectedUserChatModelParams{
		UserID:    userID,
		Purpose:   string(purpose),
		UpdatedAt: pgTimestamptz(now),
	}); err != nil {
		return err
	}

	rowsAffected, err := queries.SetSelectedUserChatModel(ctx, sqldb.SetSelectedUserChatModelParams{
		UserID:    userID,
		Purpose:   string(purpose),
		ID:        modelID,
		UpdatedAt: pgTimestamptz(now),
	})
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func getUserChatModelTx(ctx context.Context, tx pgx.Tx, userID, modelID string) (domain.UserChatModel, error) {
	row, err := sQ(tx).GetUserChatModel(ctx, sqldb.GetUserChatModelParams{
		ID:     modelID,
		UserID: userID,
	})
	if err != nil {
		return domain.UserChatModel{}, normalizeError(err)
	}
	return mapDBUserChatModel(
		row.ID, row.UserID, row.Purpose, row.Origin, row.Name, row.BaseUrl,
		row.ApiKeyEncrypted, row.ModelName, row.Temperature, row.IsSelected,
		row.CreatedAt, row.UpdatedAt,
	), nil
}

func mapDBUserChatModel(id, userID, purpose, origin, name, baseURL, apiKey, modelName string, temperature float64, isSelected bool, createdAt, updatedAt pgtype.Timestamptz) domain.UserChatModel {
	return domain.UserChatModel{
		ID:          id,
		UserID:      userID,
		Purpose:     domain.ChatModelPurpose(purpose),
		Origin:      domain.ChatModelOrigin(origin),
		Name:        name,
		BaseURL:     baseURL,
		APIKey:      apiKey,
		ModelName:   modelName,
		Temperature: temperature,
		IsSelected:  isSelected,
		CreatedAt:   pgTimestamptzValue(createdAt),
		UpdatedAt:   pgTimestamptzValue(updatedAt),
	}
}
