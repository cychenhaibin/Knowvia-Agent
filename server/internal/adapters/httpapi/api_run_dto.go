package httpapi

import (
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

type runListDTO struct {
	Items []runDTO `json:"items"`
}

type runDTO struct {
	ID                     string           `json:"id"`
	UserID                 string           `json:"userId"`
	Title                  string           `json:"title"`
	Goal                   string           `json:"goal"`
	RequestedMode          domain.RunMode   `json:"requestedMode"`
	KnowledgeConnectionIDs []string         `json:"knowledgeConnectionIds,omitempty"`
	SkillInstallationID    string           `json:"skillInstallationId,omitempty"`
	SkillSnapshotID        string           `json:"skillSnapshotId,omitempty"`
	EffectiveMode          domain.RunMode   `json:"effectiveMode"`
	Status                 domain.RunStatus `json:"status"`
	LatestArtifactID       string           `json:"latestArtifactId,omitempty"`
	ErrorMessage           string           `json:"errorMessage,omitempty"`
	CreatedAt              time.Time        `json:"createdAt"`
	UpdatedAt              time.Time        `json:"updatedAt"`
}

type runStepDTO struct {
	ID         string            `json:"id"`
	RunID      string            `json:"runId"`
	Kind       domain.StepKind   `json:"kind"`
	Label      string            `json:"label"`
	Status     domain.StepStatus `json:"status"`
	Summary    string            `json:"summary"`
	StartedAt  *time.Time        `json:"startedAt,omitempty"`
	FinishedAt *time.Time        `json:"finishedAt,omitempty"`
	CreatedAt  time.Time         `json:"createdAt"`
}

type runArtifactDTO struct {
	ID              string              `json:"id"`
	RunID           string              `json:"runId"`
	Kind            domain.ArtifactKind `json:"kind"`
	ContentMarkdown string              `json:"contentMarkdown"`
	Version         int                 `json:"version"`
	CreatedAt       time.Time           `json:"createdAt"`
}

type runSourceDTO struct {
	ID           string          `json:"id"`
	RunID        string          `json:"runId"`
	Provider     domain.Provider `json:"provider"`
	ConnectionID string          `json:"connectionId,omitempty"`
	DocumentID   string          `json:"documentId,omitempty"`
	ChunkID      string          `json:"chunkId,omitempty"`
	Title        string          `json:"title"`
	Repo         string          `json:"repo,omitempty"`
	URL          string          `json:"url,omitempty"`
	Snippet      string          `json:"snippet"`
	Score        float64         `json:"score"`
	CreatedAt    time.Time       `json:"createdAt"`
}

type runDetailsDTO struct {
	Run       runDTO           `json:"run"`
	Steps     []runStepDTO     `json:"steps"`
	Artifacts []runArtifactDTO `json:"artifacts"`
	Sources   []runSourceDTO   `json:"sources"`
}

type runEventDTO struct {
	Type      string    `json:"type"`
	RunID     string    `json:"runId"`
	Timestamp time.Time `json:"timestamp"`
	Payload   any       `json:"payload,omitempty"`
}

type runEvidenceDTO struct {
	Provider           domain.Provider `json:"provider"`
	ConnectionID       string          `json:"connectionId,omitempty"`
	DocumentID         string          `json:"documentId,omitempty"`
	ChunkID            string          `json:"chunkId,omitempty"`
	Title              string          `json:"title"`
	Repo               string          `json:"repo,omitempty"`
	URL                string          `json:"url,omitempty"`
	Snippet            string          `json:"snippet"`
	Body               string          `json:"body,omitempty"`
	MatchedAnswerLines []string        `json:"matchedAnswerLines,omitempty"`
	Score              float64         `json:"score"`
}

func mapRun(run domain.Run) runDTO {
	return runDTO{
		ID:                     run.ID,
		UserID:                 run.UserID,
		Title:                  run.Title,
		Goal:                   run.Goal,
		RequestedMode:          run.RequestedMode,
		KnowledgeConnectionIDs: append([]string(nil), run.KnowledgeConnectionIDs...),
		SkillInstallationID:    run.SkillInstallationID,
		SkillSnapshotID:        run.SkillSnapshotID,
		EffectiveMode:          run.EffectiveMode,
		Status:                 run.Status,
		LatestArtifactID:       run.LatestArtifactID,
		ErrorMessage:           run.ErrorMessage,
		CreatedAt:              run.CreatedAt,
		UpdatedAt:              run.UpdatedAt,
	}
}

func mapRuns(runs []domain.Run) []runDTO {
	items := make([]runDTO, 0, len(runs))
	for _, run := range runs {
		items = append(items, mapRun(run))
	}
	return items
}

func mapRunStep(step domain.RunStep) runStepDTO {
	return runStepDTO{
		ID:         step.ID,
		RunID:      step.RunID,
		Kind:       step.Kind,
		Label:      step.Label,
		Status:     step.Status,
		Summary:    step.Summary,
		StartedAt:  step.StartedAt,
		FinishedAt: step.FinishedAt,
		CreatedAt:  step.CreatedAt,
	}
}

func mapRunSteps(steps []domain.RunStep) []runStepDTO {
	items := make([]runStepDTO, 0, len(steps))
	for _, step := range steps {
		items = append(items, mapRunStep(step))
	}
	return items
}

func mapRunArtifact(artifact domain.RunArtifact) runArtifactDTO {
	return runArtifactDTO{
		ID:              artifact.ID,
		RunID:           artifact.RunID,
		Kind:            artifact.Kind,
		ContentMarkdown: artifact.ContentMarkdown,
		Version:         artifact.Version,
		CreatedAt:       artifact.CreatedAt,
	}
}

func mapRunArtifacts(artifacts []domain.RunArtifact) []runArtifactDTO {
	items := make([]runArtifactDTO, 0, len(artifacts))
	for _, artifact := range artifacts {
		items = append(items, mapRunArtifact(artifact))
	}
	return items
}

func mapRunSource(source domain.RunSource) runSourceDTO {
	return runSourceDTO{
		ID:           source.ID,
		RunID:        source.RunID,
		Provider:     source.Provider,
		ConnectionID: source.ConnectionID,
		DocumentID:   source.DocumentID,
		ChunkID:      source.ChunkID,
		Title:        source.Title,
		Repo:         source.Repo,
		URL:          source.URL,
		Snippet:      source.Snippet,
		Score:        source.Score,
		CreatedAt:    source.CreatedAt,
	}
}

func mapRunSources(sources []domain.RunSource) []runSourceDTO {
	items := make([]runSourceDTO, 0, len(sources))
	for _, source := range sources {
		items = append(items, mapRunSource(source))
	}
	return items
}

func mapRunDetails(details domain.RunDetails) runDetailsDTO {
	return runDetailsDTO{
		Run:       mapRun(details.Run),
		Steps:     mapRunSteps(details.Steps),
		Artifacts: mapRunArtifacts(details.Artifacts),
		Sources:   mapRunSources(details.Sources),
	}
}

func mapRunEvent(event domain.RunEvent) runEventDTO {
	return runEventDTO{
		Type:      event.Type,
		RunID:     event.RunID,
		Timestamp: event.Timestamp,
		Payload:   mapRunEventPayload(event.Payload),
	}
}

func mapRunEventPayload(payload any) any {
	switch typed := payload.(type) {
	case domain.Evidence:
		return mapRunEvidence(typed)
	case []domain.Evidence:
		return mapRunEvidences(typed)
	default:
		return payload
	}
}

func mapRunEvidence(evidence domain.Evidence) runEvidenceDTO {
	return runEvidenceDTO{
		Provider:           evidence.Provider,
		ConnectionID:       evidence.ConnectionID,
		DocumentID:         evidence.DocumentID,
		ChunkID:            evidence.ChunkID,
		Title:              evidence.Title,
		Repo:               evidence.Repo,
		URL:                evidence.URL,
		Snippet:            evidence.Snippet,
		Body:               evidence.Body,
		MatchedAnswerLines: append([]string(nil), evidence.MatchedLines...),
		Score:              evidence.Score,
	}
}

func mapRunEvidences(evidences []domain.Evidence) []runEvidenceDTO {
	items := make([]runEvidenceDTO, 0, len(evidences))
	for _, evidence := range evidences {
		items = append(items, mapRunEvidence(evidence))
	}
	return items
}
