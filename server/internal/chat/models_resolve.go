package chat

import (
	"context"
	"errors"
	"strings"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *Service) EnsureModelDefaults(ctx context.Context, userID string) error {
	if s.modelStore == nil {
		return errors.New("chat model store unavailable")
	}
	return s.modelStore.EnsureUserChatModelDefaults(ctx, userID)
}

func (s *Service) ListModels(ctx context.Context, userID string) ([]domain.UserChatModel, error) {
	if err := s.EnsureModelDefaults(ctx, userID); err != nil {
		return nil, err
	}
	return s.modelStore.ListUserChatModels(ctx, userID)
}

func (s *Service) ListAvailableModels(ctx context.Context, userID string) ([]domain.UserChatModel, error) {
	models, err := s.ListModels(ctx, userID)
	if err != nil {
		return nil, err
	}
	return annotateModelAvailability(ctx, models), nil
}

func (s *Service) ResolveModel(
	ctx context.Context,
	userID string,
	useKnowledge bool,
	requestedModel string,
) (domain.UserChatModel, error) {
	if err := s.EnsureModelDefaults(ctx, userID); err != nil {
		return domain.UserChatModel{}, err
	}

	purpose := domain.ChatModelPurposeGeneral
	if useKnowledge {
		purpose = domain.ChatModelPurposeKnowledge
	}

	models, err := s.modelStore.ListUserChatModels(ctx, userID)
	if err != nil {
		return domain.UserChatModel{}, err
	}
	var selected *domain.UserChatModel
	for _, model := range models {
		if model.Purpose != purpose {
			continue
		}
		if requestedModel != "" &&
			(strings.EqualFold(model.ID, requestedModel) || strings.EqualFold(model.Name, requestedModel)) {
			return model, nil
		}
		if model.IsSelected && selected == nil {
			candidate := model
			selected = &candidate
		}
	}
	if selected != nil {
		return *selected, nil
	}
	name, runtime := domain.DefaultChatModelConfigForPurpose(purpose)
	return domain.UserChatModel{
		ID:          "",
		UserID:      userID,
		Purpose:     purpose,
		Origin:      domain.ChatModelOriginDefault,
		Name:        name,
		BaseURL:     runtime.BaseURL,
		APIKey:      runtime.APIKey,
		ModelName:   runtime.ModelName,
		Temperature: runtime.Temperature,
		IsSelected:  true,
		Available:   true,
	}, nil
}
