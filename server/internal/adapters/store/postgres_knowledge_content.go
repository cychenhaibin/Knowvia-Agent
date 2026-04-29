package store

import (
	"context"

	sqldb "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/db"
	"github.com/jackc/pgx/v5"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *PostgresStore) ListKnowledgeMetadata(ctx context.Context, connectionID string) ([]domain.KnowledgeDocumentMeta, error) {
	rows, err := s.queries.ListMetadata(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	docs := make([]domain.KnowledgeDocumentMeta, 0, len(rows))
	for _, row := range rows {
		docs = append(docs, domain.KnowledgeDocumentMeta{
			ID:           row.ID,
			UserID:       row.UserID,
			ConnectionID: row.ConnectionID,
			Repo:         row.Repo,
			Title:        row.Title,
			DocRef:       row.DocRef,
			SourceURL:    pgTextValue(row.SourceUrl),
			ChunkCount:   int(row.ChunkCount),
			UpdatedAt:    pgTimestamptzValue(row.UpdatedAt),
			CreatedAt:    pgTimestamptzValue(row.CreatedAt),
		})
	}
	return docs, nil
}

func (s *PostgresStore) ReplaceKnowledgeMetadata(ctx context.Context, connectionID string, docs []domain.KnowledgeDocumentMeta) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	queries := s.queries.WithTx(tx)
	if _, err := queries.DeleteMetadata(ctx, connectionID); err != nil {
		return err
	}
	for _, doc := range docs {
		if _, err := queries.InsertMetadata(ctx, sqldb.InsertMetadataParams{
			ID:           doc.ID,
			UserID:       doc.UserID,
			ConnectionID: doc.ConnectionID,
			Repo:         doc.Repo,
			Title:        doc.Title,
			DocRef:       doc.DocRef,
			SourceUrl:    pgNullableText(doc.SourceURL),
			ChunkCount:   int32(doc.ChunkCount),
			UpdatedAt:    pgTimestamptz(doc.UpdatedAt),
			CreatedAt:    pgTimestamptz(doc.CreatedAt),
		}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) ListKnowledgeSourceDocuments(ctx context.Context, connectionID string) ([]domain.KnowledgeSourceDocument, error) {
	rows, err := s.queries.ListSourceDocuments(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	docs := make([]domain.KnowledgeSourceDocument, 0, len(rows))
	for _, row := range rows {
		docs = append(docs, domain.KnowledgeSourceDocument{
			ID:              row.ID,
			UserID:          row.UserID,
			ConnectionID:    row.ConnectionID,
			Provider:        domain.Provider(row.Provider),
			ExternalID:      row.ExternalID,
			Repo:            row.Repo,
			Title:           row.Title,
			DocRef:          row.DocRef,
			SourceURL:       pgTextValue(row.SourceUrl),
			RawBody:         row.RawBody,
			BodyHash:        row.BodyHash,
			SourceUpdatedAt: pgTimestamptzValue(row.SourceUpdatedAt),
			CreatedAt:       pgTimestamptzValue(row.CreatedAt),
			UpdatedAt:       pgTimestamptzValue(row.UpdatedAt),
		})
	}
	return docs, nil
}

func (s *PostgresStore) ReplaceKnowledgeSourceDocuments(ctx context.Context, connectionID string, docs []domain.KnowledgeSourceDocument) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	queries := s.queries.WithTx(tx)
	if _, err := queries.DeleteSourceDocuments(ctx, connectionID); err != nil {
		return err
	}
	for _, doc := range docs {
		if _, err := queries.InsertSourceDocument(ctx, sqldb.InsertSourceDocumentParams{
			ID:              doc.ID,
			UserID:          doc.UserID,
			ConnectionID:    doc.ConnectionID,
			Provider:        string(doc.Provider),
			ExternalID:      doc.ExternalID,
			Repo:            doc.Repo,
			Title:           doc.Title,
			DocRef:          doc.DocRef,
			SourceUrl:       pgNullableText(doc.SourceURL),
			RawBody:         doc.RawBody,
			BodyHash:        doc.BodyHash,
			SourceUpdatedAt: pgTimestamptz(doc.SourceUpdatedAt),
			CreatedAt:       pgTimestamptz(doc.CreatedAt),
			UpdatedAt:       pgTimestamptz(doc.UpdatedAt),
		}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) GetKnowledgeCorpus(ctx context.Context, connectionID string) ([]domain.KnowledgeDocument, []domain.KnowledgeChunk, error) {
	docRows, err := s.queries.ListCorpusDocuments(ctx, connectionID)
	if err != nil {
		return nil, nil, err
	}
	docs := make([]domain.KnowledgeDocument, 0, len(docRows))
	for _, row := range docRows {
		docs = append(docs, domain.KnowledgeDocument{
			ID:           row.ID,
			UserID:       row.UserID,
			ConnectionID: row.ConnectionID,
			Repo:         row.Repo,
			Title:        row.Title,
			DocRef:       row.DocRef,
			SourceURL:    pgTextValue(row.SourceUrl),
			BodyHash:     row.BodyHash,
			UpdatedAt:    pgTimestamptzValue(row.UpdatedAt),
			CreatedAt:    pgTimestamptzValue(row.CreatedAt),
		})
	}

	chunkRows, err := s.queries.ListCorpusChunks(ctx, connectionID)
	if err != nil {
		return nil, nil, err
	}
	chunks := make([]domain.KnowledgeChunk, 0, len(chunkRows))
	for _, row := range chunkRows {
		chunks = append(chunks, domain.KnowledgeChunk{
			ID:           row.ID,
			UserID:       row.UserID,
			ConnectionID: row.ConnectionID,
			DocumentID:   row.DocumentID,
			ChunkIndex:   int(row.ChunkIndex),
			Content:      row.Content,
			ContentHash:  row.ContentHash,
			CreatedAt:    pgTimestamptzValue(row.CreatedAt),
		})
	}
	return docs, chunks, nil
}

func (s *PostgresStore) ReplaceKnowledgeCorpus(ctx context.Context, connectionID string, docs []domain.KnowledgeDocument, chunks []domain.KnowledgeChunk) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	queries := s.queries.WithTx(tx)
	if _, err := queries.DeleteCorpusDocuments(ctx, connectionID); err != nil {
		return err
	}
	for _, doc := range docs {
		if _, err := queries.InsertCorpusDocument(ctx, sqldb.InsertCorpusDocumentParams{
			ID:           doc.ID,
			UserID:       doc.UserID,
			ConnectionID: doc.ConnectionID,
			Repo:         doc.Repo,
			Title:        doc.Title,
			DocRef:       doc.DocRef,
			SourceUrl:    pgNullableText(doc.SourceURL),
			BodyHash:     doc.BodyHash,
			UpdatedAt:    pgTimestamptz(doc.UpdatedAt),
			CreatedAt:    pgTimestamptz(doc.CreatedAt),
		}); err != nil {
			return err
		}
	}
	for _, chunk := range chunks {
		if _, err := queries.InsertCorpusChunk(ctx, sqldb.InsertCorpusChunkParams{
			ID:           chunk.ID,
			UserID:       chunk.UserID,
			ConnectionID: chunk.ConnectionID,
			DocumentID:   chunk.DocumentID,
			ChunkIndex:   int32(chunk.ChunkIndex),
			Content:      chunk.Content,
			ContentHash:  chunk.ContentHash,
			CreatedAt:    pgTimestamptz(chunk.CreatedAt),
		}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
