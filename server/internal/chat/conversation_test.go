package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	memorybackend "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/store/memory"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func TestStreamConversationDelegatesTaskSessionToTaskRunner(t *testing.T) {
	store := memorybackend.New()
	service := NewService(ServiceDeps{
		Models:        store,
		Sessions:      store,
		Conversations: store,
		Prepare:       store,
	}, nil, nil, nil)
	runner := &fakeTaskConversationRunner{
		result: TaskConversationResult{
			Handled: true,
			Answer:  "# Code Wiki\n\nGenerated.",
		},
	}
	service.SetTaskConversationRunner(runner)

	taskSession := domain.ChatSession{
		ID:        "task-session-1",
		UserID:    "user-1",
		Title:     "Code Wiki",
		Kind:      domain.ChatSessionKindTask,
		RunID:     "run-1",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := store.CreateChatSession(context.Background(), taskSession); err != nil {
		t.Fatalf("create task session: %v", err)
	}

	chunks := []string{}
	result, err := service.StreamConversation(
		context.Background(),
		StreamConversationRequest{
			UserID:    "user-1",
			SessionID: taskSession.ID,
			Message:   "start task",
			Skill:     SkillAnswer,
		},
		StreamConversationHooks{
			OnChunk: func(chunk string) error {
				chunks = append(chunks, chunk)
				return nil
			},
		},
	)
	if err != nil {
		t.Fatalf("stream conversation: %v", err)
	}
	if !runner.called {
		t.Fatalf("expected task runner to be called")
	}
	if runner.request.RunID != "run-1" {
		t.Fatalf("expected runner run id run-1, got %s", runner.request.RunID)
	}
	if result.Answer != "# Code Wiki\n\nGenerated." {
		t.Fatalf("unexpected answer %q", result.Answer)
	}
	if strings.Join(chunks, "") != result.Answer {
		t.Fatalf("expected chunks to stream answer, got %#v", chunks)
	}

	messages, err := store.ListChatMessages(context.Background(), "user-1", taskSession.ID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected user and assistant messages, got %d", len(messages))
	}
	if messages[1].Content != result.Answer {
		t.Fatalf("expected assistant message to persist task answer, got %q", messages[1].Content)
	}
}

type fakeTaskConversationRunner struct {
	called  bool
	request TaskConversationRequest
	result  TaskConversationResult
}

func (r *fakeTaskConversationRunner) RunTaskConversation(
	_ context.Context,
	req TaskConversationRequest,
	onChunk func(string) error,
) (TaskConversationResult, error) {
	r.called = true
	r.request = req
	if onChunk != nil {
		if err := onChunk(r.result.Answer); err != nil {
			return TaskConversationResult{}, err
		}
	}
	return r.result, nil
}
