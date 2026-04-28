package skillresolver

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/store"
)

func TestResolveInstallationRecordFallsBackWhenDefaultIsDisabled(t *testing.T) {
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

	defaultRecord, err := mem.GetSkillInstallationRecord(context.Background(), user.ID, "install-1")
	if err != nil {
		t.Fatalf("get default installation: %v", err)
	}
	defaultRecord.Installation.Enabled = false
	defaultRecord.Installation.UpdatedAt = now.Add(time.Minute)
	if err := mem.UpdateSkillInstallation(context.Background(), defaultRecord.Installation); err != nil {
		t.Fatalf("disable default installation: %v", err)
	}

	secondInstallation := domain.SkillInstallation{
		ID:                "install-2",
		UserID:            user.ID,
		DefinitionID:      "def-1",
		CurrentRevisionID: "rev-1",
		Name:              "staging",
		IsDefault:         false,
		Enabled:           true,
		CreatedAt:         now.Add(2 * time.Minute),
		UpdatedAt:         now.Add(2 * time.Minute),
	}
	if err := mem.CreateSkillInstallation(context.Background(), secondInstallation); err != nil {
		t.Fatalf("create second installation: %v", err)
	}

	record, err := ResolveInstallationRecord(context.Background(), mem, user.ID, "", "def-1")
	if err != nil {
		t.Fatalf("resolve installation: %v", err)
	}
	if record.Installation.ID != "install-2" {
		t.Fatalf("expected enabled fallback install-2, got %s", record.Installation.ID)
	}
}

func TestResolveInstallationRecordRejectsDisabledExplicitInstallation(t *testing.T) {
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

	record, err := mem.GetSkillInstallationRecord(context.Background(), user.ID, "install-1")
	if err != nil {
		t.Fatalf("get installation: %v", err)
	}
	record.Installation.Enabled = false
	record.Installation.UpdatedAt = now.Add(time.Minute)
	if err := mem.UpdateSkillInstallation(context.Background(), record.Installation); err != nil {
		t.Fatalf("disable installation: %v", err)
	}

	_, err = ResolveInstallationRecord(context.Background(), mem, user.ID, "install-1", "")
	if !errors.Is(err, ErrDisabled) {
		t.Fatalf("expected ErrDisabled, got %v", err)
	}
}
