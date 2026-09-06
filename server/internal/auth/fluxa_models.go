package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/persistence"
)

const maxFluxAModelsResponseBytes = 1 << 20

type FluxAModel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type FluxAModelGroup struct {
	Name   string       `json:"name"`
	Models []FluxAModel `json:"models"`
}

type fluxAModelGroupsFetcher struct {
	paidOrigin string
	freeOrigin string
	httpClient *http.Client
}

type fluxAResponseEnvelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
}

type fluxAAccountGroups struct {
	Group  json.RawMessage `json:"group"`
	Groups json.RawMessage `json:"groups"`
}

type fluxAUpstreamModel struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Group  json.RawMessage `json:"group"`
	Groups json.RawMessage `json:"groups"`
}

func NewFluxAModelGroupsFetcher(paidOrigin, freeOrigin string) FluxAModelGroupsFetcher {
	return newFluxAModelGroupsFetcher(paidOrigin, freeOrigin, newFluxAHTTPClient())
}

func newFluxAModelGroupsFetcher(paidOrigin, freeOrigin string, httpClient *http.Client) *fluxAModelGroupsFetcher {
	paidOrigin, _ = normalizeFluxAOrigin(paidOrigin)
	freeOrigin, _ = normalizeFluxAOrigin(freeOrigin)
	return &fluxAModelGroupsFetcher{
		paidOrigin: paidOrigin,
		freeOrigin: freeOrigin,
		httpClient: httpClient,
	}
}

func (s *Service) ListFluxAModelGroups(ctx context.Context, userID string, site FluxASite) ([]FluxAModelGroup, error) {
	if _, err := fluxAProvider(site); err != nil {
		return nil, err
	}
	if strings.TrimSpace(userID) == "" || s.fluxACredentials == nil {
		return nil, ErrFluxANotConnected
	}
	if s.fluxACipher == nil || s.fluxAModels == nil {
		return nil, ErrFluxAUnavailable
	}
	credential, err := s.fluxACredentials.GetFluxACredential(ctx, userID, site)
	if errors.Is(err, persistence.ErrNotFound) {
		return nil, ErrFluxANotConnected
	}
	if err != nil || credential.UserID != userID || credential.Site != site || strings.TrimSpace(credential.TokenCiphertext) == "" {
		return nil, ErrFluxAUnavailable
	}
	accessToken, err := s.fluxACipher.Decrypt(credential.TokenCiphertext, fluxACredentialAdditionalData(userID, site))
	if err != nil || strings.TrimSpace(accessToken) == "" {
		return nil, ErrFluxAUnavailable
	}
	return s.fluxAModels.List(ctx, site, accessToken)
}

func (f *fluxAModelGroupsFetcher) List(ctx context.Context, site FluxASite, accessToken string) ([]FluxAModelGroup, error) {
	origin, err := fluxAOrigin(site, f.paidOrigin, f.freeOrigin)
	if err != nil || f.httpClient == nil {
		return nil, ErrFluxAUnavailable
	}
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return nil, ErrFluxAReauthenticationRequired
	}
	accountPayload, err := f.get(ctx, origin, "/api/user/self", accessToken)
	if err != nil {
		return nil, err
	}
	accountGroups, err := parseFluxAAccountGroups(accountPayload)
	if err != nil {
		return nil, ErrFluxAUnavailable
	}
	modelPayload, err := f.get(ctx, origin, "/api/models", accessToken)
	if err != nil {
		return nil, err
	}
	models, err := parseFluxAModels(modelPayload)
	if err != nil {
		return nil, ErrFluxAUnavailable
	}
	return normalizeFluxAModelGroups(accountGroups, models), nil
}

func (f *fluxAModelGroupsFetcher) get(ctx context.Context, origin, path, accessToken string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+path, nil)
	if err != nil {
		return nil, ErrFluxAUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
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
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFluxAModelsResponseBytes+1))
	if err != nil || len(body) > maxFluxAModelsResponseBytes {
		return nil, ErrFluxAUnavailable
	}
	var envelope fluxAResponseEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, ErrFluxAUnavailable
	}
	if !envelope.Success || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil, ErrFluxAUnavailable
	}
	return envelope.Data, nil
}

