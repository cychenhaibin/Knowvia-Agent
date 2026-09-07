package httpapi

import "github.com/go-chi/chi/v5"

func (h *Handler) registerFluxABalanceRoutes(r chi.Router) {
	r.Get("/fluxa/balance", h.requireAuth(h.fluxABalance))
}
