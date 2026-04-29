package store

import (
	"context"
	"sort"
	"strings"

	sqldb "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/db"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *PostgresStore) SearchKnowledge(ctx context.Context, userID string, connectionIDs []string, query string, limit int) ([]domain.KnowledgeHit, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []domain.KnowledgeHit{}, nil
	}

	var (
		selectedRows []sqldb.SearchKnowledgeSelectedConnectionsRow
		allRows      []sqldb.SearchKnowledgeAllConnectionsRow
		err          error
	)
	hits := []domain.KnowledgeHit{}
	if len(connectionIDs) > 0 {
		selectedRows, err = s.queries.SearchKnowledgeSelectedConnections(ctx, sqldb.SearchKnowledgeSelectedConnectionsParams{
			UserID:  userID,
			Column2: connectionIDs,
		})
	} else {
		allRows, err = s.queries.SearchKnowledgeAllConnections(ctx, userID)
	}
	if err != nil {
		return nil, err
	}

	tokens := tokenize(query)
	loweredQuery := strings.ToLower(query)
	appendHit := func(connectionID, documentID, chunkID, provider, title, repo, url, content string) {
		hit := domain.KnowledgeHit{
			ConnectionID: connectionID,
			DocumentID:   documentID,
			ChunkID:      chunkID,
			Provider:     provider,
			Title:        title,
			Repo:         repo,
			URL:          url,
		}
		hit.Score = boostedScore(hit.Title, hit.Repo, content, tokens)
		if hit.Score == 0 &&
			!strings.Contains(strings.ToLower(content), loweredQuery) &&
			!strings.Contains(strings.ToLower(hit.Title), loweredQuery) &&
			!strings.Contains(strings.ToLower(hit.Repo), loweredQuery) {
			return
		}
		if hit.Score == 0 {
			hit.Score = 1
		}
		hit.Snippet = snippet(content, tokens, 220)
		hits = append(hits, hit)
	}
	for _, row := range allRows {
		appendHit(row.ConnectionID, row.ID, row.ID_2, row.Provider, row.Title, row.Repo, row.SourceUrl, row.Content)
	}
	for _, row := range selectedRows {
		appendHit(row.ConnectionID, row.ID, row.ID_2, row.Provider, row.Title, row.Repo, row.SourceUrl, row.Content)
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
