package knowledge

import "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"

type RepoSummary struct {
	Name          string
	DocumentCount int
}

type ListConnectionDocumentsResult struct {
	Connection    domain.KnowledgeConnection
	RepoCount     int
	DocumentCount int
	ChunkCount    int
	Repos         []RepoSummary
	Documents     any
}
