package chat

import (
	"context"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillresolver"
)

type ChatModelStore interface {
	EnsureUserChatModelDefaults(ctx context.Context, userID string) error
	ListUserChatModels(ctx context.Context, userID string) ([]domain.UserChatModel, error)
	GetUserChatModel(ctx context.Context, userID, modelID string) (domain.UserChatModel, error)
	GetSelectedUserChatModel(ctx context.Context, userID string, purpose domain.ChatModelPurpose) (domain.UserChatModel, error)
	CreateUserChatModel(ctx context.Context, model domain.UserChatModel) error
	UpdateUserChatModel(ctx context.Context, model domain.UserChatModel) error
	SelectUserChatModel(ctx context.Context, userID, modelID string) error
	DeleteUserChatModel(ctx context.Context, userID, modelID string) error
}

type SessionStore interface {
	CreateChatSession(context.Context, domain.ChatSession) error
	GetChatSession(context.Context, string, string) (domain.ChatSession, error)
	UpdateChatSession(context.Context, domain.ChatSession) error
	ListChatSessions(context.Context, string) ([]domain.ChatSession, error)
	DeleteChatSession(context.Context, string, string) error
	ListChatMessages(context.Context, string, string) ([]domain.ChatMessage, error)
	ListChatMessageSources(context.Context, string) ([]domain.ChatMessageSource, error)
}

type ConversationStore interface {
	CreateChatSession(context.Context, domain.ChatSession) error
	GetChatSession(context.Context, string, string) (domain.ChatSession, error)
	UpdateChatSession(context.Context, domain.ChatSession) error
	SaveChatMessage(context.Context, domain.ChatMessage) error
	SaveChatMessageSources(context.Context, string, []domain.ChatMessageSource) error
	CreateSkillRuntimeSnapshot(context.Context, domain.SkillRuntimeSnapshot) error
}

type PrepareStreamStore interface {
	ChatModelStore
	skillresolver.InstallationRecordStore
	GetSkill(ctx context.Context, userID, skillID string) (domain.Skill, error)
}

type ServiceDeps struct {
	Models        ChatModelStore
	Sessions      SessionStore
	Conversations ConversationStore
	Prepare       PrepareStreamStore
}
