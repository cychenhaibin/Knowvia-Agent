package httpapi

import (
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

type skillInstallationListDTO struct {
	Items []skillInstallationRecordDTO `json:"items"`
}

type skillInstallationDTO struct {
	ID                string    `json:"id"`
	UserID            string    `json:"userId"`
	DefinitionID      string    `json:"definitionId"`
	CurrentRevisionID string    `json:"currentRevisionId"`
	Name              string    `json:"name"`
	IsDefault         bool      `json:"isDefault"`
	Enabled           bool      `json:"enabled"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type skillInstallationRecordDTO struct {
	Installation    skillInstallationDTO `json:"installation"`
	Definition      skillDefinitionDTO   `json:"definition"`
	CurrentRevision skillRevisionDTO     `json:"currentRevision"`
}

type skillDefinitionDetailsDTO struct {
	Definition    skillDefinitionDTO     `json:"definition"`
	Revisions     []skillRevisionDTO     `json:"revisions"`
	Installations []skillInstallationDTO `json:"installations"`
}

func mapSkillInstallation(installation domain.SkillInstallation) skillInstallationDTO {
	return skillInstallationDTO{
		ID:                installation.ID,
		UserID:            installation.UserID,
		DefinitionID:      installation.DefinitionID,
		CurrentRevisionID: installation.CurrentRevisionID,
		Name:              installation.Name,
		IsDefault:         installation.IsDefault,
		Enabled:           installation.Enabled,
		CreatedAt:         installation.CreatedAt,
		UpdatedAt:         installation.UpdatedAt,
	}
}

func mapSkillInstallations(installations []domain.SkillInstallation) []skillInstallationDTO {
	items := make([]skillInstallationDTO, 0, len(installations))
	for _, installation := range installations {
		items = append(items, mapSkillInstallation(installation))
	}
	return items
}

func mapSkillInstallationRecord(record domain.SkillInstallationRecord) skillInstallationRecordDTO {
	return skillInstallationRecordDTO{
		Installation:    mapSkillInstallation(record.Installation),
		Definition:      mapSkillDefinition(record.Definition),
		CurrentRevision: mapSkillRevision(record.CurrentRevision),
	}
}

func mapSkillInstallationRecords(records []domain.SkillInstallationRecord) []skillInstallationRecordDTO {
	items := make([]skillInstallationRecordDTO, 0, len(records))
	for _, record := range records {
		items = append(items, mapSkillInstallationRecord(record))
	}
	return items
}

func mapSkillDefinitionDetails(details domain.SkillDefinitionDetails) skillDefinitionDetailsDTO {
	return skillDefinitionDetailsDTO{
		Definition:    mapSkillDefinition(details.Definition),
		Revisions:     mapSkillRevisions(details.Revisions),
		Installations: mapSkillInstallations(details.Installations),
	}
}
