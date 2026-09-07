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
	RevokeSession(context.Context, string) error
	CreateSession(context.Context, domain.Session) error
}

type ChatModelDefaultsStore interface {
	EnsureUserChatModelDefaults(context.Context, string) error
}

type FluxAIdentityVerifier interface {
	Verify(context.Context, FluxASite, string) (VerifiedFluxAIdentity, error)
}

type FluxACredentialAuthenticator interface {
	Login(context.Context, FluxASite, string, string) (string, error)
}

type FluxACredentialStore interface {
	UpsertFluxACredential(context.Context, domain.FluxACredential) error
	GetFluxACredential(context.Context, string, FluxASite) (domain.FluxACredential, error)
}

type FluxACredentialCipher interface {
	Encrypt(plaintext string, additionalData []byte) (string, error)
	Decrypt(ciphertext string, additionalData []byte) (string, error)
}

type FluxAModelGroupsFetcher interface {
	List(context.Context, FluxASite, string) ([]FluxAModelGroup, error)
}

type FluxAModelsFetcher interface {
	Models(context.Context, FluxASite, string, string) ([]FluxAModel, error)
}

type FluxABalanceFetcher interface {
	Balance(context.Context, FluxASite, string) (FluxABalance, error)
}

type ServiceDeps struct {
	Users              UserStore
	Identities         AuthIdentityStore
	Sessions           SessionStore
	ChatModels         ChatModelDefaultsStore
	FluxAVerifier      FluxAIdentityVerifier
	FluxAAuthenticator FluxACredentialAuthenticator
	FluxACredentials   FluxACredentialStore
	FluxACipher        FluxACredentialCipher
	FluxAModels        FluxAModelGroupsFetcher
	FluxABalance       FluxABalanceFetcher
}
