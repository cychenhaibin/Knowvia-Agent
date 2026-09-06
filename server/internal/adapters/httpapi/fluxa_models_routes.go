package httpapi

import "github.com/go-chi/chi/v5"

func (h *Handler) registerFluxAModelRoutes(r chi.Router) {
	r.Get("/fluxa/model-groups", h.requireAuth(h.fluxAModelGroups))
}
