package store

import (
	"context"
	"errors"
	"testing"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/persistence"
)

func testChatModelContract(t *testing.T, s contractStore) {
	t.Helper()

	ctx := context.Background()
	userID := "user-contract-chat-model"

	if err := s.EnsureUserChatModelDefaults(ctx, userID); err != nil {
		t.Fatalf("EnsureUserChatModelDefaults failed: %v", err)
	}

	defaultModels, err := s.ListUserChatModels(ctx, userID)
	if err != nil {
		t.Fatalf("ListUserChatModels defaults failed: %v", err)
	}
	if len(defaultModels) != 2 {
		t.Fatalf("expected 2 default chat models, got %d", len(defaultModels))
	}
	assertDefaultUserChatModel(t, defaultModels, domain.ChatModelPurposeGeneral)
	assertDefaultUserChatModel(t, defaultModels, domain.ChatModelPurposeKnowledge)

	selectedGeneral, err := s.GetSelectedUserChatModel(ctx, userID, domain.ChatModelPurposeGeneral)
	if err != nil {
		t.Fatalf("GetSelectedUserChatModel general failed: %v", err)
	}
	assertChatModelMatchesDefault(t, selectedGeneral, userID, domain.ChatModelPurposeGeneral)

	selectedKnowledge, err := s.GetSelectedUserChatModel(ctx, userID, domain.ChatModelPurposeKnowledge)
	if err != nil {
		t.Fatalf("GetSelectedUserChatModel knowledge failed: %v", err)
	}
	assertChatModelMatchesDefault(t, selectedKnowledge, userID, domain.ChatModelPurposeKnowledge)

	customGeneral := domain.UserChatModel{
		ID:          "chat-model-custom-general",
		UserID:      userID,
		Purpose:     domain.ChatModelPurposeGeneral,
		Origin:      domain.ChatModelOriginCustom,
		Name:        "General Custom",
		BaseURL:     "https://example.com/general",
		APIKey:      "general-key",
		ModelName:   "general-custom-model",
		Temperature: 0.25,
		IsSelected:  false,
		CreatedAt:   contractTime(130),
		UpdatedAt:   contractTime(130),
	}
	customKnowledge := domain.UserChatModel{
		ID:          "chat-model-custom-knowledge",
		UserID:      userID,
		Purpose:     domain.ChatModelPurposeKnowledge,
		Origin:      domain.ChatModelOriginCustom,
		Name:        "Knowledge Custom",
		BaseURL:     "https://example.com/knowledge",
		APIKey:      "knowledge-key",
		ModelName:   "knowledge-custom-model",
		Temperature: 0.35,
		IsSelected:  false,
		CreatedAt:   contractTime(131),
		UpdatedAt:   contractTime(131),
	}
	if err := s.CreateUserChatModel(ctx, customGeneral); err != nil {
		t.Fatalf("CreateUserChatModel general failed: %v", err)
	}
	if err := s.CreateUserChatModel(ctx, customKnowledge); err != nil {
		t.Fatalf("CreateUserChatModel knowledge failed: %v", err)
	}

	gotCustomGeneral, err := s.GetUserChatModel(ctx, userID, customGeneral.ID)
	if err != nil {
		t.Fatalf("GetUserChatModel general failed: %v", err)
	}
	assertDeepEqual(t, "custom chat model general", gotCustomGeneral, customGeneral)

	customGeneral.Name = "General Custom Updated"
	customGeneral.BaseURL = "https://example.com/general-updated"
	customGeneral.APIKey = "general-key-updated"
	customGeneral.ModelName = "general-custom-model-updated"
	customGeneral.Temperature = 0.45
	customGeneral.UpdatedAt = contractTime(132)
	if err := s.UpdateUserChatModel(ctx, customGeneral); err != nil {
		t.Fatalf("UpdateUserChatModel general failed: %v", err)
	}
	gotUpdatedCustomGeneral, err := s.GetUserChatModel(ctx, userID, customGeneral.ID)
	if err != nil {
		t.Fatalf("GetUserChatModel general after update failed: %v", err)
	}
	assertDeepEqual(t, "updated custom chat model general", gotUpdatedCustomGeneral, customGeneral)

	if err := s.SelectUserChatModel(ctx, userID, customGeneral.ID); err != nil {
		t.Fatalf("SelectUserChatModel general failed: %v", err)
	}
	selectedGeneral, err = s.GetSelectedUserChatModel(ctx, userID, domain.ChatModelPurposeGeneral)
	if err != nil {
		t.Fatalf("GetSelectedUserChatModel general after select failed: %v", err)
	}
	if selectedGeneral.ID != customGeneral.ID {
		t.Fatalf("expected selected general chat model %q, got %q", customGeneral.ID, selectedGeneral.ID)
	}
	if !selectedGeneral.IsSelected {
		t.Fatalf("expected selected general chat model to remain marked selected")
	}

	listedModels, err := s.ListUserChatModels(ctx, userID)
	if err != nil {
		t.Fatalf("ListUserChatModels after create/select failed: %v", err)
	}
	assertSelectedChatModelCount(t, listedModels, domain.ChatModelPurposeGeneral, 1)
	assertSelectedChatModelCount(t, listedModels, domain.ChatModelPurposeKnowledge, 1)

	defaultKnowledgeModel := findDefaultChatModel(t, listedModels, domain.ChatModelPurposeKnowledge)
	if err := s.DeleteUserChatModel(ctx, userID, defaultKnowledgeModel.ID); !errors.Is(err, persistence.ErrConflict) {
		t.Fatalf("expected deleting default knowledge model to conflict, got %v", err)
	}

	if err := s.DeleteUserChatModel(ctx, userID, customGeneral.ID); err != nil {
		t.Fatalf("DeleteUserChatModel general failed: %v", err)
	}
	selectedGeneral, err = s.GetSelectedUserChatModel(ctx, userID, domain.ChatModelPurposeGeneral)
	if err != nil {
		t.Fatalf("GetSelectedUserChatModel general after delete failed: %v", err)
	}
	assertChatModelMatchesDefault(t, selectedGeneral, userID, domain.ChatModelPurposeGeneral)

	remainingModels, err := s.ListUserChatModels(ctx, userID)
	if err != nil {
		t.Fatalf("ListUserChatModels after delete failed: %v", err)
	}
	if len(remainingModels) != 3 {
		t.Fatalf("expected 3 chat models after deleting one custom model, got %d", len(remainingModels))
	}
	assertSelectedChatModelCount(t, remainingModels, domain.ChatModelPurposeGeneral, 1)
	assertSelectedChatModelCount(t, remainingModels, domain.ChatModelPurposeKnowledge, 1)
}

