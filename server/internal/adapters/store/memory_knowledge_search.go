package store

import (
	"context"
	"sort"
	"strings"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *MemoryStore) SearchKnowledge(_ context.Context, userID string, connectionIDs []string, query string, limit int) ([]domain.KnowledgeHit, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	allowed := map[string]struct{}{}
	if len(connectionIDs) == 0 {
		for id, connection := range s.connections {
			if connection.UserID == userID {
				allowed[id] = struct{}{}
			}
		}
	} else {
		for _, id := range connectionIDs {
			if connection, ok := s.connections[id]; ok && connection.UserID == userID {
				allowed[id] = struct{}{}
			}
		}
	}

	tokens := tokenize(query)
	loweredQuery := strings.ToLower(strings.TrimSpace(query))
	hits := []domain.KnowledgeHit{}
	for connectionID := range allowed {
		docIndex := map[string]domain.KnowledgeDocument{}
		for _, doc := range s.documents[connectionID] {
			docIndex[doc.ID] = doc
		}
		for _, chunk := range s.chunks[connectionID] {
			doc := docIndex[chunk.DocumentID]
			connection := s.connections[connectionID]
			score := boostedScore(doc.Title, doc.Repo, chunk.Content, tokens)
			if score == 0 &&
				!strings.Contains(strings.ToLower(chunk.Content), loweredQuery) &&
				!strings.Contains(strings.ToLower(doc.Title), loweredQuery) &&
				!strings.Contains(strings.ToLower(doc.Repo), loweredQuery) {
				continue
			}
			if score == 0 {
				score = 1
			}
			hits = append(hits, domain.KnowledgeHit{
				ConnectionID: connectionID,
				DocumentID:   chunk.DocumentID,
				ChunkID:      chunk.ID,
				Provider:     string(connection.Provider),
				Title:        doc.Title,
				Repo:         doc.Repo,
				URL:          doc.SourceURL,
				Snippet:      snippet(chunk.Content, tokens, 220),
				Score:        score,
			})
		}
	}

	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].Title < hits[j].Title
		}
		return hits[i].Score > hits[j].Score
	})

	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}
