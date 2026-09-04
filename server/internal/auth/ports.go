package auth

import (
	"context"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

type UserStore interface {
	GetUserByUsername(context.Context, string) (domain.User, error)
	GetUserByAuthIdentity(context.Context, domain.AuthProvider, string) (domain.User, error)
	GetUserByID(context.Context, string) (domain.User, error)
	UpsertUser(context.Context, domain.User) error
}

type AuthIdentityStore interface {
	UpsertAuthIdentity(context.Context, domain.AuthIdentity) error
}

type SessionStore interface {
	GetSessionByRefreshToken(context.Context, string) (domain.Session, error)
	GetSessionByAccessToken(context.Context, string) (domain.Session, error)
	ConsumeSession(context.Context, string) error
	RevokeSession(context.Context, string) error
	CreateSession(context.Context, domain.Session) error
}

type ChatModelDefaultsStore interface {
	EnsureUserChatModelDefaults(context.Context, string) error
}

type ServiceDeps struct {
	Users      UserStore
	Identities AuthIdentityStore
	Sessions   SessionStore
	ChatModels ChatModelDefaultsStore
}
