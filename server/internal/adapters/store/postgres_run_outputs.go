package store

import (
	"context"

	sqldb "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/db"
	"github.com/jackc/pgx/v5"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *PostgresStore) SaveArtifact(ctx context.Context, artifact domain.RunArtifact) error {
	_, err := s.queries.SaveArtifact(ctx, sqldb.SaveArtifactParams{
		ID:              artifact.ID,
		RunID:           artifact.RunID,
		Kind:            string(artifact.Kind),
		ContentMarkdown: artifact.ContentMarkdown,
		Version:         int32(artifact.Version),
		CreatedAt:       pgTimestamptz(artifact.CreatedAt),
	})
	return err
}

func (s *PostgresStore) ListArtifacts(ctx context.Context, runID string) ([]domain.RunArtifact, error) {
	rows, err := s.queries.ListArtifacts(ctx, runID)
	if err != nil {
		return nil, err
	}
	artifacts := make([]domain.RunArtifact, 0, len(rows))
	for _, row := range rows {
		artifacts = append(artifacts, domain.RunArtifact{
			ID:              row.ID,
			RunID:           row.RunID,
			Kind:            domain.ArtifactKind(row.Kind),
			ContentMarkdown: row.ContentMarkdown,
			Version:         int(row.Version),
			CreatedAt:       pgTimestamptzValue(row.CreatedAt),
		})
	}
	return artifacts, nil
}

func (s *PostgresStore) SaveSources(ctx context.Context, runID string, sources []domain.RunSource) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	queries := s.queries.WithTx(tx)
	if _, err := queries.DeleteRunSources(ctx, runID); err != nil {
		return err
	}
	for _, source := range sources {
		if _, err := queries.InsertRunSource(ctx, sqldb.InsertRunSourceParams{
			ID:           source.ID,
			RunID:        source.RunID,
			Provider:     string(source.Provider),
			ConnectionID: source.ConnectionID,
			DocumentID:   source.DocumentID,
			ChunkID:      source.ChunkID,
			Title:        source.Title,
			Repo:         source.Repo,
			Url:          source.URL,
			Snippet:      source.Snippet,
			Score:        source.Score,
			CreatedAt:    pgTimestamptz(source.CreatedAt),
		}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) ListSources(ctx context.Context, runID string) ([]domain.RunSource, error) {
	rows, err := s.queries.ListRunSources(ctx, runID)
	if err != nil {
		return nil, err
	}
	sources := make([]domain.RunSource, 0, len(rows))
	for _, row := range rows {
		sources = append(sources, domain.RunSource{
			ID:           row.ID,
			RunID:        row.RunID,
			Provider:     domain.Provider(row.Provider),
			ConnectionID: row.ConnectionID,
			DocumentID:   row.DocumentID,
			ChunkID:      row.ChunkID,
			Title:        row.Title,
			Repo:         row.Repo,
			URL:          row.Url,
			Snippet:      row.Snippet,
			Score:        row.Score,
			CreatedAt:    pgTimestamptzValue(row.CreatedAt),
		})
	}
	return sources, nil
}
