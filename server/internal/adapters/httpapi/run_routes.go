package httpapi

import "github.com/go-chi/chi/v5"

func (h *Handler) registerRunRoutes(r chi.Router) {
	r.Get("/runs", h.requireAuth(h.listRuns))
	r.Post("/runs", h.requireAuth(h.createRun))
	r.Get("/runs/{runID}", h.requireAuth(h.getRun))
	r.Get("/runs/{runID}/events", h.requireAuth(h.runEvents))
}
