package app

import (
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/chat"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/tools"
)

type chatBackend interface {
	chat.ChatModelStore
	chat.SessionStore
	chat.ConversationStore
	chat.PrepareStreamStore
}

func buildChatService(chatStore chatBackend, clients externalClients, knowledgeTool tools.KnowledgeSearcher) *chat.Service {
	return chat.NewService(
		chat.ServiceDeps{
			Models:        chatStore,
			Sessions:      chatStore,
			Conversations: chatStore,
			Prepare:       chatStore,
		},
		knowledgeTool,
		clients.llm,
		clients.forward,
	)
}
