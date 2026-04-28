package tools

import (
	"context"
	"testing"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/provider"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/store"
)

type fakeKnowledgeRetrieveClient struct {
	request provider.ForwardedRetrieveRequest
	result  provider.ForwardedRetrieveResult
	err     error
}

func (f *fakeKnowledgeRetrieveClient) RetrieveKnowledge(
	_ context.Context,
	req provider.ForwardedRetrieveRequest,
) (provider.ForwardedRetrieveResult, error) {
	f.request = req
	return f.result, f.err
}

func TestKnowledgeSearchToolPrefersForwardedRetrieval(t *testing.T) {
	mem := store.NewMemoryStore()
	forward := &fakeKnowledgeRetrieveClient{
		result: provider.ForwardedRetrieveResult{
			Query: "timeline",
			Sources: []provider.ForwardedSource{
				{
					Type:         "knowledge_base",
					Provider:     "feishu",
					ConnectionID: "conn-1",
					DocumentID:   "doc-1",
					Title:        "发布说明",
					Repo:         "产品文档",
					URL:          "https://example.com/release",
					Snippet:      "新增 timeline 视图",
					Score:        1.4,
				},
			},
		},
	}

	tool := NewKnowledgeSearchTool(mem, forward)
	items, err := tool.Search(
		context.Background(),
		"user-1",
		[]string{"conn-1"},
		"timeline",
		4,
		&TraceContext{RunID: "run-1", SkillID: "skill-1"},
	)
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	if forward.request.TopK != 4 {
		t.Fatalf("expected top_k 4, got %d", forward.request.TopK)
	}
	if forward.request.Trace == nil || forward.request.Trace.RunID != "run-1" {
		t.Fatalf("expected trace context to be forwarded, got %#v", forward.request.Trace)
	}
	if len(items) != 1 || items[0].Provider != domain.ProviderFeishu {
		t.Fatalf("expected forwarded feishu evidence, got %+v", items)
	}
	if items[0].ChunkID != "" {
		t.Fatalf("expected empty chunk id from fixture, got %q", items[0].ChunkID)
	}
}

func TestKnowledgeSearchToolFallsBackToLocalStore(t *testing.T) {
	mem := store.NewMemoryStore()
	now := time.Now().UTC()
	if err := mem.CreateKnowledgeConnection(context.Background(), domain.KnowledgeConnection{
		ID:        "conn-1",
		UserID:    "user-1",
		Provider:  domain.ProviderYuque,
		Name:      "产品文档",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create connection: %v", err)
	}
	if err := mem.ReplaceKnowledgeCorpus(context.Background(), "conn-1",
		[]domain.KnowledgeDocument{
			{
				ID:           "doc-1",
				UserID:       "user-1",
				ConnectionID: "conn-1",
				Repo:         "产品文档",
				Title:        "发布说明",
				DocRef:       "release-note",
				SourceURL:    "https://example.com/release",
				UpdatedAt:    now,
				CreatedAt:    now,
			},
		},
		[]domain.KnowledgeChunk{
			{
				ID:           "chunk-1",
				UserID:       "user-1",
				ConnectionID: "conn-1",
				DocumentID:   "doc-1",
				ChunkIndex:   0,
				Content:      "Knowvia 新增 timeline 和 artifact 视图。",
				ContentHash:  "hash-1",
				CreatedAt:    now,
			},
		},
	); err != nil {
		t.Fatalf("replace corpus: %v", err)
	}

	tool := NewKnowledgeSearchTool(mem, &fakeKnowledgeRetrieveClient{err: context.DeadlineExceeded})
	items, err := tool.Search(context.Background(), "user-1", []string{"conn-1"}, "timeline", 4, nil)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(items) != 1 || items[0].Title != "发布说明" {
		t.Fatalf("expected local fallback result, got %+v", items)
	}
}
