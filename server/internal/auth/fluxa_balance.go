package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strings"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/persistence"
)

const maxFluxABalanceResponseBytes = 1 << 20

type FluxABalance struct {
	Quota                      float64 `json:"quota"`
	QuotaPerUnit               float64 `json:"quota_per_unit"`
	DisplayType                string  `json:"quota_display_type"`
	USDExchangeRate            float64 `json:"usd_exchange_rate"`
	CustomCurrencySymbol       string  `json:"custom_currency_symbol"`
	CustomCurrencyExchangeRate float64 `json:"custom_currency_exchange_rate"`
	Group                      string  `json:"group"`
}

type fluxABalanceFetcher struct {
	paidOrigin string
	freeOrigin string
	httpClient *http.Client
}

type fluxABalanceStatusData struct {
	QuotaPerUnit               *float64 `json:"quota_per_unit"`
	DisplayType                string   `json:"quota_display_type"`
	USDExchangeRate            *float64 `json:"usd_exchange_rate"`
	CustomCurrencySymbol       string   `json:"custom_currency_symbol"`
	CustomCurrencyExchangeRate *float64 `json:"custom_currency_exchange_rate"`
}

type fluxABalanceSelfData struct {
	Quota *float64        `json:"quota"`
	Group json.RawMessage `json:"group"`
}

func NewFluxABalanceFetcher(paidOrigin, freeOrigin string) FluxABalanceFetcher {
	paidOrigin, _ = normalizeFluxAOrigin(paidOrigin)
	freeOrigin, _ = normalizeFluxAOrigin(freeOrigin)
	return newFluxABalanceFetcher(paidOrigin, freeOrigin, newFluxAHTTPClient())
}

func newFluxABalanceFetcher(paidOrigin, freeOrigin string, httpClient *http.Client) *fluxABalanceFetcher {
	return &fluxABalanceFetcher{paidOrigin: paidOrigin, freeOrigin: freeOrigin, httpClient: httpClient}
}

func (s *Service) GetFluxABalance(ctx context.Context, userID string, site FluxASite) (FluxABalance, error) {
	if _, err := fluxAProvider(site); err != nil {
		return FluxABalance{}, err
	}
	if strings.TrimSpace(userID) == "" || s.fluxACredentials == nil {
		return FluxABalance{}, ErrFluxANotConnected
	}
	if s.fluxACipher == nil || s.fluxABalance == nil {
		return FluxABalance{}, ErrFluxAUnavailable
	}
	credential, err := s.fluxACredentials.GetFluxACredential(ctx, userID, site)
	if errors.Is(err, persistence.ErrNotFound) {
		return FluxABalance{}, ErrFluxANotConnected
	}
	if err != nil || credential.UserID != userID || credential.Site != site || strings.TrimSpace(credential.TokenCiphertext) == "" {
		return FluxABalance{}, ErrFluxAUnavailable
	}
	token, err := s.fluxACipher.Decrypt(credential.TokenCiphertext, fluxACredentialAdditionalData(userID, site))
	if err != nil || strings.TrimSpace(token) == "" {
		return FluxABalance{}, ErrFluxAUnavailable
	}
	return s.fluxABalance.Balance(ctx, site, token)
}

func (f *fluxABalanceFetcher) Balance(ctx context.Context, site FluxASite, accessToken string) (FluxABalance, error) {
	origin, err := fluxAOrigin(site, f.paidOrigin, f.freeOrigin)
	if err != nil || f.httpClient == nil {
		return FluxABalance{}, ErrFluxAUnavailable
	}
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return FluxABalance{}, ErrFluxAReauthenticationRequired
	}
	statusData, err := f.get(ctx, origin+"/api/status", "")
	if err != nil {
		return FluxABalance{}, err
	}
	var status fluxABalanceStatusData
	if err := decodeFluxABalanceData(statusData, &status); err != nil || !validFluxABalanceStatus(status) {
		return FluxABalance{}, ErrFluxAUnavailable
	}
	selfData, err := f.get(ctx, origin+"/api/user/self", accessToken)
	if err != nil {
		return FluxABalance{}, err
	}
	var self fluxABalanceSelfData
	if err := decodeFluxABalanceData(selfData, &self); err != nil || self.Quota == nil || !finite(*self.Quota) {
		return FluxABalance{}, ErrFluxAUnavailable
	}
	balance := FluxABalance{Quota: *self.Quota, QuotaPerUnit: *status.QuotaPerUnit, DisplayType: strings.TrimSpace(status.DisplayType), CustomCurrencySymbol: strings.TrimSpace(status.CustomCurrencySymbol)}
	if status.USDExchangeRate != nil {
		balance.USDExchangeRate = *status.USDExchangeRate
	}
	if status.CustomCurrencyExchangeRate != nil {
		balance.CustomCurrencyExchangeRate = *status.CustomCurrencyExchangeRate
	}
	var group string
	if len(self.Group) > 0 && json.Unmarshal(self.Group, &group) == nil {
		if group = strings.TrimSpace(group); group != "" {
			balance.Group = group
		}
	}
	return balance, nil
}

func (f *fluxABalanceFetcher) get(ctx context.Context, endpoint, accessToken string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, ErrFluxAUnavailable
	}
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, ErrFluxAUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, ErrFluxAReauthenticationRequired
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ErrFluxAUnavailable
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFluxABalanceResponseBytes+1))
	if err != nil || len(body) > maxFluxABalanceResponseBytes {
		return nil, ErrFluxAUnavailable
	}
	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || !envelope.Success || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil, ErrFluxAUnavailable
	}
	return envelope.Data, nil
}

func decodeFluxABalanceData(raw json.RawMessage, target any) error {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON")
	}
	return nil
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func validFluxABalanceStatus(status fluxABalanceStatusData) bool {
	if status.QuotaPerUnit == nil || !finite(*status.QuotaPerUnit) || *status.QuotaPerUnit <= 0 {
		return false
	}
	switch strings.TrimSpace(status.DisplayType) {
	case "USD", "TOKENS":
		return true
	case "CNY":
		return status.USDExchangeRate != nil && finite(*status.USDExchangeRate)
	case "CUSTOM":
		return status.CustomCurrencyExchangeRate != nil && finite(*status.CustomCurrencyExchangeRate)
	default:
		return false
	}
}