func assertDefaultUserChatModel(t *testing.T, models []domain.UserChatModel, purpose domain.ChatModelPurpose) {
	t.Helper()

	var matches []domain.UserChatModel
	for _, model := range models {
		if model.Purpose == purpose && model.Origin == domain.ChatModelOriginDefault {
			matches = append(matches, model)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("expected exactly one default chat model for purpose %q, got %d", purpose, len(matches))
	}
	assertChatModelMatchesDefault(t, matches[0], matches[0].UserID, purpose)
}

func findDefaultChatModel(t *testing.T, models []domain.UserChatModel, purpose domain.ChatModelPurpose) domain.UserChatModel {
	t.Helper()

	for _, model := range models {
		if model.Purpose == purpose && model.Origin == domain.ChatModelOriginDefault {
			return model
		}
	}
	t.Fatalf("default chat model for purpose %q not found", purpose)
	return domain.UserChatModel{}
}

func assertChatModelMatchesDefault(t *testing.T, model domain.UserChatModel, userID string, purpose domain.ChatModelPurpose) {
	t.Helper()

	name, runtime := domain.DefaultChatModelConfigForPurpose(purpose)
	if model.UserID != userID {
		t.Fatalf("default chat model user mismatch: got %q want %q", model.UserID, userID)
	}
	if model.Purpose != purpose {
		t.Fatalf("default chat model purpose mismatch: got %q want %q", model.Purpose, purpose)
	}
	if model.Origin != domain.ChatModelOriginDefault {
		t.Fatalf("default chat model origin mismatch: got %q want %q", model.Origin, domain.ChatModelOriginDefault)
	}
	if model.Name != name {
		t.Fatalf("default chat model name mismatch: got %q want %q", model.Name, name)
	}
	if model.BaseURL != runtime.BaseURL {
		t.Fatalf("default chat model base url mismatch: got %q want %q", model.BaseURL, runtime.BaseURL)
	}
	if model.APIKey != runtime.APIKey {
		t.Fatalf("default chat model api key mismatch: got %q want %q", model.APIKey, runtime.APIKey)
	}
	if model.ModelName != runtime.ModelName {
		t.Fatalf("default chat model model name mismatch: got %q want %q", model.ModelName, runtime.ModelName)
	}
	if model.Temperature != runtime.Temperature {
		t.Fatalf("default chat model temperature mismatch: got %v want %v", model.Temperature, runtime.Temperature)
	}
	if !model.IsSelected {
		t.Fatalf("expected default chat model for purpose %q to be selected", purpose)
	}
	if model.ID == "" {
		t.Fatalf("expected default chat model for purpose %q to have non-empty id", purpose)
	}
	if model.CreatedAt.IsZero() || model.UpdatedAt.IsZero() {
		t.Fatalf("expected default chat model for purpose %q to have timestamps set", purpose)
	}
}

func assertSelectedChatModelCount(t *testing.T, models []domain.UserChatModel, purpose domain.ChatModelPurpose, want int) {
	t.Helper()

	got := 0
	for _, model := range models {
		if model.Purpose == purpose && model.IsSelected {
			got++
		}
	}
	if got != want {
		t.Fatalf("selected chat model count for purpose %q mismatch: got %d want %d", purpose, got, want)
	}
}
