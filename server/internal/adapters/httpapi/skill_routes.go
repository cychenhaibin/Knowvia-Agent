package httpapi

import "github.com/go-chi/chi/v5"

func (h *Handler) registerSkillRoutes(r chi.Router) {
	r.Post("/skills", h.requireAuth(h.createSkill))
	r.Get("/skills", h.requireAuth(h.listSkills))
	r.Patch("/skills/{skillID}", h.requireAuth(h.updateSkill))
	r.Delete("/skills/{skillID}", h.requireAuth(h.deleteSkill))
	r.Get("/skill-definitions", h.requireAuth(h.listSkillDefinitions))
	r.Get("/skill-definitions/{definitionID}", h.requireAuth(h.getSkillDefinition))
	r.Get("/skill-definitions/{definitionID}/revisions", h.requireAuth(h.listSkillRevisions))
	r.Get("/skill-installations", h.requireAuth(h.listSkillInstallations))
	r.Get("/skill-installations/{installationID}", h.requireAuth(h.getSkillInstallation))
	r.Post("/skill-installations", h.requireAuth(h.createSkillInstallation))
	r.Patch("/skill-installations/{installationID}", h.requireAuth(h.updateSkillInstallation))
	r.Delete("/skill-installations/{installationID}", h.requireAuth(h.deleteSkillInstallation))
	r.Get("/skill-imports", h.requireAuth(h.listSkillImportJobs))
	r.Get("/skill-imports/{jobID}", h.requireAuth(h.getSkillImportJob))
	r.Post("/skill-imports/github", h.requireAuth(h.importSkillFromGitHub))
	r.Post("/skill-imports/upload", h.requireAuth(h.importSkillFromUpload))
	r.Get("/skill-artifacts/{artifactID}/files", h.requireAuth(h.listSkillArtifactFiles))
	r.Get("/skill-artifacts/{artifactID}/content", h.requireAuth(h.getSkillArtifactFileContent))
}
