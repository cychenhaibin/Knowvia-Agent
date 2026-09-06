package httpapi

import (
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/auth"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

type authUserDTO struct {
	ID          string          `json:"id"`
	Username    string          `json:"username"`
	DisplayName string          `json:"displayName"`
	Email       string          `json:"email,omitempty"`
	AvatarURL   string          `json:"avatarUrl,omitempty"`
	FluxASite   *auth.FluxASite `json:"fluxaSite,omitempty"`
}

type sessionDTO struct {
	User         authUserDTO `json:"user"`
	AccessToken  string      `json:"accessToken"`
	RefreshToken string      `json:"refreshToken"`
	ExpiresIn    int         `json:"expiresIn"`
}

func mapAuthUser(user domain.User) authUserDTO {
	return authUserDTO{
		ID:          user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Email:       user.Email,
		AvatarURL:   user.AvatarURL,
	}
}

func mapSession(tokens auth.TokenPair) sessionDTO {
	user := mapAuthUser(tokens.User)
	user.FluxASite = tokens.FluxASite

	return sessionDTO{
		User:         user,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    int(tokens.AccessTTL.Seconds()),
	}
}
