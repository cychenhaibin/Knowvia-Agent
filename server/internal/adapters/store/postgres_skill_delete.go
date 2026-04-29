package store

import (
	"context"

	sqldb "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/db"
)

func (s *PostgresStore) DeleteSkill(ctx context.Context, userID, skillID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	queries := s.queries.WithTx(tx)

	deleteContext, err := queries.GetDeletedSkillContext(ctx, sqldb.GetDeletedSkillContextParams{
		ID:     skillID,
		UserID: userID,
	})
	if err != nil {
		return normalizeError(err)
	}
	rowsAffected, err := queries.DeleteSkillInstallation(ctx, sqldb.DeleteSkillInstallationParams{
		ID:     skillID,
		UserID: userID,
	})
	if err != nil {
		return normalizeError(err)
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	if deleteContext.IsDefault {
		if _, err := queries.PromoteLatestSkillInstallation(ctx, sqldb.PromoteLatestSkillInstallationParams{
			UserID:       userID,
			DefinitionID: deleteContext.DefinitionID,
		}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
