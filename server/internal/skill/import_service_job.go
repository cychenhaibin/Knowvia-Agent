package skill

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/persistence"
)

func mapImportError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, persistence.ErrConflict) {
		return ErrSkillConflict
	}
	if strings.Contains(err.Error(), "github archive download failed") {
		return &GitHubArchiveDownloadError{Err: err}
	}
	return err
}

func newSkillImportJob(userID string, source domain.SkillSource, request any) domain.SkillImportJob {
	now := time.Now().UTC()
	requestJSON := "{}"
	if request != nil {
		if raw, err := json.Marshal(request); err == nil {
			requestJSON = string(raw)
		}
	}
	return domain.SkillImportJob{
		ID:          uuid.NewString(),
		UserID:      userID,
		Source:      source,
		Status:      domain.SkillImportPending,
		RequestJSON: requestJSON,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func (s *ImportService) failSkillImportJob(ctx context.Context, job *domain.SkillImportJob, cause error) {
	if job == nil {
		return
	}
	now := time.Now().UTC()
	job.Status = domain.SkillImportFailed
	job.ErrorMessage = cause.Error()
	job.UpdatedAt = now
	job.CompletedAt = &now
	_ = s.jobs.UpdateSkillImportJob(ctx, *job)
}
