package app

import (
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/config"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/mirror"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/run"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/tools"
)

func bootstrapServices(
	cfg config.Config,
	closeFn func(),
	authStore authBackend,
	chatStore chatBackend,
	knowledgeStore knowledgeBackend,
	mirrorStore mirror.MirrorStore,
	runStore runBackend,
	skillServiceStore skillServiceBackend,
	skillImportStore skillImportBackend,
	searchStore tools.KnowledgeSearchStore,
) *Services {
	clients := buildExternalClients(cfg)
	authService := buildAuthService(authStore, cfg)
	mirrorService := buildMirrorService(mirrorStore, clients)
	knowledgeService, knowledgeSearchTool := buildKnowledgeModule(knowledgeStore, searchStore, clients)
	chatService := buildChatService(chatStore, clients, knowledgeSearchTool)
	skillService, skillImportService := buildSkillServices(skillServiceStore, skillImportStore, mirrorService)
	broker := run.NewEventBroker()
	runService := buildRunService(runStore, clients, broker, knowledgeSearchTool)

	return &Services{
		closeFn:             closeFn,
		Auth:                authService,
		Chat:                chatService,
		Knowledge:           knowledgeService,
		Mirror:              mirrorService,
		Run:                 runService,
		Skill:               skillService,
		SkillImport:         skillImportService,
		Broker:              broker,
		KnowledgeSearchTool: knowledgeSearchTool,
	}
}
