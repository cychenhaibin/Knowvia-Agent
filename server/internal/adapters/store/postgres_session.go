package store

import (
	"context"

	sqldb "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/db"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *PostgresStore) CreateSession(ctx context.Context, session domain.Session) error {
	_, err := s.queries.CreateSession(ctx, sqldb.CreateSessionParams{
		ID:           session.ID,
		UserID:       session.UserID,
		AccessToken:  session.AccessToken,
		RefreshToken: session.RefreshToken,
		ExpiresAt:    pgTimestamptz(session.ExpiresAt),
		RevokedAt:    pgNullableTimestamptz(session.RevokedAt),
		CreatedAt:    pgTimestamptz(session.CreatedAt),
	})
	return err
}

func (s *PostgresStore) GetSessionByRefreshToken(ctx context.Context, refreshToken string) (domain.Session, error) {
	session, err := s.queries.GetSessionByRefreshToken(ctx, refreshToken)
	if err != nil {
		return domain.Session{}, normalizeError(err)
	}
	return mapDBSession(session), nil
}

func (s *PostgresStore) RevokeSession(ctx context.Context, sessionID string) error {
	rowsAffected, err := s.queries.RevokeSession(ctx, sessionID)
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func mapDBSession(session sqldb.Session) domain.Session {
	return domain.Session{
		ID:           session.ID,
		UserID:       session.UserID,
		AccessToken:  session.AccessToken,
		RefreshToken: session.RefreshToken,
		ExpiresAt:    pgTimestamptzValue(session.ExpiresAt),
		CreatedAt:    pgTimestamptzValue(session.CreatedAt),
		RevokedAt:    pgNullableTimestamptzPtr(session.RevokedAt),
	}
}
