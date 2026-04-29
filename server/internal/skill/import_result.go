package skill

import "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"

type ImportResult struct {
	Job      domain.SkillImportJob
	Artifact *domain.SkillArtifact
}
