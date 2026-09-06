package app

import (
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/auth"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/config"
)

type authBackend interface {
	auth.UserStore
	auth.AuthIdentityStore
	auth.SessionStore
	auth.ChatModelDefaultsStore
	auth.FluxACredentialStore
}

func buildAuthService(authStore authBackend, cfg config.Config) *auth.Service {
	return auth.NewService(auth.ServiceDeps{
		Users:            authStore,
		Identities:       authStore,
		Sessions:         authStore,
		ChatModels:       authStore,
		FluxACredentials: authStore,
	}, cfg)
}
