package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/auth"
)

func (h *Handler) fluxAModelGroups(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	groups, err := h.authService.ListFluxAModelGroups(r.Context(), user.ID, auth.FluxASitePaid)
	if errors.Is(err, auth.ErrFluxANotConnected) {
		groups, err = h.authService.ListFluxAModelGroups(r.Context(), user.ID, auth.FluxASiteFree)
	}
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrFluxANotConnected):
			writeError(w, http.StatusForbidden, "FluxA account is not connected")
		case errors.Is(err, auth.ErrFluxAReauthenticationRequired):
			writeUnauthorized(w, "sign in to FluxA again")
		case errors.Is(err, auth.ErrFluxAUnavailable):
			writeServiceUnavailable(w, "FluxA model service is unavailable")
		default:
			writeInternalError(w, "FluxA model groups are unavailable")
		}
		return
	}
	writeJSON(w, http.StatusOK, groups)
}

func (h *Handler) fluxAModels(w http.ResponseWriter, r *http.Request) {
	group := strings.TrimSpace(r.URL.Query().Get("group"))
	if group == "" {
		writeError(w, http.StatusBadRequest, "FluxA model group is required")
		return
	}
	user := currentUser(r.Context())
	models, err := h.authService.ListFluxAModels(r.Context(), user.ID, auth.FluxASitePaid, group)
	if errors.Is(err, auth.ErrFluxANotConnected) {
		models, err = h.authService.ListFluxAModels(r.Context(), user.ID, auth.FluxASiteFree, group)
	}
	if err != nil {
		if errors.Is(err, auth.ErrFluxAReauthenticationRequired) {
			writeUnauthorized(w, "sign in to FluxA again")
		} else {
			writeServiceUnavailable(w, "FluxA model service is unavailable")
		}
		return
	}
	writeJSON(w, http.StatusOK, models)
}
