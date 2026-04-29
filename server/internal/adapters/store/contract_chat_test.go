package store

import (
	"context"
	"errors"
	"testing"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/persistence"
)

func testChatContract(t *testing.T, s contractStore) {
	t.Helper()

	ctx := context.Background()
	userID := "user-contract-chat"
	lastMessagePrimary := contractTime(71)
	sessionSecondaryLastMessage := contractTime(69)
	sessionPrimary := domain.ChatSession{
		ID:            "chat-session-primary",
		UserID:        userID,
		Title:         "Primary Session",
		Pinned:        true,
		LastMessageAt: &lastMessagePrimary,
		CreatedAt:     contractTime(70),
		UpdatedAt:     contractTime(72),
	}
	sessionSecondary := domain.ChatSession{
		ID:            "chat-session-secondary",
		UserID:        userID,
		Title:         "Secondary Session",
		Pinned:        false,
		LastMessageAt: &sessionSecondaryLastMessage,
		CreatedAt:     contractTime(68),
		UpdatedAt:     contractTime(69),
	}

	if err := s.CreateChatSession(ctx, sessionPrimary); err != nil {
		t.Fatalf("CreateChatSession primary failed: %v", err)
	}
	if err := s.CreateChatSession(ctx, sessionSecondary); err != nil {
		t.Fatalf("CreateChatSession secondary failed: %v", err)
	}

	gotSession, err := s.GetChatSession(ctx, userID, sessionPrimary.ID)
	if err != nil {
		t.Fatalf("GetChatSession failed: %v", err)
	}
	assertDeepEqual(t, "chat session by id", gotSession, sessionPrimary)

	sessionPrimary.Title = "Primary Session Updated"
	sessionPrimary.UpdatedAt = contractTime(73)
	if err := s.UpdateChatSession(ctx, sessionPrimary); err != nil {
		t.Fatalf("UpdateChatSession failed: %v", err)
	}
	gotUpdatedSession, err := s.GetChatSession(ctx, userID, sessionPrimary.ID)
	if err != nil {
		t.Fatalf("GetChatSession after update failed: %v", err)
	}
	assertDeepEqual(t, "updated chat session", gotUpdatedSession, sessionPrimary)

	listedSessions, err := s.ListChatSessions(ctx, userID)
	if err != nil {
		t.Fatalf("ListChatSessions failed: %v", err)
	}
	assertDeepEqual(t, "chat sessions list order", listedSessions, []domain.ChatSession{sessionPrimary, sessionSecondary})

	userMessage := domain.ChatMessage{
		ID:           "chat-message-user",
		SessionID:    sessionPrimary.ID,
		UserID:       userID,
		Role:         domain.ChatRoleUser,
		Content:      "hello",
		Skill:        "summarize",
		UseKnowledge: true,
		CreatedAt:    contractTime(74),
	}
	assistantCompletedAt := contractTime(76)
	assistantMessage := domain.ChatMessage{
		ID:           "chat-message-assistant",
		SessionID:    sessionPrimary.ID,
		UserID:       userID,
		Role:         domain.ChatRoleAssistant,
		Content:      "hi there",
		UseKnowledge: true,
		CreatedAt:    contractTime(75),
		CompletedAt:  &assistantCompletedAt,
	}
	if err := s.SaveChatMessage(ctx, assistantMessage); err != nil {
		t.Fatalf("SaveChatMessage assistant failed: %v", err)
	}
	if err := s.SaveChatMessage(ctx, userMessage); err != nil {
		t.Fatalf("SaveChatMessage user failed: %v", err)
	}

	listedMessages, err := s.ListChatMessages(ctx, userID, sessionPrimary.ID)
	if err != nil {
		t.Fatalf("ListChatMessages failed: %v", err)
	}
	assertDeepEqual(t, "chat messages list order", listedMessages, []domain.ChatMessage{userMessage, assistantMessage})

	sources := []domain.ChatMessageSource{
		{
			ID:           "source-a",
			MessageID:    assistantMessage.ID,
			Provider:     domain.ProviderYuque,
			ConnectionID: "conn-a",
			DocumentID:   "doc-a",
			ChunkID:      "chunk-a",
			Title:        "Source A",
			Repo:         "repo-a",
			URL:          "https://example.com/a",
			Snippet:      "snippet a",
			MatchedLines: []string{"line a1", "line a2"},
			Score:        0.8,
			CreatedAt:    contractTime(77),
		},
		{
			ID:           "source-b",
			MessageID:    assistantMessage.ID,
			Provider:     domain.ProviderWeb,
			ConnectionID: "",
			DocumentID:   "",
			ChunkID:      "",
			Title:        "Source B",
			Repo:         "",
			URL:          "https://example.com/b",
			Snippet:      "snippet b",
			MatchedLines: []string{"line b1"},
			Score:        0.6,
			CreatedAt:    contractTime(78),
		},
	}
	if err := s.SaveChatMessageSources(ctx, assistantMessage.ID, sources); err != nil {
		t.Fatalf("SaveChatMessageSources failed: %v", err)
	}
	gotSources, err := s.ListChatMessageSources(ctx, assistantMessage.ID)
	if err != nil {
		t.Fatalf("ListChatMessageSources failed: %v", err)
	}
	assertDeepEqual(t, "chat message sources", gotSources, sources)

	if err := s.DeleteChatSession(ctx, userID, sessionPrimary.ID); err != nil {
		t.Fatalf("DeleteChatSession failed: %v", err)
	}
	if _, err := s.GetChatSession(ctx, userID, sessionPrimary.ID); !errors.Is(err, persistence.ErrNotFound) {
		t.Fatalf("expected deleted chat session to return not found, got %v", err)
	}
	remainingSessions, err := s.ListChatSessions(ctx, userID)
	if err != nil {
		t.Fatalf("ListChatSessions after delete failed: %v", err)
	}
	assertDeepEqual(t, "remaining chat sessions after delete", remainingSessions, []domain.ChatSession{sessionSecondary})

	remainingSources, err := s.ListChatMessageSources(ctx, assistantMessage.ID)
	if err != nil {
		t.Fatalf("ListChatMessageSources after delete failed: %v", err)
	}
	assertDeepEqual(t, "chat message sources after delete", remainingSources, []domain.ChatMessageSource{})
}
