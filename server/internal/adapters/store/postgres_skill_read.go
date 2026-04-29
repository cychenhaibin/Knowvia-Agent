package store

import (
	"context"

	sqldb "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/db"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *PostgresStore) GetSkill(ctx context.Context, userID, skillID string) (domain.Skill, error) {
	row, err := s.queries.GetSkill(ctx, sqldb.GetSkillParams{ID: skillID, UserID: userID})
	if err != nil {
		return domain.Skill{}, normalizeError(err)
	}
	return mapDBSkill(row)
}

func (s *PostgresStore) ListSkills(ctx context.Context, userID string) ([]domain.Skill, error) {
	rows, err := s.queries.ListSkills(ctx, userID)
	if err != nil {
		return nil, err
	}

	skills := make([]domain.Skill, 0, len(rows))
	for _, row := range rows {
		skill, err := mapDBSkillListRow(row)
		if err != nil {
			return nil, err
		}
		skills = append(skills, skill)
	}
	return skills, nil
}

func (s *PostgresStore) GetSkillInstallationRecord(ctx context.Context, userID, installationID string) (domain.SkillInstallationRecord, error) {
	row, err := s.queries.GetSkillInstallationRecord(ctx, sqldb.GetSkillInstallationRecordParams{
		ID:     installationID,
		UserID: userID,
	})
	if err != nil {
		return domain.SkillInstallationRecord{}, normalizeError(err)
	}
	return mapDBSkillInstallationRecord(row)
}

func (s *PostgresStore) GetDefaultSkillInstallationRecord(ctx context.Context, userID, definitionID string) (domain.SkillInstallationRecord, error) {
	row, err := s.queries.GetDefaultSkillInstallationRecord(ctx, sqldb.GetDefaultSkillInstallationRecordParams{
		UserID:       userID,
		DefinitionID: definitionID,
	})
	if err != nil {
		return domain.SkillInstallationRecord{}, normalizeError(err)
	}
	return mapDBDefaultSkillInstallationRecord(row)
}

func (s *PostgresStore) ListSkillInstallations(ctx context.Context, userID string) ([]domain.SkillInstallationRecord, error) {
	rows, err := s.queries.ListSkillInstallations(ctx, userID)
	if err != nil {
		return nil, err
	}

	items := make([]domain.SkillInstallationRecord, 0, len(rows))
	for _, row := range rows {
		item, err := mapDBListedSkillInstallationRecord(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *PostgresStore) ListSkillDefinitions(ctx context.Context, userID string) ([]domain.SkillDefinition, error) {
	rows, err := s.queries.ListSkillDefinitions(ctx, userID)
	if err != nil {
		return nil, err
	}

	items := make([]domain.SkillDefinition, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapDBSkillDefinition(row))
	}
	return items, nil
}

func (s *PostgresStore) GetSkillDefinitionDetails(ctx context.Context, userID, definitionID string) (domain.SkillDefinitionDetails, error) {
	row, err := s.queries.GetSkillDefinition(ctx, sqldb.GetSkillDefinitionParams{
		ID:     definitionID,
		UserID: userID,
	})
	if err != nil {
		return domain.SkillDefinitionDetails{}, normalizeError(err)
	}
	definition := mapDBSkillDefinition(row)

	revisions, err := s.ListSkillRevisions(ctx, userID, definitionID)
	if err != nil {
		return domain.SkillDefinitionDetails{}, err
	}

	rows, err := s.queries.ListDefinitionInstallations(ctx, sqldb.ListDefinitionInstallationsParams{
		UserID:       userID,
		DefinitionID: definitionID,
	})
	if err != nil {
		return domain.SkillDefinitionDetails{}, err
	}

	installations := make([]domain.SkillInstallation, 0, len(rows))
	for _, row := range rows {
		installations = append(installations, mapDBSkillInstallation(row))
	}

	return domain.SkillDefinitionDetails{
		Definition:    definition,
		Revisions:     revisions,
		Installations: installations,
	}, nil
}

func (s *PostgresStore) ListSkillRevisions(ctx context.Context, userID, definitionID string) ([]domain.SkillRevision, error) {
	if _, err := s.queries.EnsureSkillDefinitionExists(ctx, sqldb.EnsureSkillDefinitionExistsParams{
		ID:     definitionID,
		UserID: userID,
	}); err != nil {
		return nil, normalizeError(err)
	}
	rows, err := s.queries.ListSkillRevisions(ctx, sqldb.ListSkillRevisionsParams{
		DefinitionID: definitionID,
		UserID:       userID,
	})
	if err != nil {
		return nil, err
	}

	items := make([]domain.SkillRevision, 0, len(rows))
	for _, row := range rows {
		item, err := mapDBSkillRevision(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}
