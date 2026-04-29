package domain

import "time"

type ChatRole string

const (
	ChatRoleUser      ChatRole = "user"
	ChatRoleAssistant ChatRole = "assistant"
)

type ChatModelPurpose string

const (
	ChatModelPurposeGeneral   ChatModelPurpose = "general"
	ChatModelPurposeKnowledge ChatModelPurpose = "knowledge"
)

type ChatModelOrigin string

const (
	ChatModelOriginDefault ChatModelOrigin = "default"
	ChatModelOriginCustom  ChatModelOrigin = "custom"
)

const (
	DefaultChatAPIBaseURL              = "http://127.0.0.1:11434/v1"
	DefaultChatAPIKey                  = "ollama"
	DefaultGeneralChatModelName        = "gemma3n:e4b"
	DefaultGeneralChatModelNameLabel   = "Gemma 3n E4B"
	DefaultKnowledgeChatModelName      = "qwen3:8b"
	DefaultKnowledgeChatModelNameLabel = "Qwen3 8B"
	DefaultGeneralChatTemperature      = 0.05
	DefaultKnowledgeChatTemperature    = 0.05
)

type ChatRuntimeConfig struct {
	BaseURL        string
	APIKey         string
	ModelName      string
	Temperature    float64
	TemperatureSet bool
	EnableSearch   bool
}

type ChatUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

func (u ChatUsage) IsZero() bool {
	return u.PromptTokens == 0 && u.CompletionTokens == 0 && u.TotalTokens == 0
}

func DefaultChatTemperatureForPurpose(purpose ChatModelPurpose) float64 {
	switch purpose {
	case ChatModelPurposeKnowledge:
		return DefaultKnowledgeChatTemperature
	default:
		return DefaultGeneralChatTemperature
	}
}

func DefaultChatModelConfigForPurpose(purpose ChatModelPurpose) (name string, runtime ChatRuntimeConfig) {
	switch purpose {
	case ChatModelPurposeKnowledge:
		return DefaultKnowledgeChatModelNameLabel, ChatRuntimeConfig{
			BaseURL:     DefaultChatAPIBaseURL,
			APIKey:      DefaultChatAPIKey,
			ModelName:   DefaultKnowledgeChatModelName,
			Temperature: DefaultKnowledgeChatTemperature,
		}
	default:
		return DefaultGeneralChatModelNameLabel, ChatRuntimeConfig{
			BaseURL:     DefaultChatAPIBaseURL,
			APIKey:      DefaultChatAPIKey,
			ModelName:   DefaultGeneralChatModelName,
			Temperature: DefaultGeneralChatTemperature,
		}
	}
}

type ChatSession struct {
	ID            string
	UserID        string
	Title         string
	Pinned        bool
	LastMessageAt *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type ChatMessage struct {
	ID           string
	SessionID    string
	UserID       string
	Role         ChatRole
	Content      string
	Skill        string
	UseKnowledge bool
	Usage        *ChatUsage
	CreatedAt    time.Time
	CompletedAt  *time.Time
}

type ChatMessageSource struct {
	ID           string
	MessageID    string
	Provider     Provider
	ConnectionID string
	DocumentID   string
	ChunkID      string
	Title        string
	Repo         string
	URL          string
	Snippet      string
	MatchedLines []string
	Score        float64
	CreatedAt    time.Time
}

type UserChatModel struct {
	ID          string
	UserID      string
	Purpose     ChatModelPurpose
	Origin      ChatModelOrigin
	Name        string
	BaseURL     string
	APIKey      string
	ModelName   string
	Temperature float64
	IsSelected  bool
	Available   bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (m UserChatModel) RuntimeConfig() ChatRuntimeConfig {
	return ChatRuntimeConfig{
		BaseURL:     m.BaseURL,
		APIKey:      m.APIKey,
		ModelName:   m.ModelName,
		Temperature: m.Temperature,
	}
}
