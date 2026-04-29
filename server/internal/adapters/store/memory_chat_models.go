package store

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *MemoryStore) EnsureUserChatModelDefaults(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	s.ensureDefaultChatModelsLocked(userID, now)
	return nil
}

func (s *MemoryStore) ListUserChatModels(_ context.Context, userID string) ([]domain.UserChatModel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDefaultChatModelsLocked(userID, time.Now().UTC())
	items := make([]domain.UserChatModel, 0)
	for _, model := range s.chatModels {
		if model.UserID == userID {
			items = append(items, model)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Purpose != items[j].Purpose {
			return items[i].Purpose < items[j].Purpose
		}
		if items[i].IsSelected != items[j].IsSelected {
			return items[i].IsSelected
		}
		if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		}
		return items[i].Name < items[j].Name
	})
	return append([]domain.UserChatModel(nil), items...), nil
}

func (s *MemoryStore) GetUserChatModel(_ context.Context, userID, modelID string) (domain.UserChatModel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	model, ok := s.chatModels[modelID]
	if !ok || model.UserID != userID {
		return domain.UserChatModel{}, ErrNotFound
	}
	return model, nil
}

func (s *MemoryStore) GetSelectedUserChatModel(_ context.Context, userID string, purpose domain.ChatModelPurpose) (domain.UserChatModel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDefaultChatModelsLocked(userID, time.Now().UTC())
	for _, model := range s.chatModels {
		if model.UserID == userID && model.Purpose == purpose && model.IsSelected {
			return model, nil
		}
	}
	return domain.UserChatModel{}, ErrNotFound
}

func (s *MemoryStore) CreateUserChatModel(_ context.Context, model domain.UserChatModel) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDefaultChatModelsLocked(model.UserID, time.Now().UTC())
	for _, existing := range s.chatModels {
		if existing.UserID == model.UserID &&
			existing.Purpose == model.Purpose &&
			strings.EqualFold(existing.Name, model.Name) {
			return ErrConflict
		}
	}
	if !s.hasSelectedChatModelLocked(model.UserID, model.Purpose) {
		model.IsSelected = true
	}
	s.chatModels[model.ID] = model
	return nil
}

func (s *MemoryStore) UpdateUserChatModel(_ context.Context, model domain.UserChatModel) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.chatModels[model.ID]
	if !ok || existing.UserID != model.UserID {
		return ErrNotFound
	}
	for _, item := range s.chatModels {
		if item.ID == model.ID {
			continue
		}
		if item.UserID == model.UserID &&
			item.Purpose == model.Purpose &&
			strings.EqualFold(item.Name, model.Name) {
			return ErrConflict
		}
	}
	s.chatModels[model.ID] = model
	return nil
}

func (s *MemoryStore) SelectUserChatModel(_ context.Context, userID, modelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	model, ok := s.chatModels[modelID]
	if !ok || model.UserID != userID {
		return ErrNotFound
	}
	now := time.Now().UTC()
	for id, existing := range s.chatModels {
		if existing.UserID != userID || existing.Purpose != model.Purpose {
			continue
		}
		existing.IsSelected = id == modelID
		existing.UpdatedAt = now
		s.chatModels[id] = existing
	}
	return nil
}

func (s *MemoryStore) DeleteUserChatModel(_ context.Context, userID, modelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	model, ok := s.chatModels[modelID]
	if !ok || model.UserID != userID {
		return ErrNotFound
	}
	if model.Origin == domain.ChatModelOriginDefault {
		return ErrConflict
	}
	remaining := make([]domain.UserChatModel, 0)
	for id, existing := range s.chatModels {
		if id == modelID {
			continue
		}
		if existing.UserID == userID && existing.Purpose == model.Purpose {
			remaining = append(remaining, existing)
		}
	}
	if len(remaining) == 0 {
		return ErrConflict
	}
	delete(s.chatModels, modelID)
	if model.IsSelected {
		sort.Slice(remaining, func(i, j int) bool {
			if !remaining[i].CreatedAt.Equal(remaining[j].CreatedAt) {
				return remaining[i].CreatedAt.Before(remaining[j].CreatedAt)
			}
			return remaining[i].Name < remaining[j].Name
		})
		fallback := remaining[0]
		fallback.IsSelected = true
		fallback.UpdatedAt = time.Now().UTC()
		s.chatModels[fallback.ID] = fallback
	}
	return nil
}

