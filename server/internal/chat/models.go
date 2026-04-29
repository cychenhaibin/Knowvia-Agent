package chat

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/persistence"
)

var ErrChatModelNotFound = errors.New("chat: model not found")
var ErrDefaultChatModelImmutable = errors.New("chat: default chat models cannot be modified")
var ErrChatModelConflict = errors.New("chat: model configuration conflict")

type CreateModelInput struct {
	UserID      string
	Purpose     domain.ChatModelPurpose
	Name        string
	BaseURL     string
	APIKey      string
	ModelName   string
	Temperature float64
}

type UpdateModelInput struct {
	UserID      string
	ModelID     string
	Name        string
	BaseURL     string
	APIKey      string
	ModelName   string
	Temperature *float64
}

func (s *Service) CreateModel(ctx context.Context, input CreateModelInput) (domain.UserChatModel, error) {
	if err := s.EnsureModelDefaults(ctx, input.UserID); err != nil {
		return domain.UserChatModel{}, err
	}

	now := time.Now().UTC()
	model := domain.UserChatModel{
		ID:          uuid.NewString(),
		UserID:      input.UserID,
		Purpose:     input.Purpose,
		Origin:      domain.ChatModelOriginCustom,
		Name:        input.Name,
		BaseURL:     input.BaseURL,
		APIKey:      input.APIKey,
		ModelName:   input.ModelName,
		Temperature: input.Temperature,
		IsSelected:  false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.modelStore.CreateUserChatModel(ctx, model); err != nil {
		if errors.Is(err, persistence.ErrConflict) {
			return domain.UserChatModel{}, ErrChatModelConflict
		}
		return domain.UserChatModel{}, err
	}

	stored, err := s.modelStore.GetUserChatModel(ctx, input.UserID, model.ID)
	if err != nil {
		return domain.UserChatModel{}, err
	}
	return stored, nil
}

func (s *Service) UpdateModel(ctx context.Context, input UpdateModelInput) (domain.UserChatModel, error) {
	if err := s.EnsureModelDefaults(ctx, input.UserID); err != nil {
		return domain.UserChatModel{}, err
	}

	model, err := s.modelStore.GetUserChatModel(ctx, input.UserID, input.ModelID)
	if err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return domain.UserChatModel{}, ErrChatModelNotFound
		}
		return domain.UserChatModel{}, err
	}
	if model.Origin == domain.ChatModelOriginDefault {
		return domain.UserChatModel{}, ErrDefaultChatModelImmutable
	}

	model.Name = input.Name
	model.BaseURL = input.BaseURL
	model.APIKey = input.APIKey
	model.ModelName = input.ModelName
	temperature, err := normalizeTemperature(input.Temperature, model.Temperature)
	if err != nil {
		return domain.UserChatModel{}, err
	}
	model.Temperature = temperature
	model.UpdatedAt = time.Now().UTC()

	if err := s.modelStore.UpdateUserChatModel(ctx, model); err != nil {
		if errors.Is(err, persistence.ErrConflict) {
			return domain.UserChatModel{}, ErrChatModelConflict
		}
		return domain.UserChatModel{}, err
	}

	stored, err := s.modelStore.GetUserChatModel(ctx, input.UserID, input.ModelID)
	if err != nil {
		return domain.UserChatModel{}, err
	}
	return stored, nil
}

func normalizeTemperature(raw *float64, fallback float64) (float64, error) {
	if raw == nil {
		return fallback, nil
	}
	value := *raw
	if value < 0 || value > 2 {
		return 0, errors.New("temperature must be between 0 and 2")
	}
	return value, nil
}
