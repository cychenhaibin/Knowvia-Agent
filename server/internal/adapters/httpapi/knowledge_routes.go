package httpapi

import "github.com/go-chi/chi/v5"

func (h *Handler) registerKnowledgeRoutes(r chi.Router) {
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
}
