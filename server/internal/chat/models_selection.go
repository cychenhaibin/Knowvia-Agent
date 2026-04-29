package chat

import (
	"context"
	"errors"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/persistence"
)

func (s *Service) SelectModel(ctx context.Context, userID, modelID string) (domain.UserChatModel, error) {
	if err := s.EnsureModelDefaults(ctx, userID); err != nil {
		return domain.UserChatModel{}, err
	}

	model, err := s.modelStore.GetUserChatModel(ctx, userID, modelID)
	if err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return domain.UserChatModel{}, ErrChatModelNotFound
		}
		return domain.UserChatModel{}, err
	}
	if err := s.modelStore.SelectUserChatModel(ctx, userID, model.ID); err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return domain.UserChatModel{}, ErrChatModelNotFound
		}
		return domain.UserChatModel{}, err
	}

	selected, err := s.modelStore.GetSelectedUserChatModel(ctx, userID, model.Purpose)
	if err != nil {
		return domain.UserChatModel{}, err
	}
	return selected, nil
}

func (s *Service) DeleteModel(ctx context.Context, userID, modelID string) error {
	if err := s.EnsureModelDefaults(ctx, userID); err != nil {
		return err
	}

	model, err := s.modelStore.GetUserChatModel(ctx, userID, modelID)
	if err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return ErrChatModelNotFound
		}
		return err
	}
	if model.Origin == domain.ChatModelOriginDefault {
		return ErrDefaultChatModelImmutable
	}
	if err := s.modelStore.DeleteUserChatModel(ctx, userID, modelID); err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return ErrChatModelNotFound
		}
		return err
	}
	return nil
}