func (s *MemoryStore) ensureDefaultChatModelsLocked(userID string, now time.Time) {
	if !s.hasAnyChatModelForPurposeLocked(userID, domain.ChatModelPurposeGeneral) {
		name, runtime := domain.DefaultChatModelConfigForPurpose(domain.ChatModelPurposeGeneral)
		model := domain.UserChatModel{
			ID:          userID + ":general:default",
			UserID:      userID,
			Purpose:     domain.ChatModelPurposeGeneral,
			Origin:      domain.ChatModelOriginDefault,
			Name:        name,
			BaseURL:     runtime.BaseURL,
			APIKey:      runtime.APIKey,
			ModelName:   runtime.ModelName,
			Temperature: runtime.Temperature,
			IsSelected:  true,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		s.chatModels[model.ID] = model
	}
	if !s.hasAnyChatModelForPurposeLocked(userID, domain.ChatModelPurposeKnowledge) {
		name, runtime := domain.DefaultChatModelConfigForPurpose(domain.ChatModelPurposeKnowledge)
		model := domain.UserChatModel{
			ID:          userID + ":knowledge:default",
			UserID:      userID,
			Purpose:     domain.ChatModelPurposeKnowledge,
			Origin:      domain.ChatModelOriginDefault,
			Name:        name,
			BaseURL:     runtime.BaseURL,
			APIKey:      runtime.APIKey,
			ModelName:   runtime.ModelName,
			Temperature: runtime.Temperature,
			IsSelected:  true,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		s.chatModels[model.ID] = model
	}
	if !s.hasSelectedChatModelLocked(userID, domain.ChatModelPurposeGeneral) {
		s.selectFirstChatModelLocked(userID, domain.ChatModelPurposeGeneral, now)
	}
	if !s.hasSelectedChatModelLocked(userID, domain.ChatModelPurposeKnowledge) {
		s.selectFirstChatModelLocked(userID, domain.ChatModelPurposeKnowledge, now)
	}
}

func (s *MemoryStore) hasAnyChatModelForPurposeLocked(userID string, purpose domain.ChatModelPurpose) bool {
	for _, model := range s.chatModels {
		if model.UserID == userID && model.Purpose == purpose {
			return true
		}
	}
	return false
}

func (s *MemoryStore) hasSelectedChatModelLocked(userID string, purpose domain.ChatModelPurpose) bool {
	for _, model := range s.chatModels {
		if model.UserID == userID && model.Purpose == purpose && model.IsSelected {
			return true
		}
	}
	return false
}

func (s *MemoryStore) selectFirstChatModelLocked(userID string, purpose domain.ChatModelPurpose, now time.Time) {
	candidates := make([]domain.UserChatModel, 0)
	for _, model := range s.chatModels {
		if model.UserID == userID && model.Purpose == purpose {
			candidates = append(candidates, model)
		}
	}
	if len(candidates) == 0 {
		return
	}
	sort.Slice(candidates, func(i, j int) bool {
		if !candidates[i].CreatedAt.Equal(candidates[j].CreatedAt) {
			return candidates[i].CreatedAt.Before(candidates[j].CreatedAt)
		}
		return candidates[i].Name < candidates[j].Name
	})
	targetID := candidates[0].ID
	for id, model := range s.chatModels {
		if model.UserID != userID || model.Purpose != purpose {
			continue
		}
		model.IsSelected = id == targetID
		model.UpdatedAt = now
		s.chatModels[id] = model
	}
}
