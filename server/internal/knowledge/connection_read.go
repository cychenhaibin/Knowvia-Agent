package knowledge

import (
	"context"
	"errors"
	"sort"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/persistence"
)

func (s *Service) ListConnections(ctx context.Context, userID string) ([]domain.KnowledgeConnection, error) {
	return s.connectionStore.ListKnowledgeConnections(ctx, userID)
}

func (s *Service) ListConnectionDocuments(ctx context.Context, userID, connectionID string) (ListConnectionDocumentsResult, error) {
	connection, err := s.connectionStore.GetKnowledgeConnection(ctx, userID, connectionID)
	if err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return ListConnectionDocumentsResult{}, ErrConnectionNotFound
		}
		return ListConnectionDocumentsResult{}, err
	}

	docs, err := s.metadataStore.ListKnowledgeMetadata(ctx, connection.ID)
	if err != nil {
		return ListConnectionDocumentsResult{}, err
	}
	if len(docs) == 0 {
		legacyDocs, chunks, err := s.corpusStore.GetKnowledgeCorpus(ctx, connection.ID)
		if err != nil {
			return ListConnectionDocumentsResult{}, err
		}
		sort.Slice(legacyDocs, func(i, j int) bool {
			if legacyDocs[i].Repo != legacyDocs[j].Repo {
				return legacyDocs[i].Repo < legacyDocs[j].Repo
			}
			if !legacyDocs[i].UpdatedAt.Equal(legacyDocs[j].UpdatedAt) {
				return legacyDocs[i].UpdatedAt.After(legacyDocs[j].UpdatedAt)
			}
			return legacyDocs[i].Title < legacyDocs[j].Title
		})

		repos := summarizeDocumentRepos(legacyDocs, func(doc domain.KnowledgeDocument) string { return doc.Repo })
		return ListConnectionDocumentsResult{
			Connection:    connection,
			RepoCount:     len(repos),
			DocumentCount: len(legacyDocs),
			ChunkCount:    len(chunks),
			Repos:         repos,
			Documents:     legacyDocs,
		}, nil
	}

	sort.Slice(docs, func(i, j int) bool {
		if docs[i].Repo != docs[j].Repo {
			return docs[i].Repo < docs[j].Repo
		}
		if !docs[i].UpdatedAt.Equal(docs[j].UpdatedAt) {
			return docs[i].UpdatedAt.After(docs[j].UpdatedAt)
		}
		return docs[i].Title < docs[j].Title
	})

	chunkCount := 0
	for _, doc := range docs {
		chunkCount += doc.ChunkCount
	}
	repos := summarizeDocumentRepos(docs, func(doc domain.KnowledgeDocumentMeta) string { return doc.Repo })
	return ListConnectionDocumentsResult{
		Connection:    connection,
		RepoCount:     len(repos),
		DocumentCount: len(docs),
		ChunkCount:    chunkCount,
		Repos:         repos,
		Documents:     docs,
	}, nil
}

func (s *Service) ListSyncJobs(ctx context.Context, userID, connectionID string) ([]domain.KnowledgeSyncJob, error) {
	jobs, err := s.syncJobStore.ListKnowledgeSyncJobs(ctx, userID, connectionID)
	if err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return nil, ErrConnectionNotFound
		}
		return nil, err
	}
	return jobs, nil
}

func summarizeDocumentRepos[T any](docs []T, repoOf func(T) string) []RepoSummary {
	repoCounts := map[string]int{}
	for _, doc := range docs {
		repoCounts[repoOf(doc)]++
	}

	repos := make([]RepoSummary, 0, len(repoCounts))
	for name, count := range repoCounts {
		repos = append(repos, RepoSummary{Name: name, DocumentCount: count})
	}
	sort.Slice(repos, func(i, j int) bool {
		return repos[i].Name < repos[j].Name
	})
	return repos
}
