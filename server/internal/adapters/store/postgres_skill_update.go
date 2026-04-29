package store

import (
	"context"
	"errors"
	"strings"

	sqldb "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/db"
	"github.com/jackc/pgx/v5"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillruntime"
)

func (s *PostgresStore) UpdateSkill(ctx context.Context, skill domain.Skill) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	manifestJSON, err := skillruntime.EncodeRevisionManifest(skill)
	if err != nil {
		return err
	}
	queries := s.queries.WithTx(tx)

	rowsAffected, err := queries.UpdateSkillDefinition(ctx, sqldb.UpdateSkillDefinitionParams{
		ID:        skill.DefinitionID,
		Slug:      skill.Slug,
		Kind:      string(skill.Kind),
		Source:    string(skill.Source),
		RepoUrl:   skill.RepoURL,
		UpdatedAt: pgTimestamptz(skill.UpdatedAt),
		UserID:    skill.UserID,
	})
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	if _, err := queries.CreateSkillRevision(ctx, sqldb.CreateSkillRevisionParams{
		ID:           skill.RevisionID,
		DefinitionID: skill.DefinitionID,
		Version:      int32(skill.Version),
		Title:        skill.Title,
		Description:  skill.Description,
		Prompt:       skill.Prompt,
		Mode:         skill.Mode,
		ManifestJson: manifestJSON,
		CreatedAt:    pgTimestamptz(skill.UpdatedAt),
	}); err != nil {
		return err
	}
	rowsAffected, err = queries.UpdateSkillInstallationRevision(ctx, sqldb.UpdateSkillInstallationRevisionParams{
		ID:                skill.ID,
		CurrentRevisionID: skill.RevisionID,
		Enabled:           skill.Enabled,
		UpdatedAt:         pgTimestamptz(skill.UpdatedAt),
		UserID:            skill.UserID,
	})
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) UpdateSkillInstallation(ctx context.Context, installation domain.SkillInstallation) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	queries := s.queries.WithTx(tx)

	updateContext, err := queries.GetInstallationUpdateContext(ctx, sqldb.GetInstallationUpdateContextParams{
		ID:     installation.ID,
		ID_2:   installation.CurrentRevisionID,
		UserID: installation.UserID,
	})
	if err != nil {
		return normalizeError(err)
	}
	definitionID := updateContext.DefinitionID
	revisionTitle := updateContext.Title
	currentDefault := updateContext.IsDefault
	if strings.TrimSpace(installation.Name) == "" {
		installation.Name = revisionTitle
	}
	var promoteSiblingID string
	promoteSibling := false
	if currentDefault && !installation.IsDefault {
		promoteSiblingID, err = queries.SelectPromoteSiblingInstallation(ctx, sqldb.SelectPromoteSiblingInstallationParams{
			UserID:       installation.UserID,
			DefinitionID: definitionID,
			ID:           installation.ID,
		})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil {
			promoteSibling = true
		}
		if !promoteSibling {
			installation.IsDefault = true
		}
	}
	if installation.IsDefault {
		if _, err := queries.ClearOtherDefaultInstallations(ctx, sqldb.ClearOtherDefaultInstallationsParams{
			UserID:       installation.UserID,
			DefinitionID: definitionID,
			ID:           installation.ID,
		}); err != nil {
			return err
		}
	}
	rowsAffected, err := queries.UpdateSkillInstallation(ctx, sqldb.UpdateSkillInstallationParams{
		ID:                installation.ID,
		CurrentRevisionID: installation.CurrentRevisionID,
		Name:              installation.Name,
		IsDefault:         installation.IsDefault,
		Enabled:           installation.Enabled,
		UpdatedAt:         pgTimestamptz(installation.UpdatedAt),
		UserID:            installation.UserID,
	})
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	if promoteSibling {
		if _, err := queries.PromoteSkillInstallationDefault(ctx, promoteSiblingID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
