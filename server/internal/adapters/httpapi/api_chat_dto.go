package httpapi

import (
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/chat"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

type chatModelsResponseDTO struct {
	GeneralModels   []chatModelDTO `json:"generalModels"`
	KnowledgeModels []chatModelDTO `json:"knowledgeModels"`
}

type chatModelDTO struct {
	ID          string                  `json:"id"`
	UserID      string                  `json:"userId"`
	Purpose     domain.ChatModelPurpose `json:"purpose"`
	Origin      domain.ChatModelOrigin  `json:"origin"`
	Name        string                  `json:"name"`
	BaseURL     string                  `json:"baseUrl"`
	APIKey      string                  `json:"apiKey,omitempty"`
	ModelName   string                  `json:"modelName"`
	Temperature float64                 `json:"temperature"`
	IsSelected  bool                    `json:"isSelected"`
	Available   bool                    `json:"available"`
	CreatedAt   time.Time               `json:"createdAt"`
	UpdatedAt   time.Time               `json:"updatedAt"`
}

type chatSessionListDTO struct {
	Items []chatSessionDTO `json:"items"`
}

type chatMessageListDTO struct {
	Items []chatMessageDTO `json:"items"`
}

type chatSessionDTO struct {
	ID            string     `json:"id"`
	UserID        string     `json:"userId"`
	Title         string     `json:"title"`
	Pinned        bool       `json:"pinned"`
	LastMessageAt *time.Time `json:"lastMessageAt,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

type chatMessageSourceDTO struct {
	Provider           domain.Provider `json:"provider"`
	Title              string          `json:"title"`
	Repo               string          `json:"repo,omitempty"`
	URL                string          `json:"url,omitempty"`
	Snippet            string          `json:"snippet"`
	MatchedAnswerLines []string        `json:"matchedAnswerLines,omitempty"`
	Score              float64         `json:"score"`
}

type chatUsageDTO struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type chatStreamSourceDTO struct {
	Provider           domain.Provider `json:"provider"`
	Title              string          `json:"title"`
	Repo               string          `json:"repo,omitempty"`
	URL                string          `json:"url,omitempty"`
	Snippet            string          `json:"snippet"`
	MatchedAnswerLines []string        `json:"matched_answer_lines,omitempty"`
	Score              float64         `json:"score"`
}

type chatMessageDTO struct {
	ID           string                 `json:"id"`
	SessionID    string                 `json:"sessionId"`
	Role         domain.ChatRole        `json:"role"`
	Content      string                 `json:"content"`
	Skill        string                 `json:"skill"`
	UseKnowledge bool                   `json:"useKnowledge"`
	CreatedAt    time.Time              `json:"createdAt"`
	CompletedAt  *time.Time             `json:"completedAt,omitempty"`
	Sources      []chatMessageSourceDTO `json:"sources"`
	Usage        *chatUsageDTO          `json:"usage,omitempty"`
}

func mapChatModel(model domain.UserChatModel) chatModelDTO {
	return chatModelDTO{
		ID:          model.ID,
		UserID:      model.UserID,
		Purpose:     model.Purpose,
		Origin:      model.Origin,
		Name:        model.Name,
		BaseURL:     model.BaseURL,
		APIKey:      model.APIKey,
		ModelName:   model.ModelName,
		Temperature: model.Temperature,
		IsSelected:  model.IsSelected,
		Available:   model.Available,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}
}

func mapChatModelsResponse(models []domain.UserChatModel) chatModelsResponseDTO {
	response := chatModelsResponseDTO{
		GeneralModels:   make([]chatModelDTO, 0),
		KnowledgeModels: make([]chatModelDTO, 0),
	}
	for _, model := range models {
		mapped := mapChatModel(model)
		switch model.Purpose {
		case domain.ChatModelPurposeKnowledge:
			response.KnowledgeModels = append(response.KnowledgeModels, mapped)
		default:
			response.GeneralModels = append(response.GeneralModels, mapped)
		}
	}
	return response
}

func mapChatSession(session domain.ChatSession) chatSessionDTO {
	return chatSessionDTO{
		ID:            session.ID,
		UserID:        session.UserID,
		Title:         session.Title,
		Pinned:        session.Pinned,
		LastMessageAt: session.LastMessageAt,
		CreatedAt:     session.CreatedAt,
		UpdatedAt:     session.UpdatedAt,
	}
}

func mapChatSessions(sessions []domain.ChatSession) []chatSessionDTO {
	items := make([]chatSessionDTO, 0, len(sessions))
	for _, session := range sessions {
		items = append(items, mapChatSession(session))
	}
	return items
}

func mapChatMessageSource(source chat.MessageSourceView) chatMessageSourceDTO {
	return chatMessageSourceDTO{
		Provider:           source.Provider,
		Title:              source.Title,
		Repo:               source.Repo,
		URL:                source.URL,
		Snippet:            source.Snippet,
		MatchedAnswerLines: append([]string(nil), source.MatchedLines...),
		Score:              source.Score,
	}
}

func mapChatMessageSources(sources []chat.MessageSourceView) []chatMessageSourceDTO {
	items := make([]chatMessageSourceDTO, 0, len(sources))
	for _, source := range sources {
		items = append(items, mapChatMessageSource(source))
	}
	return items
}

func mapChatMessage(message chat.MessageView) chatMessageDTO {
	return chatMessageDTO{
		ID:           message.ID,
		SessionID:    message.SessionID,
		Role:         message.Role,
		Content:      message.Content,
		Skill:        message.Skill,
		UseKnowledge: message.UseKnowledge,
		CreatedAt:    message.CreatedAt,
		CompletedAt:  message.CompletedAt,
		Sources:      mapChatMessageSources(message.Sources),
		Usage:        mapChatUsage(message.Usage),
	}
}

func mapChatMessages(messages []chat.MessageView) []chatMessageDTO {
	items := make([]chatMessageDTO, 0, len(messages))
	for _, message := range messages {
		items = append(items, mapChatMessage(message))
	}
	return items
}

func mapChatStreamSource(evidence domain.Evidence) chatStreamSourceDTO {
	return chatStreamSourceDTO{
		Provider:           evidence.Provider,
		Title:              evidence.Title,
		Repo:               evidence.Repo,
		URL:                evidence.URL,
		Snippet:            evidence.Snippet,
		MatchedAnswerLines: append([]string(nil), evidence.MatchedLines...),
		Score:              evidence.Score,
	}
}

func mapChatStreamSources(evidences []domain.Evidence) []chatStreamSourceDTO {
	items := make([]chatStreamSourceDTO, 0, len(evidences))
	for _, evidence := range evidences {
		items = append(items, mapChatStreamSource(evidence))
	}
	return items
}

func mapChatUsage(usage *domain.ChatUsage) *chatUsageDTO {
	if usage == nil || usage.IsZero() {
		return nil
	}
	return &chatUsageDTO{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}
}
