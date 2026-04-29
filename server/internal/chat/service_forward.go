package chat

import (
	"encoding/json"
	"strings"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/provider"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func forwardedSkillSnapshot(snapshot *domain.SkillRuntimeSnapshot) *provider.ForwardedSkillSnapshot {
	if snapshot == nil {
		return nil
	}
	runtimeSpec := map[string]any{}
	if strings.TrimSpace(snapshot.RuntimeSpecJSON) != "" {
		_ = json.Unmarshal([]byte(snapshot.RuntimeSpecJSON), &runtimeSpec)
	}
	return &provider.ForwardedSkillSnapshot{
		SnapshotID:     snapshot.ID,
		InstallationID: snapshot.InstallationID,
		DefinitionID:   snapshot.DefinitionID,
		RevisionID:     snapshot.RevisionID,
		Kind:           string(snapshot.Kind),
		Title:          snapshot.Title,
		Description:    snapshot.Description,
		Mode:           snapshot.Mode,
		Prompt:         snapshot.Prompt,
		RuntimeSpec:    runtimeSpec,
	}
}

func forwardedSourcesToEvidence(items []provider.ForwardedSource) []domain.Evidence {
	sources := make([]domain.Evidence, 0, len(items))
	for _, source := range items {
		providerName := domain.ProviderWeb
		if strings.EqualFold(source.Type, "knowledge_base") {
			switch strings.ToLower(strings.TrimSpace(source.Provider)) {
			case string(domain.ProviderFeishu):
				providerName = domain.ProviderFeishu
			default:
				providerName = domain.ProviderYuque
			}
		}
		sources = append(sources, domain.Evidence{
			Provider:     providerName,
			ConnectionID: strings.TrimSpace(firstNonEmpty(source.ConnectionID, source.ScopeID)),
			DocumentID:   strings.TrimSpace(source.DocumentID),
			ChunkID:      strings.TrimSpace(source.ChunkID),
			Title:        strings.TrimSpace(source.Title),
			Repo:         strings.TrimSpace(source.Repo),
			URL:          strings.TrimSpace(source.URL),
			Snippet:      strings.TrimSpace(source.Snippet),
			MatchedLines: append([]string(nil), source.MatchedLines...),
			Score:        source.Score,
		})
	}
	return sources
}
