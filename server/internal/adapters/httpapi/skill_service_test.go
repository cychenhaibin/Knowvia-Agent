package httpapi

import (
	"context"
	"testing"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/store"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/mirror"
	skillsvc "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skill"
)

func TestSkillServiceSyncsMirror(t *testing.T) {
	mem := store.NewMemoryStore()
	mirrorClient := &errorMirrorClient{}
	service := newSkillServiceForTest(mem, mirror.NewService(mem, mirrorClient))

	_, err := service.CreateSkill(context.Background(), "user-1", skillsvc.CreateInput{
		Title:  "Summary Skill",
		Prompt: "summarize",
	})
	if err != nil {
		t.Fatalf("create skill: %v", err)
	}
	if len(mirrorClient.upsertedSkills) != 1 {
		t.Fatalf("expected service to mirror exactly once, got %d", len(mirrorClient.upsertedSkills))
	}
}

func TestResolveChatSkillSelectionByDefinitionUsesDefaultInstallation(t *testing.T) {
	mem := store.NewMemoryStore()
	user := domain.User{
		ID:        "user-1",
		Username:  "tester",
		CreatedAt: time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	now := time.Now().UTC()
	skill := domain.Skill{
		ID:           "install-1",
		UserID:       user.ID,
		DefinitionID: "def-1",
		RevisionID:   "rev-1",
		Version:      1,
		Slug:         "summary-skill",
		Kind:         domain.SkillKindChatProfile,
		Title:        "Summary Skill",
		Description:  "summarize things",
		Prompt:       "summarize",
		Mode:         "summary",
		Source:       domain.SkillSourceManual,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := mem.CreateSkill(context.Background(), skill); err != nil {
		t.Fatalf("create skill: %v", err)
	}

	secondInstallation := domain.SkillInstallation{
		ID:                "install-2",
		UserID:            user.ID,
		DefinitionID:      "def-1",
		CurrentRevisionID: "rev-1",
		Name:              "staging",
		IsDefault:         false,
		Enabled:           true,
		CreatedAt:         now.Add(time.Minute),
		UpdatedAt:         now.Add(time.Minute),
	}
	if err := mem.CreateSkillInstallation(context.Background(), secondInstallation); err != nil {
		t.Fatalf("create second installation: %v", err)
	}

	resolvedID, resolvedSkill, err := newSkillServiceForTest(mem, nil).ResolveChatSelection(context.Background(), user.ID, "", "def-1")
	if err != nil {
		t.Fatalf("resolve selection: %v", err)
	}
	if resolvedID != "install-1" {
		t.Fatalf("expected default installation install-1, got %s", resolvedID)
	}
	if resolvedSkill.ID != "install-1" {
		t.Fatalf("expected resolved skill install-1, got %s", resolvedSkill.ID)
	}
	if resolvedSkill.DefinitionID != "def-1" {
		t.Fatalf("expected definition def-1, got %s", resolvedSkill.DefinitionID)
	}
}
