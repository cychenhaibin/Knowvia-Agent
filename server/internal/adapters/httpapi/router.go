package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/auth"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/chat"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/knowledge"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/run"
	skillsvc "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skill"
)

func NewRouter(
	authService *auth.Service,
	runService *run.Service,
	knowledgeService *knowledge.Service,
	chatService *chat.Service,
	skillService *skillsvc.Service,
	skillImporter *skillsvc.ImportService,
) http.Handler {
	h := &Handler{
		authService:       authService,
		runService:        runService,
		knowledgeService:  knowledgeService,
		chatService:       chatService,
		skillService:      skillService,
		skillImporter:     skillImporter,
		fluxALoginLimiter: newDefaultFluxALoginLimiter(),
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
		h.registerAuthRoutes(r)
		h.registerFluxAModelRoutes(r)
		h.registerChatRoutes(r)
		h.registerRunRoutes(r)
		h.registerSkillRoutes(r)
		h.registerKnowledgeRoutes(r)
	})
	return router
}
