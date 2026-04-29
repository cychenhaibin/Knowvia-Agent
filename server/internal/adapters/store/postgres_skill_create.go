package store

import (
	"context"
	"strings"

	sqldb "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/db"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillruntime"
)

func (s *PostgresStore) CreateSkill(ctx context.Context, skill domain.Skill) error {
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

	if _, err := queries.CreateSkillDefinition(ctx, sqldb.CreateSkillDefinitionParams{
		ID:        skill.DefinitionID,
		UserID:    skill.UserID,
		Slug:      skill.Slug,
		Kind:      string(skill.Kind),
		Source:    string(skill.Source),
		RepoUrl:   skill.RepoURL,
		CreatedAt: pgTimestamptz(skill.CreatedAt),
		UpdatedAt: pgTimestamptz(skill.UpdatedAt),
	}); err != nil {
		return err
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
	if _, err := queries.CreateSkillInstallation(ctx, sqldb.CreateSkillInstallationParams{
		ID:                skill.ID,
		UserID:            skill.UserID,
		DefinitionID:      skill.DefinitionID,
		CurrentRevisionID: skill.RevisionID,
		Name:              skill.Title,
		IsDefault:         true,
		Enabled:           skill.Enabled,
		CreatedAt:         pgTimestamptz(skill.CreatedAt),
		UpdatedAt:         pgTimestamptz(skill.UpdatedAt),
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) CreateSkillDefinitionRevision(ctx context.Context, definition domain.SkillDefinition, revision domain.SkillRevision) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	manifestJSON := revision.ManifestJSON
	if strings.TrimSpace(manifestJSON) == "" {
		manifestJSON = "{}"
	}
	queries := s.queries.WithTx(tx)
	if _, err := queries.CreateSkillDefinition(ctx, sqldb.CreateSkillDefinitionParams{
		ID:        definition.ID,
		UserID:    definition.UserID,
		Slug:      definition.Slug,
		Kind:      string(definition.Kind),
		Source:    string(definition.Source),
		RepoUrl:   definition.RepoURL,
		CreatedAt: pgTimestamptz(definition.CreatedAt),
		UpdatedAt: pgTimestamptz(definition.UpdatedAt),
	}); err != nil {
		return normalizeError(err)
	}
	if _, err := queries.CreateSkillRevision(ctx, sqldb.CreateSkillRevisionParams{
		ID:           revision.ID,
		DefinitionID: revision.DefinitionID,
		Version:      int32(revision.Version),
		Title:        revision.Title,
		Description:  revision.Description,
		Prompt:       revision.Prompt,
		Mode:         revision.Mode,
		ManifestJson: manifestJSON,
		CreatedAt:    pgTimestamptz(revision.CreatedAt),
	}); err != nil {
		return normalizeError(err)
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) CreateSkillInstallation(ctx context.Context, installation domain.SkillInstallation) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	queries := s.queries.WithTx(tx)

	revisionTitle, err := queries.GetInstallationRevisionTitle(ctx, sqldb.GetInstallationRevisionTitleParams{
		ID:     installation.DefinitionID,
		ID_2:   installation.CurrentRevisionID,
		UserID: installation.UserID,
	})
	if err != nil {
		return normalizeError(err)
	}
	if strings.TrimSpace(installation.Name) == "" {
		installation.Name = revisionTitle
	}
	existingCount, err := queries.CountDefinitionInstallations(ctx, sqldb.CountDefinitionInstallationsParams{
		UserID:       installation.UserID,
		DefinitionID: installation.DefinitionID,
	})
	if err != nil {
		return err
	}
	if existingCount == 0 {
		installation.IsDefault = true
	}
	if installation.IsDefault {
		if _, err := queries.ClearDefinitionDefaultInstallations(ctx, sqldb.ClearDefinitionDefaultInstallationsParams{
			UserID:       installation.UserID,
			DefinitionID: installation.DefinitionID,
		}); err != nil {
			return err
		}
	}
	if _, err := queries.CreateSkillInstallation(ctx, sqldb.CreateSkillInstallationParams{
		ID:                installation.ID,
		UserID:            installation.UserID,
		DefinitionID:      installation.DefinitionID,
		CurrentRevisionID: installation.CurrentRevisionID,
		Name:              installation.Name,
		IsDefault:         installation.IsDefault,
		Enabled:           installation.Enabled,
		CreatedAt:         pgTimestamptz(installation.CreatedAt),
		UpdatedAt:         pgTimestamptz(installation.UpdatedAt),
	}); err != nil {
		return normalizeError(err)
	}
	return tx.Commit(ctx)
}
