package app

import (
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/provider"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/provider/openai"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/provider/pythonproxy"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/config"
)

type externalClients struct {
	llm     provider.ChatClient
	forward interface {
		provider.ForwardChatClient
		provider.KnowledgeRetrieveForwardClient
		provider.EvidenceMergeForwardClient
		provider.ReportForwardClient
		provider.MirrorClient
		provider.KnowledgeSyncClient
	}
}

func buildExternalClients(cfg config.Config) externalClients {
	return externalClients{
		llm:     openai.New(cfg),
		forward: pythonproxy.New(cfg),
	}
}
