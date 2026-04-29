package knowledge

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/provider/feishu"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func (s *Service) CreateConnection(ctx context.Context, input CreateConnectionInput) (domain.KnowledgeConnection, error) {
	now := time.Now().UTC()
	providerName := domain.Provider(strings.TrimSpace(input.Provider))
	connection := domain.KnowledgeConnection{
		ID:          uuid.NewString(),
		UserID:      input.UserID,
		Provider:    providerName,
		Name:        strings.TrimSpace(input.Name),
		SyncEnabled: input.SyncEnabled,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	switch providerName {
	case domain.ProviderYuque:
		var cfg struct {
			Token      string `json:"token"`
			GroupLogin string `json:"groupLogin"`
			Namespace  string `json:"namespace"`
		}
		if err := json.Unmarshal(input.Config, &cfg); err != nil {
			return domain.KnowledgeConnection{}, ErrInvalidRequestBody
		}
		if connection.Name == "" {
			connection.Name = "Yuque Connection"
		}
		connection.Yuque = &domain.KnowledgeConnectionYuqueConfig{
			Token:      strings.TrimSpace(cfg.Token),
			GroupLogin: strings.TrimSpace(cfg.GroupLogin),
			Namespace:  strings.TrimSpace(cfg.Namespace),
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		missingFields := make([]string, 0, 3)
		if connection.Yuque.Token == "" {
			missingFields = append(missingFields, "config.token")
		}
		if connection.Yuque.GroupLogin == "" {
			missingFields = append(missingFields, "config.groupLogin")
		}
		if connection.Yuque.Namespace == "" {
			missingFields = append(missingFields, "config.namespace")
		}
		if len(missingFields) > 0 {
			return domain.KnowledgeConnection{}, &ValidationError{
				Message: "yuque token, groupLogin and namespace are required",
				Fields:  missingFields,
			}
		}
	case domain.ProviderFeishu:
		var cfg struct {
			AppID         string `json:"appId"`
			AppSecret     string `json:"appSecret"`
			EntryType     string `json:"entryType"`
			EntryToken    string `json:"entryToken"`
			DocumentID    string `json:"documentId"`
			DocumentURL   string `json:"documentUrl"`
			DocumentInput string `json:"documentInput"`
		}
		if err := json.Unmarshal(input.Config, &cfg); err != nil {
			return domain.KnowledgeConnection{}, ErrInvalidRequestBody
		}
		if connection.Name == "" {
			connection.Name = "Feishu Document"
		}
		rawEntryHint := strings.TrimSpace(cfg.EntryToken)
		if rawEntryHint == "" {
			rawEntryHint = strings.TrimSpace(cfg.DocumentInput)
		}
		if rawEntryHint == "" {
			rawEntryHint = strings.TrimSpace(cfg.DocumentURL)
		}
		if rawEntryHint == "" {
			rawEntryHint = strings.TrimSpace(cfg.DocumentID)
		}
		connection.Feishu = &domain.KnowledgeConnectionFeishuConfig{
			AppID:      strings.TrimSpace(cfg.AppID),
			AppSecret:  strings.TrimSpace(cfg.AppSecret),
			EntryType:  inferFeishuEntryType(cfg.EntryType, rawEntryHint),
			EntryToken: strings.TrimSpace(cfg.EntryToken),
			CreatedAt:  now,
			UpdatedAt:  now,
		}

		if connection.Feishu.EntryType == "" {
			connection.Feishu.EntryType = "docx"
		}
		if connection.Feishu.EntryToken == "" {
			switch connection.Feishu.EntryType {
			case "wiki_node", "wiki_space":
				connection.Feishu.EntryToken = normalizeFeishuWikiTokenCandidates(cfg.EntryToken, cfg.DocumentInput, cfg.DocumentURL, cfg.DocumentID)
			default:
				connection.Feishu.EntryType = "docx"
				connection.Feishu.EntryToken = normalizeFeishuDocumentIDCandidates(cfg.DocumentID, cfg.DocumentURL, cfg.DocumentInput, cfg.EntryToken)
			}
		}
		missingFields := make([]string, 0, 3)
		if connection.Feishu.AppID == "" {
			missingFields = append(missingFields, "config.appId")
		}
		if connection.Feishu.AppSecret == "" {
			missingFields = append(missingFields, "config.appSecret")
		}
		if connection.Feishu.EntryToken == "" {
			missingFields = append(missingFields, "config.entryToken")
		}
		if len(missingFields) > 0 {
			return domain.KnowledgeConnection{}, &ValidationError{
				Message: "feishu appId, appSecret and entry token are required",
				Fields:  missingFields,
			}
		}
	default:
		return domain.KnowledgeConnection{}, &ValidationError{
			Message: "unsupported knowledge provider",
			Fields:  []string{"provider"},
		}
	}
	if err := s.connectionStore.CreateKnowledgeConnection(ctx, connection); err != nil {
		return domain.KnowledgeConnection{}, err
	}
	return connection, nil
}

func (s *Service) CreateYuqueConnection(ctx context.Context, input CreateYuqueConnectionInput) (domain.KnowledgeConnection, error) {
	cfg, err := json.Marshal(map[string]any{
		"token":      strings.TrimSpace(input.Token),
		"groupLogin": strings.TrimSpace(input.GroupLogin),
		"namespace":  strings.TrimSpace(input.Namespace),
	})
	if err != nil {
		return domain.KnowledgeConnection{}, ErrInvalidRequestBody
	}
	return s.CreateConnection(ctx, CreateConnectionInput{
		UserID:      input.UserID,
		Provider:    string(domain.ProviderYuque),
		Name:        input.Name,
		SyncEnabled: input.SyncEnabled,
		Config:      cfg,
	})
}

func normalizeFeishuDocumentIDCandidates(values ...string) string {
	for _, raw := range values {
		candidate := strings.TrimSpace(raw)
		if candidate == "" {
			continue
		}
		if !strings.Contains(candidate, "://") {
			return candidate
		}
		parsed, err := url.Parse(candidate)
		if err != nil {
			continue
		}
		if queryValue := strings.TrimSpace(parsed.Query().Get("document_id")); queryValue != "" {
			return queryValue
		}
		path := strings.Trim(parsed.Path, "/")
		segments := strings.Split(path, "/")
		for index, segment := range segments {
			if index+1 >= len(segments) {
				continue
			}
			token := strings.TrimSpace(segments[index+1])
			if token == "" {
				continue
			}
			switch segment {
			case "docx":
				return token
			case "wiki":
				return "wiki:" + token
			}
		}
	}
	return ""
}

func normalizeFeishuWikiTokenCandidates(values ...string) string {
	for _, raw := range values {
		candidate := strings.TrimSpace(raw)
		if candidate == "" {
			continue
		}
		if !strings.Contains(candidate, "://") {
			return strings.TrimPrefix(candidate, "wiki:")
		}
		parsed, err := url.Parse(candidate)
		if err != nil {
			continue
		}
		if queryValue := strings.TrimSpace(parsed.Query().Get("wiki")); queryValue != "" {
			return queryValue
		}
		path := strings.Trim(parsed.Path, "/")
		segments := strings.Split(path, "/")
		for index, segment := range segments {
			if segment != "wiki" || index+1 >= len(segments) {
				continue
			}
			token := strings.TrimSpace(segments[index+1])
			if token != "" {
				return token
			}
		}
	}
	return ""
}

func inferFeishuEntryType(entryType, entryToken string) string {
	normalizedType := strings.TrimSpace(strings.ToLower(entryType))
	if normalizedType == "" {
		normalizedType = "docx"
	}
	if normalizedType == "docx" && feishu.TokenLooksLikeWiki(entryToken) {
		return "wiki_node"
	}
	return normalizedType
}
