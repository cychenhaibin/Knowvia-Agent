package httpapi

import (
	"errors"
	"net/http"

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
