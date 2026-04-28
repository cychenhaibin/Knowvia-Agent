package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/auth"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/chat"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/knowledge"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/mirror"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/run"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillimport"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/store"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/taskqueue"
)

func NewRouter(
	authService *auth.Service,
	store store.Store,
	runService *run.Service,
	knowledgeService *knowledge.Service,
	mirrorService *mirror.Service,
	chatService *chat.Service,
	skillImporter *skillimport.Service,
	dispatcher taskqueue.Dispatcher,
	broker *run.EventBroker,
) http.Handler {
	h := &Handler{
		authService:      authService,
		store:            store,
		runService:       runService,
		knowledgeService: knowledgeService,
		mirrorService:    mirrorService,
		chatService:      chatService,
		skillImporter:    skillImporter,
		dispatcher:       dispatcher,
		broker:           broker,
	}

	router := chi.NewRouter()
	router.Use(logRequests)
	router.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		writeNotFound(w, "route not found", "route")
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		writeErrorCode(w, http.StatusMethodNotAllowed, errorCodeInvalidRequestBody, "method not allowed")
	})
	router.Route("/v1", func(r chi.Router) {
		r.Post("/auth/login", h.login)
		r.Post("/auth/google", h.loginWithGoogle)
		r.Post("/auth/microsoft", h.loginWithMicrosoft)
		r.Post("/auth/refresh", h.refresh)
		r.Post("/auth/logout", h.logout)
		r.Get("/me", h.requireAuth(h.me))
		r.Get("/chat-models", h.requireAuth(h.listChatModels))
		r.Post("/chat-models", h.requireAuth(h.createChatModel))
		r.Put("/chat-models/{modelID}", h.requireAuth(h.updateChatModel))
		r.Post("/chat-models/{modelID}/select", h.requireAuth(h.selectChatModel))
		r.Delete("/chat-models/{modelID}", h.requireAuth(h.deleteChatModel))

		r.Get("/runs", h.requireAuth(h.listRuns))
		r.Post("/runs", h.requireAuth(h.createRun))
		r.Get("/runs/{runID}", h.requireAuth(h.getRun))
		r.Get("/runs/{runID}/events", h.requireAuth(h.runEvents))
		r.Get("/chat/stream", h.requireAuth(h.chatStream))
		r.Post("/chat/sessions", h.requireAuth(h.createChatSession))
		r.Get("/chat/sessions", h.requireAuth(h.listChatSessions))
		r.Patch("/chat/sessions/{sessionID}", h.requireAuth(h.updateChatSession))
		r.Delete("/chat/sessions/{sessionID}", h.requireAuth(h.deleteChatSession))
		r.Get("/chat/sessions/{sessionID}/messages", h.requireAuth(h.listChatMessages))

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

		r.Post("/knowledge/connections", h.requireAuth(h.createKnowledgeConnection))
		r.Get("/knowledge/connections", h.requireAuth(h.listKnowledgeConnections))
		r.Delete("/knowledge/connections/{connectionID}", h.requireAuth(h.deleteKnowledgeConnection))
		r.Get("/knowledge/connections/{connectionID}/documents", h.requireAuth(h.listConnectionDocuments))
		r.Post("/knowledge/connections/{connectionID}/sync", h.requireAuth(h.triggerSync))
		r.Get("/knowledge/connections/{connectionID}/sync-jobs", h.requireAuth(h.listSyncJobs))

		r.Post("/knowledge/yuque/connections", h.requireAuth(h.createYuqueConnection))
		r.Get("/knowledge/yuque/connections", h.requireAuth(h.listYuqueConnections))
		r.Delete("/knowledge/yuque/connections/{connectionID}", h.requireAuth(h.deleteYuqueConnection))
		r.Get("/knowledge/yuque/connections/{connectionID}/documents", h.requireAuth(h.listConnectionDocuments))
		r.Post("/knowledge/yuque/connections/{connectionID}/sync", h.requireAuth(h.triggerSync))
		r.Get("/knowledge/yuque/connections/{connectionID}/sync-jobs", h.requireAuth(h.listSyncJobs))
	})
	return router
}
