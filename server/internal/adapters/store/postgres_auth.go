package store

import (
	"context"
	"strings"

	sqldb "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/db"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *PostgresStore) UpsertUser(ctx context.Context, user domain.User) error {
	_, err := s.queries.UpsertUser(ctx, sqldb.UpsertUserParams{
		ID:           user.ID,
		Username:     user.Username,
		DisplayName:  user.DisplayName,
		Email:        user.Email,
		AvatarUrl:    user.AvatarURL,
		PasswordHash: user.PasswordHash,
		CreatedAt:    pgTimestamptz(user.CreatedAt),
	})
	return err
}

func (s *PostgresStore) GetUserByUsername(ctx context.Context, username string) (domain.User, error) {
	row, err := s.queries.GetUserByUsername(ctx, username)
	if err != nil {
		return domain.User{}, normalizeError(err)
	}
	return mapDBUser(row.ID, row.Username, row.DisplayName, row.Email, row.AvatarUrl, row.PasswordHash, row.CreatedAt), nil
}

func (s *PostgresStore) GetUserByID(ctx context.Context, userID string) (domain.User, error) {
	row, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		return domain.User{}, normalizeError(err)
	}
	return mapDBUser(row.ID, row.Username, row.DisplayName, row.Email, row.AvatarUrl, row.PasswordHash, row.CreatedAt), nil
}

func (s *PostgresStore) GetUserByAuthIdentity(ctx context.Context, provider domain.AuthProvider, subject string) (domain.User, error) {
	row, err := s.queries.GetUserByAuthIdentity(ctx, sqldb.GetUserByAuthIdentityParams{
		Provider:        string(provider),
		ProviderSubject: strings.TrimSpace(subject),
	})
	if err != nil {
		return domain.User{}, normalizeError(err)
	}
	return mapDBUser(row.ID, row.Username, row.DisplayName, row.Email, row.AvatarUrl, row.PasswordHash, row.CreatedAt), nil
}

func (s *PostgresStore) UpsertAuthIdentity(ctx context.Context, identity domain.AuthIdentity) error {
	_, err := s.queries.UpsertAuthIdentity(ctx, sqldb.UpsertAuthIdentityParams{
		ID:              identity.ID,
		UserID:          identity.UserID,
		Provider:        string(identity.Provider),
		ProviderSubject: identity.ProviderSubject,
		Email:           identity.Email,
		EmailVerified:   identity.EmailVerified,
		AvatarUrl:       identity.AvatarURL,
		CreatedAt:       pgTimestamptz(identity.CreatedAt),
		UpdatedAt:       pgTimestamptz(identity.UpdatedAt),
	})
	return err
}

func (s *PostgresStore) UpsertFluxACredential(ctx context.Context, credential domain.FluxACredential) error {
	_, err := s.queries.UpsertFluxACredential(ctx, sqldb.UpsertFluxACredentialParams{
		UserID:          credential.UserID,
		Site:            string(credential.Site),
		TokenCiphertext: credential.TokenCiphertext,
		CreatedAt:       pgTimestamptz(credential.CreatedAt),
		UpdatedAt:       pgTimestamptz(credential.UpdatedAt),
	})
	return err
}

func (s *PostgresStore) GetFluxACredential(ctx context.Context, userID string, site domain.FluxASite) (domain.FluxACredential, error) {
	row, err := s.queries.GetFluxACredential(ctx, sqldb.GetFluxACredentialParams{UserID: userID, Site: string(site)})
	if err != nil {
		return domain.FluxACredential{}, normalizeError(err)
	}
	return domain.FluxACredential{
		UserID:          row.UserID,
		Site:            domain.FluxASite(row.Site),
		TokenCiphertext: row.TokenCiphertext,
		CreatedAt:       pgTimestamptzValue(row.CreatedAt),
		UpdatedAt:       pgTimestamptzValue(row.UpdatedAt),
	}, nil
}

func mapDBUser(id, username, displayName, email, avatarURL, passwordHash string, createdAt pgtype.Timestamptz) domain.User {
	return domain.User{
		ID:           id,
		Username:     username,
		DisplayName:  displayName,
		Email:        email,
		AvatarURL:    avatarURL,
		PasswordHash: passwordHash,
		CreatedAt:    pgTimestamptzValue(createdAt),
	}
}
