package httpapi

import "github.com/go-chi/chi/v5"

func (h *Handler) registerChatRoutes(r chi.Router) {
	r.Get("/chat-models", h.requireAuth(h.listChatModels))
	r.Post("/chat-models", h.requireAuth(h.createChatModel))
	r.Put("/chat-models/{modelID}", h.requireAuth(h.updateChatModel))
	r.Post("/chat-models/{modelID}/select", h.requireAuth(h.selectChatModel))
	r.Delete("/chat-models/{modelID}", h.requireAuth(h.deleteChatModel))

	r.Get("/chat/stream", h.requireAuth(h.chatStream))
	r.Post("/chat/sessions", h.requireAuth(h.createChatSession))
	r.Get("/chat/sessions", h.requireAuth(h.listChatSessions))
	r.Patch("/chat/sessions/{sessionID}", h.requireAuth(h.updateChatSession))
	r.Delete("/chat/sessions/{sessionID}", h.requireAuth(h.deleteChatSession))
	r.Get("/chat/sessions/{sessionID}/messages", h.requireAuth(h.listChatMessages))
}
