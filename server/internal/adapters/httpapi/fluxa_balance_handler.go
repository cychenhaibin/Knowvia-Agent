package httpapi

import (
	"errors"
	"net/http"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/auth"
)

type fluxABalanceResponse struct {
	Quota                      float64 `json:"quota"`
	QuotaPerUnit               float64 `json:"quotaPerUnit"`
	QuotaDisplayType           string  `json:"quotaDisplayType"`
	USDExchangeRate            float64 `json:"usdExchangeRate"`
	CustomCurrencySymbol       string  `json:"customCurrencySymbol"`
	CustomCurrencyExchangeRate float64 `json:"customCurrencyExchangeRate"`
}

func (h *Handler) fluxABalance(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	balance, err := h.authService.GetFluxABalance(r.Context(), user.ID, auth.FluxASitePaid)
	if errors.Is(err, auth.ErrFluxANotConnected) {
		balance, err = h.authService.GetFluxABalance(r.Context(), user.ID, auth.FluxASiteFree)
	}
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrFluxANotConnected):
			writeError(w, http.StatusForbidden, "FluxA account is not connected")
		case errors.Is(err, auth.ErrFluxAReauthenticationRequired):
			writeUnauthorized(w, "sign in to FluxA again")
		case errors.Is(err, auth.ErrFluxAUnavailable):
			writeServiceUnavailable(w, "FluxA balance service is unavailable")
		default:
			writeInternalError(w, "FluxA balance is unavailable")
		}
		return
	}
	writeJSON(w, http.StatusOK, fluxABalanceResponse{
		Quota:                      balance.Quota,
		QuotaPerUnit:               balance.QuotaPerUnit,
		QuotaDisplayType:           balance.DisplayType,
		USDExchangeRate:            balance.USDExchangeRate,
		CustomCurrencySymbol:       balance.CustomCurrencySymbol,
		CustomCurrencyExchangeRate: balance.CustomCurrencyExchangeRate,
	})
}
