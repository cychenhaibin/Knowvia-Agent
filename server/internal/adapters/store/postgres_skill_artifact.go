package store

import (
	"context"

	sqldb "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/db"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *PostgresStore) CreateSkillRuntimeSnapshot(ctx context.Context, snapshot domain.SkillRuntimeSnapshot) error {
	_, err := s.queries.CreateSkillRuntimeSnapshot(ctx, sqldb.CreateSkillRuntimeSnapshotParams{
		ID:              snapshot.ID,
		Scope:           string(snapshot.Scope),
		ScopeID:         snapshot.ScopeID,
		UserID:          snapshot.UserID,
		InstallationID:  snapshot.InstallationID,
		DefinitionID:    snapshot.DefinitionID,
		RevisionID:      snapshot.RevisionID,
		Kind:            string(snapshot.Kind),
		Title:           snapshot.Title,
		Description:     snapshot.Description,
		Mode:            snapshot.Mode,
		Prompt:          snapshot.Prompt,
		RuntimeSpecJson: snapshot.RuntimeSpecJSON,
		CreatedAt:       pgTimestamptz(snapshot.CreatedAt),
	})
	return err
}

func (s *PostgresStore) CreateSkillArtifact(ctx context.Context, artifact domain.SkillArtifact) error {
	_, err := s.queries.CreateSkillArtifact(ctx, sqldb.CreateSkillArtifactParams{
		ID:               artifact.ID,
		UserID:           artifact.UserID,
		DefinitionID:     artifact.DefinitionID,
		RevisionID:       artifact.RevisionID,
		Source:           string(artifact.Source),
		FileName:         artifact.FileName,
		MediaType:        artifact.MediaType,
		SourceUrl:        artifact.SourceURL,
		Sha256:           artifact.SHA256,
		SizeBytes:        artifact.SizeBytes,
		EntryPath:        artifact.EntryPath,
		ManifestPath:     artifact.ManifestPath,
		InstructionsPath: artifact.InstructionsPath,
		ArchiveBytes:     artifact.ArchiveBytes,
		CreatedAt:        pgTimestamptz(artifact.CreatedAt),
	})
	return normalizeError(err)
}

func (s *PostgresStore) GetSkillArtifact(ctx context.Context, userID, artifactID string) (domain.SkillArtifact, error) {
	row, err := s.queries.GetSkillArtifact(ctx, sqldb.GetSkillArtifactParams{ID: artifactID, UserID: userID})
	if err != nil {
		return domain.SkillArtifact{}, normalizeError(err)
	}
	return mapDBSkillArtifact(row), nil
}

func (s *PostgresStore) ReplaceSkillArtifactFiles(ctx context.Context, artifactID string, files []domain.SkillArtifactFile) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	queries := s.queries.WithTx(tx)
	userID, err := queries.GetSkillArtifactUserId(ctx, artifactID)
	if err != nil {
		return normalizeError(err)
	}
	if _, err := queries.DeleteSkillArtifactFiles(ctx, artifactID); err != nil {
		return err
	}
	for _, file := range files {
		if _, err := queries.InsertSkillArtifactFile(ctx, sqldb.InsertSkillArtifactFileParams{
			ID:             file.ID,
			ArtifactID:     artifactID,
			UserID:         userID,
			Path:           file.Path,
			MediaType:      file.MediaType,
			SizeBytes:      file.SizeBytes,
			Sha256:         file.SHA256,
			IsManifest:     file.IsManifest,
			IsInstructions: file.IsInstructions,
			CreatedAt:      pgTimestamptz(file.CreatedAt),
		}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) ListSkillArtifactFiles(ctx context.Context, userID, artifactID string) ([]domain.SkillArtifactFile, error) {
	rows, err := s.queries.ListSkillArtifactFiles(ctx, sqldb.ListSkillArtifactFilesParams{
		ArtifactID: artifactID,
		UserID:     userID,
	})
	if err != nil {
		return nil, err
	}

	items := make([]domain.SkillArtifactFile, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapDBSkillArtifactFile(row))
	}
	if len(items) == 0 {
		if _, err := s.GetSkillArtifact(ctx, userID, artifactID); err != nil {
			return nil, err
		}
	}
	return items, nil
}
