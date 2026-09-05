package httpapi

import "github.com/go-chi/chi/v5"

func (h *Handler) registerAuthRoutes(r chi.Router) {
	r.Post("/auth/login", h.login)
	r.Post("/auth/google", h.loginWithGoogle)
	r.Post("/auth/microsoft", h.loginWithMicrosoft)
	r.Post("/auth/fluxa", h.loginWithFluxA)
	r.Post("/auth/refresh", h.refresh)
	r.Post("/auth/logout", h.logout)
	r.Get("/me", h.requireAuth(h.me))
}
