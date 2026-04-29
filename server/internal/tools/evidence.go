package tools

import (
	"context"
	"slices"
	"strings"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/provider"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

// EvidenceMergeTool lets the Python llm own the run-time evidence merge step
// while keeping a deterministic local fallback when the forward client is
// unavailable.
type EvidenceMergeTool struct {
	forward provider.EvidenceMergeForwardClient
}

func NewEvidenceMergeTool(forward provider.EvidenceMergeForwardClient) *EvidenceMergeTool {
	return &EvidenceMergeTool{forward: forward}
}

func (t *EvidenceMergeTool) Merge(
	ctx context.Context,
	userID string,
	goal string,
	mode domain.RunMode,
	evidences []domain.Evidence,
	snapshot *domain.SkillRuntimeSnapshot,
) (EvidenceMergeResult, error) {
	if t.forward != nil && strings.TrimSpace(userID) != "" {
		result, err := t.forward.MergeEvidence(ctx, provider.ForwardedEvidenceMergeRequest{
			UserID:        strings.TrimSpace(userID),
			Goal:          strings.TrimSpace(goal),
			Mode:          string(mode),
			SkillSnapshot: forwardedSkillSnapshot(snapshot),
			Evidences:     forwardedEvidence(evidences),
			TopK:          len(evidences),
		})
		if err == nil {
			normalized := EvidenceMergeResult{
				Evidences:      mergedEvidenceToDomain(result.Evidences),
				DuplicateCount: result.DuplicateCount,
				GroupCount:     result.GroupCount,
				ConflictCount:  result.ConflictCount,
				GroupLabels:    append([]string(nil), result.GroupLabels...),
				Warnings:       append([]string(nil), result.Warnings...),
			}
			if normalized.GroupCount == 0 && len(normalized.Evidences) > 0 {
				normalized.GroupCount = len(normalized.Evidences)
			}
			if normalized.GroupLabels == nil {
				normalized.GroupLabels = evidenceTitles(normalized.Evidences)
			}
			return normalized, nil
		}
	}
	fallback := LocalMergeEvidence(evidences)
	return EvidenceMergeResult{
		Evidences:      fallback,
		DuplicateCount: maxInt(0, len(evidences)-len(fallback)),
		GroupCount:     len(fallback),
		ConflictCount:  0,
		GroupLabels:    evidenceTitles(fallback),
		Warnings:       nil,
	}, nil
}

func mergedEvidenceToDomain(items []provider.ForwardedMergedEvidence) []domain.Evidence {
	result := make([]domain.Evidence, 0, len(items))
	for _, item := range items {
		providerName := domain.ProviderYuque
		switch item.Provider {
		case string(domain.ProviderFeishu):
			providerName = domain.ProviderFeishu
		case string(domain.ProviderWeb):
			providerName = domain.ProviderWeb
		}
		result = append(result, domain.Evidence{
			Provider:     providerName,
			ConnectionID: item.ConnectionID,
			DocumentID:   item.DocumentID,
			ChunkID:      item.ChunkID,
			Title:        item.Title,
			Repo:         item.Repo,
			URL:          item.URL,
			Snippet:      item.Snippet,
			Body:         item.Body,
			Score:        item.Score,
		})
	}
	return result
}

// LocalMergeEvidence mirrors the previous Go-only merge behavior.
func LocalMergeEvidence(items []domain.Evidence) []domain.Evidence {
	keys := map[string]int{}
	result := []domain.Evidence{}
	for _, item := range items {
		key := strings.Join([]string{string(item.Provider), item.Title, item.URL, item.ChunkID}, "|")
		if idx, ok := keys[key]; ok {
			if item.Score > result[idx].Score {
				result[idx] = item
			}
			continue
		}
		keys[key] = len(result)
		result = append(result, item)
	}
	slices.SortFunc(result, func(a, b domain.Evidence) int {
		switch {
		case a.Score > b.Score:
			return -1
		case a.Score < b.Score:
			return 1
		default:
			return strings.Compare(a.Title, b.Title)
		}
	})
	return result
}

func mergeEvidence(base, extra []domain.Evidence) []domain.Evidence {
	return LocalMergeEvidence(append(base, extra...))
}

func evidenceTitles(items []domain.Evidence) []string {
	titles := make([]string, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Title) == "" {
			continue
		}
		titles = append(titles, item.Title)
	}
	return titles
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
