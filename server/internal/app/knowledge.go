package app

import (
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/knowledge"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/tools"
)

type knowledgeBackend interface {
	knowledge.KnowledgeConnectionStore
	knowledge.KnowledgeSyncJobStore
	knowledge.KnowledgeMetadataStore
	knowledge.KnowledgeCorpusStore
}

func buildKnowledgeModule(
	knowledgeStore knowledgeBackend,
	searchStore tools.KnowledgeSearchStore,
	clients externalClients,
) (*knowledge.Service, tools.KnowledgeSearcher) {
	knowledgeService := knowledge.NewService(knowledge.ServiceDeps{
		Connections: knowledgeStore,
		SyncJobs:    knowledgeStore,
		Metadata:    knowledgeStore,
		Corpus:      knowledgeStore,
	}, clients.forward, clients.forward)
	knowledgeSearchTool := tools.NewKnowledgeSearchTool(searchStore, clients.forward)
	return knowledgeService, knowledgeSearchTool
}