func parseFluxAAccountGroups(data json.RawMessage) ([]string, error) {
	var payload fluxAAccountGroups
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	groups, err := parseFluxAGroupValue(payload.Groups)
	if err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		groups, err = parseFluxAGroupValue(payload.Group)
		if err != nil {
			return nil, err
		}
	}
	return uniqueFluxAStrings(groups), nil
}

func parseFluxAModels(data json.RawMessage) ([]fluxAUpstreamModel, error) {
	var items []fluxAUpstreamModel
	if err := json.Unmarshal(data, &items); err == nil {
		if string(data) == "null" {
			return nil, errors.New("invalid FluxA models payload")
		}
		return validateFluxAModels(items)
	}
	var wrapped struct {
		Models json.RawMessage `json:"models"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil || len(wrapped.Models) == 0 {
		return nil, errors.New("invalid FluxA models payload")
	}
	if string(wrapped.Models) == "null" {
		return nil, errors.New("invalid FluxA models payload")
	}
	if err := json.Unmarshal(wrapped.Models, &items); err != nil {
		return nil, err
	}
	return validateFluxAModels(items)
}

func validateFluxAModels(models []fluxAUpstreamModel) ([]fluxAUpstreamModel, error) {
	for _, model := range models {
		if strings.TrimSpace(model.ID) == "" || strings.TrimSpace(model.Name) == "" {
			return nil, errors.New("invalid FluxA model")
		}
		if _, err := parseFluxAGroupValue(model.Group); err != nil {
			return nil, err
		}
		if _, err := parseFluxAGroupValue(model.Groups); err != nil {
			return nil, err
		}
	}
	return models, nil
}

func parseFluxAGroupValue(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return []string{single}, nil
	}
	var groups []string
	if err := json.Unmarshal(raw, &groups); err != nil {
		return nil, err
	}
	return groups, nil
}

func normalizeFluxAModelGroups(accountGroups []string, models []fluxAUpstreamModel) []FluxAModelGroup {
	defaultGroup := "default"
	if len(accountGroups) > 0 {
		defaultGroup = accountGroups[0]
	}
	groupModels := make(map[string]map[string]FluxAModel, len(accountGroups)+1)
	for _, group := range accountGroups {
		groupModels[group] = map[string]FluxAModel{}
	}
	if len(groupModels) == 0 {
		groupModels[defaultGroup] = map[string]FluxAModel{}
	}
	for _, upstream := range models {
		model := FluxAModel{ID: strings.TrimSpace(upstream.ID), Name: strings.TrimSpace(upstream.Name)}
		if model.ID == "" || model.Name == "" {
			continue
		}
		groups, err := parseFluxAGroupValue(upstream.Groups)
		if err != nil {
			continue
		}
		if len(groups) == 0 {
			groups, err = parseFluxAGroupValue(upstream.Group)
			if err != nil {
				continue
			}
		}
		groups = uniqueFluxAStrings(groups)
		if len(groups) == 0 {
			groups = []string{defaultGroup}
		}
		for _, group := range groups {
			if _, ok := groupModels[group]; !ok {
				groupModels[group] = map[string]FluxAModel{}
			}
			groupModels[group][model.ID+"\x00"+model.Name] = model
		}
	}
	groups := make([]string, 0, len(groupModels))
	for group := range groupModels {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	result := make([]FluxAModelGroup, 0, len(groups))
	for _, group := range groups {
		models := make([]FluxAModel, 0, len(groupModels[group]))
		for _, model := range groupModels[group] {
			models = append(models, model)
		}
		sort.Slice(models, func(i, j int) bool {
			if models[i].Name == models[j].Name {
				return models[i].ID < models[j].ID
			}
			return models[i].Name < models[j].Name
		})
		result = append(result, FluxAModelGroup{Name: group, Models: models})
	}
	return result
}

func uniqueFluxAStrings(values []string) []string {
	unique := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}
