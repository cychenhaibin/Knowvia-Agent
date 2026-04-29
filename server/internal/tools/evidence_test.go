package tools

import (
	"context"
	"testing"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/provider"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

type fakeEvidenceMergeClient struct {
	request provider.ForwardedEvidenceMergeRequest
	result  provider.ForwardedEvidenceMergeResult
	err     error
}

func (f *fakeEvidenceMergeClient) MergeEvidence(
	_ context.Context,
	req provider.ForwardedEvidenceMergeRequest,
) (provider.ForwardedEvidenceMergeResult, error) {
	f.request = req
	return f.result, f.err
}

func TestEvidenceMergeToolPrefersForwardedMerge(t *testing.T) {
	forward := &fakeEvidenceMergeClient{
		result: provider.ForwardedEvidenceMergeResult{
			Goal: "总结",
			Mode: "kb_only",
			Evidences: []provider.ForwardedMergedEvidence{
				{
					Provider:     "yuque",
					ConnectionID: "conn-1",
					DocumentID:   "doc-1",
					ChunkID:      "chunk-1",
					Title:        "发布说明",
					Repo:         "产品文档",
					Snippet:      "新增 timeline",
					Score:        2.0,
				},
			},
		},
	}

	tool := NewEvidenceMergeTool(forward)
	result, err := tool.Merge(
		context.Background(),
		"user-1",
		"总结",
		domain.RunModeKBOnly,
		[]domain.Evidence{
			{Provider: domain.ProviderYuque, ConnectionID: "conn-1", Title: "发布说明", Snippet: "新增 timeline", Score: 1.0},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if forward.request.TopK != 1 {
		t.Fatalf("expected top_k 1, got %d", forward.request.TopK)
	}
	if len(result.Evidences) != 1 || result.Evidences[0].Score != 2.0 {
		t.Fatalf("expected forwarded evidence, got %+v", result.Evidences)
	}
	if result.GroupCount != 1 || result.ConflictCount != 0 {
		t.Fatalf("expected merge diagnostics, got %+v", result)
	}
}

func TestEvidenceMergeToolFallsBackLocally(t *testing.T) {
	tool := NewEvidenceMergeTool(&fakeEvidenceMergeClient{err: context.DeadlineExceeded})
	result, err := tool.Merge(
		context.Background(),
		"user-1",
		"总结",
		domain.RunModeKBOnly,
		[]domain.Evidence{
			{Provider: domain.ProviderYuque, Title: "B", URL: "u2", Score: 0.5},
			{Provider: domain.ProviderYuque, Title: "A", URL: "u1", Score: 0.9},
			{Provider: domain.ProviderYuque, Title: "A", URL: "u1", Score: 0.7},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if len(result.Evidences) != 2 {
		t.Fatalf("expected deduped evidence, got %+v", result.Evidences)
	}
	if result.Evidences[0].Title != "A" {
		t.Fatalf("expected score-sorted fallback results, got %+v", result.Evidences)
	}
	if result.DuplicateCount != 1 {
		t.Fatalf("expected duplicate count 1, got %+v", result)
	}
}
