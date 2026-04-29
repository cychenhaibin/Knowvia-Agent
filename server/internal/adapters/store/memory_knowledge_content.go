package store

import (
	"context"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *MemoryStore) ListKnowledgeMetadata(_ context.Context, connectionID string) ([]domain.KnowledgeDocumentMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]domain.KnowledgeDocumentMeta(nil), s.metadata[connectionID]...), nil
}

func (s *MemoryStore) ReplaceKnowledgeMetadata(_ context.Context, connectionID string, docs []domain.KnowledgeDocumentMeta) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metadata[connectionID] = append([]domain.KnowledgeDocumentMeta(nil), docs...)
	return nil
}

func (s *MemoryStore) ListKnowledgeSourceDocuments(_ context.Context, connectionID string) ([]domain.KnowledgeSourceDocument, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]domain.KnowledgeSourceDocument(nil), s.sourceDocuments[connectionID]...), nil
}

func (s *MemoryStore) ReplaceKnowledgeSourceDocuments(_ context.Context, connectionID string, docs []domain.KnowledgeSourceDocument) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sourceDocuments[connectionID] = append([]domain.KnowledgeSourceDocument(nil), docs...)
	return nil
}

func (s *MemoryStore) GetKnowledgeCorpus(_ context.Context, connectionID string) ([]domain.KnowledgeDocument, []domain.KnowledgeChunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	docs := append([]domain.KnowledgeDocument(nil), s.documents[connectionID]...)
	chunks := append([]domain.KnowledgeChunk(nil), s.chunks[connectionID]...)
	return docs, chunks, nil
}

func (s *MemoryStore) ReplaceKnowledgeCorpus(_ context.Context, connectionID string, docs []domain.KnowledgeDocument, chunks []domain.KnowledgeChunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.documents[connectionID] = append([]domain.KnowledgeDocument(nil), docs...)
	s.chunks[connectionID] = append([]domain.KnowledgeChunk(nil), chunks...)
	return nil
}
