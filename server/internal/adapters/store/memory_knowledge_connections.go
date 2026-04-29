package store

import (
	"context"
	"sort"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *MemoryStore) CreateKnowledgeConnection(_ context.Context, connection domain.KnowledgeConnection) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connections[connection.ID] = connection
	return nil
}

func (s *MemoryStore) UpdateKnowledgeConnection(_ context.Context, connection domain.KnowledgeConnection) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connections[connection.ID] = connection
	return nil
}

func (s *MemoryStore) GetKnowledgeConnection(_ context.Context, userID, connectionID string) (domain.KnowledgeConnection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	connection, ok := s.connections[connectionID]
	if !ok || connection.UserID != userID {
		return domain.KnowledgeConnection{}, ErrNotFound
	}
	return connection, nil
}

func (s *MemoryStore) GetKnowledgeConnectionByID(_ context.Context, connectionID string) (domain.KnowledgeConnection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	connection, ok := s.connections[connectionID]
	if !ok {
		return domain.KnowledgeConnection{}, ErrNotFound
	}
	return connection, nil
}

func (s *MemoryStore) ListKnowledgeConnections(_ context.Context, userID string) ([]domain.KnowledgeConnection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	connections := []domain.KnowledgeConnection{}
	for _, connection := range s.connections {
		if connection.UserID == userID {
			connections = append(connections, connection)
		}
	}
	sort.Slice(connections, func(i, j int) bool {
		return connections[i].UpdatedAt.After(connections[j].UpdatedAt)
	})
	return connections, nil
}

func (s *MemoryStore) DeleteKnowledgeConnection(_ context.Context, userID, connectionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	connection, ok := s.connections[connectionID]
	if !ok || connection.UserID != userID {
		return ErrNotFound
	}

	delete(s.connections, connectionID)
	delete(s.metadata, connectionID)
	delete(s.sourceDocuments, connectionID)
	delete(s.documents, connectionID)
	delete(s.chunks, connectionID)
	for jobID, job := range s.syncJobs {
		if job.ConnectionID == connectionID && job.UserID == userID {
			delete(s.syncJobs, jobID)
		}
	}
	return nil
}
