package chat

import (
	"context"
	"testing"

	memorybackend "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/store/memory"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func TestResolveModelReturnsSelectedCustomModel(t *testing.T) {
	t.Parallel()

	store := memorybackend.New()
	service := NewService(ServiceDeps{
		Models:        store,
		Sessions:      store,
		Conversations: store,
		Prepare:       store,
	}, nil, nil, nil)
	ctx := context.Background()
	userID := "chat-model-user"

	created, err := service.CreateModel(ctx, CreateModelInput{
		UserID:      userID,
		Purpose:     domain.ChatModelPurposeGeneral,
		Name:        "Custom General",
		BaseURL:     "http://example.com/v1",
		APIKey:      "secret",
		ModelName:   "gpt-custom",
		Temperature: 0.4,
	})
	if err != nil {
		t.Fatalf("CreateModel failed: %v", err)
	}

	selected, err := service.SelectModel(ctx, userID, created.ID)
	if err != nil {
		t.Fatalf("SelectModel failed: %v", err)
	}

	resolved, err := service.ResolveModel(ctx, userID, false, "")
	if err != nil {
		t.Fatalf("ResolveModel failed: %v", err)
	}
	if resolved.ID != selected.ID {
		t.Fatalf("ResolveModel returned %q, want selected %q", resolved.ID, selected.ID)
	}
	if !resolved.IsSelected {
		t.Fatalf("ResolveModel should return selected model")
	}
}

func TestResolveModelFallsBackToDefaultConfig(t *testing.T) {
	t.Parallel()

	store := memorybackend.New()
	service := NewService(ServiceDeps{
		Models:        store,
		Sessions:      store,
		Conversations: store,
		Prepare:       store,
	}, nil, nil, nil)

	resolved, err := service.ResolveModel(context.Background(), "default-only-user", true, "")
	if err != nil {
		t.Fatalf("ResolveModel failed: %v", err)
	}

	expectedName, expectedRuntime := domain.DefaultChatModelConfigForPurpose(domain.ChatModelPurposeKnowledge)
	if resolved.Origin != domain.ChatModelOriginDefault {
		t.Fatalf("ResolveModel origin = %q, want default", resolved.Origin)
	}
	if resolved.Name != expectedName || resolved.ModelName != expectedRuntime.ModelName {
		t.Fatalf("ResolveModel returned unexpected default model: %#v", resolved)
	}
}

func TestDeleteModelRejectsDefaultModel(t *testing.T) {
	t.Parallel()

	store := memorybackend.New()
	service := NewService(ServiceDeps{
		Models:        store,
		Sessions:      store,
		Conversations: store,
		Prepare:       store,
	}, nil, nil, nil)
	ctx := context.Background()
	userID := "default-delete-user"

	models, err := service.ListModels(ctx, userID)
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	var defaultModel domain.UserChatModel
	for _, model := range models {
		if model.Origin == domain.ChatModelOriginDefault {
			defaultModel = model
			break
		}
	}
	if defaultModel.ID == "" {
		t.Fatalf("expected at least one default model")
	}

	if err := service.DeleteModel(ctx, userID, defaultModel.ID); err != ErrDefaultChatModelImmutable {
		t.Fatalf("DeleteModel error = %v, want %v", err, ErrDefaultChatModelImmutable)
	}
}
