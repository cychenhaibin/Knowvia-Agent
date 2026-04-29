package app

import (
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/mirror"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skill"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillimport"
)

type skillServiceBackend interface {
	skill.SkillMutationStore
	skill.SkillInstallationStore
	skill.SkillRecordStore
	skill.SkillDefinitionStore
	skill.SkillImportJobStore
	skill.SkillSelectionStore
	skill.ImportArtifactStore
}

type skillImportBackend interface {
	skill.ImportDefinitionStore
	skill.SkillInstallationStore
	skill.ImportArtifactStore
	skill.ImportJobLifecycleStore
	skill.SkillRecordStore
}

func buildSkillServices(
	serviceStore skillServiceBackend,
	importStore skillImportBackend,
	mirrorService *mirror.Service,
) (*skill.Service, *skill.ImportService) {
	skillService := skill.NewService(skill.ServiceDeps{
		Mutations:     serviceStore,
		Installations: serviceStore,
		Records:       serviceStore,
		Definitions:   serviceStore,
		ImportJobs:    serviceStore,
		Artifacts:     serviceStore,
		Selection:     serviceStore,
	}, mirrorService)
	skillImporter := skillimport.NewService(nil)
	skillImportService := skill.NewImportService(skill.ImportDeps{
		Definitions:   importStore,
		Installations: importStore,
		Artifacts:     importStore,
		Jobs:          importStore,
		Records:       importStore,
	}, skillImporter, mirrorService)
	return skillService, skillImportService
}
