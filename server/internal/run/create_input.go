package run

import "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"

type CreateRunInput struct {
	Title                  string
	Goal                   string
	Kind                   domain.RunKind
	SourceURL              string
	Mode                   domain.RunMode
	KnowledgeConnectionIDs []string
	SkillInstallationID    string
	SkillDefinitionID      string
}
