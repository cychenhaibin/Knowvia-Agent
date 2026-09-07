package domain

import "time"

type AuthProvider string

const (
	AuthProviderPassword  AuthProvider = "password"
	AuthProviderGoogle    AuthProvider = "google"
	AuthProviderMicrosoft AuthProvider = "microsoft"
	AuthProviderWeChat    AuthProvider = "wechat"
	AuthProviderApple     AuthProvider = "apple"
	AuthProviderFacebook  AuthProvider = "facebook"
)

type User struct {
	ID           string
	Username     string
	DisplayName  string
	Email        string
	AvatarURL    string
	PasswordHash string
	CreatedAt    time.Time
}

type AuthIdentity struct {
	ID              string
	UserID          string
	Provider        AuthProvider
	ProviderSubject string
	Email           string
	EmailVerified   bool
	AvatarURL       string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Session struct {
	ID           string
	UserID       string
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	CreatedAt    time.Time
	RevokedAt    *time.Time
}
