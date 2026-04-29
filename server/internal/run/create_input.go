package run

import "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"

type CreateRunInput struct {
	Title                  string
	Goal                   string
	Mode                   domain.RunMode
	KnowledgeConnectionIDs []string
	SkillInstallationID    string
	SkillDefinitionID      string
}
