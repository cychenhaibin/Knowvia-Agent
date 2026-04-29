package store

import (
	"context"
	"encoding/json"
	"strings"

	sqldb "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/db"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *PostgresStore) CreateChatSession(ctx context.Context, session domain.ChatSession) error {
	_, err := s.queries.CreateChatSession(ctx, sqldb.CreateChatSessionParams{
		ID:            session.ID,
		UserID:        session.UserID,
		Title:         session.Title,
		Pinned:        session.Pinned,
		LastMessageAt: pgNullableTimestamptz(session.LastMessageAt),
		CreatedAt:     pgTimestamptz(session.CreatedAt),
		UpdatedAt:     pgTimestamptz(session.UpdatedAt),
	})
	return err
}

func (s *PostgresStore) GetChatSession(ctx context.Context, userID, sessionID string) (domain.ChatSession, error) {
	session, err := s.queries.GetChatSession(ctx, sqldb.GetChatSessionParams{
		ID:     sessionID,
		UserID: userID,
	})
	if err != nil {
		return domain.ChatSession{}, normalizeError(err)
	}
	return mapDBChatSession(session), nil
}

func (s *PostgresStore) UpdateChatSession(ctx context.Context, session domain.ChatSession) error {
	rowsAffected, err := s.queries.UpdateChatSession(ctx, sqldb.UpdateChatSessionParams{
		ID:            session.ID,
		UserID:        session.UserID,
		Title:         session.Title,
		Pinned:        session.Pinned,
		LastMessageAt: pgNullableTimestamptz(session.LastMessageAt),
		CreatedAt:     pgTimestamptz(session.CreatedAt),
		UpdatedAt:     pgTimestamptz(session.UpdatedAt),
	})
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) ListChatSessions(ctx context.Context, userID string) ([]domain.ChatSession, error) {
	rows, err := s.queries.ListChatSessions(ctx, userID)
	if err != nil {
		return nil, err
	}
	sessions := make([]domain.ChatSession, 0, len(rows))
	for _, row := range rows {
		sessions = append(sessions, mapDBChatSession(row))
	}
	return sessions, nil
}

func (s *PostgresStore) DeleteChatSession(ctx context.Context, userID, sessionID string) error {
	rowsAffected, err := s.queries.DeleteChatSession(ctx, sqldb.DeleteChatSessionParams{
		ID:     sessionID,
		UserID: userID,
	})
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) SaveChatMessage(ctx context.Context, message domain.ChatMessage) error {
	_, err := s.queries.SaveChatMessage(ctx, sqldb.SaveChatMessageParams{
		ID:               message.ID,
		SessionID:        message.SessionID,
		UserID:           message.UserID,
		Role:             string(message.Role),
		Content:          message.Content,
		Skill:            message.Skill,
		UseKnowledge:     message.UseKnowledge,
		PromptTokens:     int32(chatUsageValue(message.Usage, "prompt")),
		CompletionTokens: int32(chatUsageValue(message.Usage, "completion")),
		TotalTokens:      int32(chatUsageValue(message.Usage, "total")),
		CreatedAt:        pgTimestamptz(message.CreatedAt),
		CompletedAt:      pgNullableTimestamptz(message.CompletedAt),
	})
	return err
}

func (s *PostgresStore) ListChatMessages(ctx context.Context, userID, sessionID string) ([]domain.ChatMessage, error) {
	rows, err := s.queries.ListChatMessages(ctx, sqldb.ListChatMessagesParams{
		SessionID: sessionID,
		UserID:    userID,
	})
	if err != nil {
		return nil, err
	}
	messages := make([]domain.ChatMessage, 0, len(rows))
	for _, row := range rows {
		messages = append(messages, mapDBChatMessage(row))
	}
	return messages, nil
}

