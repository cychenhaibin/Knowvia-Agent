package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/persistence"
)

const maxFluxAModelsResponseBytes = 1 << 20

type FluxAModel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type FluxAModelGroup struct {
	ID     string       `json:"id"`
	Name   string       `json:"name"`
	Group  string       `json:"group"`
	Desc   string       `json:"desc,omitempty"`
	Ratio  float64      `json:"ratio,omitempty"`
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

type fluxATokenList struct {
	Items []struct {
		ID    int64  `json:"id"`
		Name  string `json:"name"`
		Group string `json:"group"`
	} `json:"items"`
}

type fluxAToken struct {
	ID    string
	Name  string
	Group string
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

func (s *Service) ListFluxAModels(ctx context.Context, userID string, site FluxASite, group string) ([]FluxAModel, error) {
	if _, err := fluxAProvider(site); err != nil || strings.TrimSpace(userID) == "" || s.fluxACredentials == nil || s.fluxACipher == nil || s.fluxAModels == nil {
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
	fetcher, ok := s.fluxAModels.(FluxAModelsFetcher)
	if !ok {
		return nil, ErrFluxAUnavailable
	}
	return fetcher.Models(ctx, site, accessToken, group)
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
	tokenPayload, err := f.get(ctx, origin, "/api/token/?p=1&size=20", accessToken)
	if err != nil {
		return nil, err
	}
	tokens, err := parseFluxATokens(tokenPayload)
	if err != nil {
		log.Printf("fluxa_model_groups_parse_failure route=/api/token/ classification=tokens_payload_invalid shape=%s", jsonShapeSummary(tokenPayload))
		return nil, ErrFluxAUnavailable
	}
	modelPayload, err := f.get(ctx, origin, "/api/user/self/groups", accessToken)
	if err != nil {
		return nil, err
	}
	groups, err := parseFluxAModelGroups(modelPayload)
	if err != nil {
		log.Printf("fluxa_model_groups_parse_failure route=/api/user/self/groups classification=groups_payload_invalid shape=%s", jsonShapeSummary(modelPayload))
		return nil, ErrFluxAUnavailable
	}
	return mergeFluxATokensWithGroups(tokens, groups), nil
}

func (f *fluxAModelGroupsFetcher) Models(ctx context.Context, site FluxASite, accessToken, group string) ([]FluxAModel, error) {
	origin, err := fluxAOrigin(site, f.paidOrigin, f.freeOrigin)
	if err != nil || f.httpClient == nil || strings.TrimSpace(group) == "" {
		return nil, ErrFluxAUnavailable
	}
	payload, err := f.get(ctx, origin, "/api/user/models?group="+url.QueryEscape(strings.TrimSpace(group)), accessToken)
	if err != nil {
		return nil, err
	}
	var names []string
	if err := json.Unmarshal(payload, &names); err != nil {
		return nil, ErrFluxAUnavailable
	}
	models := make([]FluxAModel, 0, len(names))
	for _, name := range names {
		if name = strings.TrimSpace(name); name != "" {
			models = append(models, FluxAModel{ID: name, Name: name})
		}
	}
	return models, nil
}

func parseFluxATokens(data json.RawMessage) ([]fluxAToken, error) {
	var payload fluxATokenList
	if err := json.Unmarshal(data, &payload); err != nil || payload.Items == nil {
		return nil, errors.New("invalid FluxA tokens payload")
	}
	tokens := make([]fluxAToken, 0, len(payload.Items))
	for _, item := range payload.Items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		if item.ID <= 0 {
			continue
		}
		tokens = append(tokens, fluxAToken{ID: strconv.FormatInt(item.ID, 10), Name: name, Group: strings.TrimSpace(item.Group)})
	}
	return tokens, nil
}

func mergeFluxATokensWithGroups(tokens []fluxAToken, groups []FluxAModelGroup) []FluxAModelGroup {
	metadata := make(map[string]FluxAModelGroup, len(groups))
	for _, group := range groups {
		metadata[group.Name] = group
	}
	result := make([]FluxAModelGroup, 0, len(tokens))
	for _, token := range tokens {
		group := metadata[token.Group]
		group.Name = token.Name
		group.ID = token.ID
		group.Group = token.Group
		result = append(result, group)
	}
	return result
}

func parseFluxAModelGroups(data json.RawMessage) ([]FluxAModelGroup, error) {
	var wrapped struct {
		Groups json.RawMessage `json:"groups"`
	}
	if json.Unmarshal(data, &wrapped) == nil && len(wrapped.Groups) > 0 && string(wrapped.Groups) != "null" {
		return parseFluxAModelGroups(wrapped.Groups)
	}
	var groups []FluxAModelGroup
	if json.Unmarshal(data, &groups) == nil && groups != nil {
		for i := range groups {
			groups[i].Name = strings.TrimSpace(groups[i].Name)
			if groups[i].Name == "" {
				return nil, errors.New("invalid FluxA group")
			}
		}
		return groups, nil
	}
	var grouped map[string]json.RawMessage
	if json.Unmarshal(data, &grouped) != nil {
		return nil, errors.New("invalid FluxA groups payload")
	}
	result := make([]FluxAModelGroup, 0, len(grouped))
	for name, rawModels := range grouped {
		var names []string
		if json.Unmarshal(rawModels, &names) != nil {
			var metadata struct {
				Desc  string  `json:"desc"`
				Ratio float64 `json:"ratio"`
			}
			if json.Unmarshal(rawModels, &metadata) != nil {
				continue
			}
			if strings.TrimSpace(name) != "" {
				result = append(result, FluxAModelGroup{Name: strings.TrimSpace(name), Desc: strings.TrimSpace(metadata.Desc), Ratio: metadata.Ratio})
			}
			continue
		}
		models := make([]FluxAModel, 0, len(names))
		for _, modelName := range names {
			modelName = strings.TrimSpace(modelName)
			if modelName != "" {
				models = append(models, FluxAModel{ID: modelName, Name: modelName})
			}
		}
		if strings.TrimSpace(name) != "" {
			result = append(result, FluxAModelGroup{Name: strings.TrimSpace(name), Models: models})
		}
	}
	if len(result) == 0 {
		return nil, errors.New("invalid FluxA groups payload")
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func (f *fluxAModelGroupsFetcher) get(ctx context.Context, origin, path, accessToken string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+path, nil)
	if err != nil {
		logFluxAModelGroupsUpstreamFailure(path, 0, "request_error")
		return nil, ErrFluxAUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	resp, err := f.httpClient.Do(req)
	if err != nil {
		logFluxAModelGroupsUpstreamFailure(path, 0, "transport_error")
		return nil, ErrFluxAUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		logFluxAModelGroupsUpstreamFailure(path, resp.StatusCode, "reauthentication_required")
		return nil, ErrFluxAReauthenticationRequired
	}
	if resp.StatusCode != http.StatusOK {
		logFluxAModelGroupsUpstreamFailure(path, resp.StatusCode, "http_status")
		return nil, ErrFluxAUnavailable
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFluxAModelsResponseBytes+1))
	if err != nil || len(body) > maxFluxAModelsResponseBytes {
		logFluxAModelGroupsUpstreamFailure(path, resp.StatusCode, "body_read_error")
		return nil, ErrFluxAUnavailable
	}
	var envelope fluxAResponseEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		logFluxAModelGroupsUpstreamFailure(path, resp.StatusCode, "invalid_json")
		return nil, ErrFluxAUnavailable
	}
	if !envelope.Success || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		classification := "unsuccessful_envelope"
		if envelope.Success {
			classification = "empty_data"
		}
		logFluxAModelGroupsUpstreamFailure(path, resp.StatusCode, classification)
		return nil, ErrFluxAUnavailable
	}
	return envelope.Data, nil
}

func logFluxAModelGroupsUpstreamFailure(path string, status int, classification string) {
	log.Printf("fluxa_model_groups_upstream_failure route=%s status=%d classification=%s", path, status, classification)
}

func jsonShapeSummary(raw json.RawMessage) string {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return "invalid"
	}
	switch typed := value.(type) {
	case []any:
		return "array:length=" + strconv.Itoa(len(typed))
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key, nested := range typed {
			shape := "value"
			switch nested.(type) {
			case []any:
				items := nested.([]any)
				shape = "array:length=" + strconv.Itoa(len(items))
				if len(items) > 0 {
					if item, ok := items[0].(map[string]any); ok {
						itemKeys := make([]string, 0, len(item))
						for itemKey := range item {
							itemKeys = append(itemKeys, itemKey)
						}
						sort.Strings(itemKeys)
						shape += ":item_keys=" + strings.Join(itemKeys, ",")
					}
				}
			case map[string]any:
				itemKeys := make([]string, 0, len(nested.(map[string]any)))
				for itemKey := range nested.(map[string]any) {
					itemKeys = append(itemKeys, itemKey)
				}
				sort.Strings(itemKeys)
				shape = "object:keys=" + strings.Join(itemKeys, ",")
			}
			keys = append(keys, key+":"+shape)
		}
		sort.Strings(keys)
		return "object:keys=" + strings.Join(keys, ",")
	default:
		return "scalar"
	}
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
	var grouped map[string]json.RawMessage
	if err := json.Unmarshal(data, &grouped); err == nil {
		items = make([]fluxAUpstreamModel, 0)
		for group, rawItems := range grouped {
			var values []string
			if err := json.Unmarshal(rawItems, &values); err != nil {
				continue
			}
			for _, value := range values {
				value = strings.TrimSpace(value)
				if value == "" {
					continue
				}
				items = append(items, fluxAUpstreamModel{ID: value, Name: value, Group: json.RawMessage(strconv.Quote(group))})
			}
		}
		if len(items) > 0 {
			return validateFluxAModels(items)
		}
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