func (s *PostgresStore) SaveChatMessageSources(ctx context.Context, messageID string, sources []domain.ChatMessageSource) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	queries := s.queries.WithTx(tx)
	if _, err := queries.DeleteChatMessageSources(ctx, messageID); err != nil {
		return err
	}
	for _, source := range sources {
		matchedLinesJSON, err := json.Marshal(source.MatchedLines)
		if err != nil {
			return err
		}
		if _, err := queries.InsertChatMessageSource(ctx, sqldb.InsertChatMessageSourceParams{
			ID:               source.ID,
			MessageID:        source.MessageID,
			Provider:         string(source.Provider),
			ConnectionID:     source.ConnectionID,
			DocumentID:       source.DocumentID,
			ChunkID:          source.ChunkID,
			Title:            source.Title,
			Repo:             source.Repo,
			Url:              source.URL,
			Snippet:          source.Snippet,
			MatchedLinesJson: string(matchedLinesJSON),
			Score:            source.Score,
			CreatedAt:        pgTimestamptz(source.CreatedAt),
		}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) ListChatMessageSources(ctx context.Context, messageID string) ([]domain.ChatMessageSource, error) {
	rows, err := s.queries.ListChatMessageSources(ctx, messageID)
	if err != nil {
		return nil, err
	}
	sources := make([]domain.ChatMessageSource, 0, len(rows))
	for _, row := range rows {
		sources = append(sources, mapDBChatMessageSource(
			row.ID, row.MessageID, row.Provider, row.ConnectionID, row.DocumentID, row.ChunkID,
			row.Title, row.Repo, row.Url, row.Snippet, row.MatchedLinesJson, row.Score, row.CreatedAt,
		))
	}
	return sources, nil
}

func mapDBChatSession(session sqldb.ChatSession) domain.ChatSession {
	return domain.ChatSession{
		ID:            session.ID,
		UserID:        session.UserID,
		Title:         session.Title,
		Pinned:        session.Pinned,
		LastMessageAt: pgNullableTimestamptzPtr(session.LastMessageAt),
		CreatedAt:     pgTimestamptzValue(session.CreatedAt),
		UpdatedAt:     pgTimestamptzValue(session.UpdatedAt),
	}
}

func mapDBChatMessage(message sqldb.ChatMessage) domain.ChatMessage {
	return domain.ChatMessage{
		ID:           message.ID,
		SessionID:    message.SessionID,
		UserID:       message.UserID,
		Role:         domain.ChatRole(message.Role),
		Content:      message.Content,
		Skill:        message.Skill,
		UseKnowledge: message.UseKnowledge,
		Usage:        mapDBChatUsage(message.PromptTokens, message.CompletionTokens, message.TotalTokens),
		CreatedAt:    pgTimestamptzValue(message.CreatedAt),
		CompletedAt:  pgNullableTimestamptzPtr(message.CompletedAt),
	}
}

func mapDBChatUsage(promptTokens, completionTokens, totalTokens int32) *domain.ChatUsage {
	usage := domain.ChatUsage{
		PromptTokens:     int(promptTokens),
		CompletionTokens: int(completionTokens),
		TotalTokens:      int(totalTokens),
	}
	if usage.IsZero() {
		return nil
	}
	return &usage
}

func chatUsageValue(usage *domain.ChatUsage, kind string) int {
	if usage == nil {
		return 0
	}
	switch kind {
	case "prompt":
		return usage.PromptTokens
	case "completion":
		return usage.CompletionTokens
	case "total":
		return usage.TotalTokens
	default:
		return 0
	}
}

func mapDBChatMessageSource(id, messageID, provider, connectionID, documentID, chunkID, title, repo, url, snippet, matchedLinesJSON string, score float64, createdAt pgtype.Timestamptz) domain.ChatMessageSource {
	source := domain.ChatMessageSource{
		ID:           id,
		MessageID:    messageID,
		Provider:     domain.Provider(provider),
		ConnectionID: connectionID,
		DocumentID:   documentID,
		ChunkID:      chunkID,
		Title:        title,
		Repo:         repo,
		URL:          url,
		Snippet:      snippet,
		Score:        score,
		CreatedAt:    pgTimestamptzValue(createdAt),
	}
	if strings.TrimSpace(matchedLinesJSON) != "" {
		_ = json.Unmarshal([]byte(matchedLinesJSON), &source.MatchedLines)
	}
	return source
}
