package auth

import (
	"context"
	"testing"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/config"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/store"
)

type staticGoogleVerifier struct {
	identity VerifiedGoogleIdentity
	err      error
}

func (v staticGoogleVerifier) VerifyIDToken(context.Context, string) (VerifiedGoogleIdentity, error) {
	return v.identity, v.err
}

type staticMicrosoftVerifier struct {
	identity VerifiedMicrosoftIdentity
	err      error
}

func (v staticMicrosoftVerifier) VerifyIDToken(context.Context, string) (VerifiedMicrosoftIdentity, error) {
	return v.identity, v.err
}

func testAuthConfig() config.Config {
	return config.Config{
		JWTSecret:  "test-secret",
		AccessTTL:  30 * time.Minute,
		RefreshTTL: 24 * time.Hour,
	}
}

func TestLoginWithGoogleCreatesUserAndIdentity(t *testing.T) {
	mem := store.NewMemoryStore()
	service := NewServiceWithGoogleVerifier(mem, testAuthConfig(), staticGoogleVerifier{
		identity: VerifiedGoogleIdentity{
			Subject:       "google-subject-1",
			Email:         "haibinchenleo@gmail.com",
			EmailVerified: true,
			DisplayName:   "Haibin Chen",
			AvatarURL:     "https://example.com/avatar.png",
		},
	})

	tokens, err := service.LoginWithGoogle(context.Background(), "id-token")
	if err != nil {
		t.Fatalf("login with google: %v", err)
	}
	if tokens.User.ID == "" || tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatalf("expected issued session, got %#v", tokens)
	}
	if tokens.User.Email != "haibinchenleo@gmail.com" {
		t.Fatalf("expected email to be populated, got %q", tokens.User.Email)
	}

	user, err := mem.GetUserByAuthIdentity(context.Background(), domain.AuthProviderGoogle, "google-subject-1")
	if err != nil {
		t.Fatalf("lookup user by auth identity: %v", err)
	}
	if user.ID != tokens.User.ID {
		t.Fatalf("expected same user ID, got %s want %s", user.ID, tokens.User.ID)
	}
}

func TestLoginWithGoogleCreatesDistinctUserWhenEmailAlreadyExists(t *testing.T) {
	mem := store.NewMemoryStore()
	existing := domain.User{
		ID:           "user-1",
		Username:     "haibin",
		DisplayName:  "Haibin",
		Email:        "haibinchenleo@gmail.com",
		PasswordHash: "hashed",
		CreatedAt:    time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), existing); err != nil {
		t.Fatalf("upsert existing user: %v", err)
	}

	service := NewServiceWithGoogleVerifier(mem, testAuthConfig(), staticGoogleVerifier{
		identity: VerifiedGoogleIdentity{
			Subject:       "google-subject-2",
			Email:         "haibinchenleo@gmail.com",
			EmailVerified: true,
			DisplayName:   "Haibin Chen",
		},
	})

	tokens, err := service.LoginWithGoogle(context.Background(), "id-token")
	if err != nil {
		t.Fatalf("login with google: %v", err)
	}
	if tokens.User.ID == existing.ID {
		t.Fatalf("expected google login to create a separate account for the same email")
	}
	if tokens.User.Email != existing.Email {
		t.Fatalf("expected google login to keep provider email, got %q want %q", tokens.User.Email, existing.Email)
	}
}

func TestLoginWithMicrosoftKeepsProviderIdentitySeparateFromSameEmailUser(t *testing.T) {
	mem := store.NewMemoryStore()

	legacyMicrosoftUser := domain.User{
		ID:           "user-legacy-ms",
		Username:     "google-weird-subject",
		DisplayName:  "google-weird-subject",
		Email:        "",
		PasswordHash: "",
		CreatedAt:    time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), legacyMicrosoftUser); err != nil {
		t.Fatalf("upsert legacy microsoft user: %v", err)
	}
	if err := mem.UpsertAuthIdentity(context.Background(), domain.AuthIdentity{
		ID:              authIdentityID(domain.AuthProviderMicrosoft, "microsoft-subject-1"),
		UserID:          legacyMicrosoftUser.ID,
		Provider:        domain.AuthProviderMicrosoft,
		ProviderSubject: "microsoft-subject-1",
		Email:           "",
		EmailVerified:   false,
		AvatarURL:       "",
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}); err != nil {
		t.Fatalf("upsert legacy microsoft identity: %v", err)
	}

	existingEmailUser := domain.User{
		ID:           "user-email-owner",
		Username:     "haibin",
		DisplayName:  "Haibin",
		Email:        "haibinchenleo@outlook.com",
		PasswordHash: "hashed",
		CreatedAt:    time.Now().UTC(),
	}
	if err := mem.UpsertUser(context.Background(), existingEmailUser); err != nil {
		t.Fatalf("upsert existing email user: %v", err)
	}

	service := NewServiceWithMicrosoftVerifier(mem, testAuthConfig(), staticMicrosoftVerifier{
		identity: VerifiedMicrosoftIdentity{
			Subject:       "microsoft-subject-1",
			Email:         "haibinchenleo@outlook.com",
			EmailVerified: true,
			DisplayName:   "Haibin Chen",
		},
	})

	tokens, err := service.LoginWithMicrosoft(context.Background(), "id-token")
	if err != nil {
		t.Fatalf("login with microsoft: %v", err)
	}
	if tokens.User.ID != legacyMicrosoftUser.ID {
		t.Fatalf("expected microsoft login to stay bound to its existing provider account, got %s want %s", tokens.User.ID, legacyMicrosoftUser.ID)
	}

	linkedUser, err := mem.GetUserByAuthIdentity(context.Background(), domain.AuthProviderMicrosoft, "microsoft-subject-1")
	if err != nil {
		t.Fatalf("lookup microsoft auth identity: %v", err)
	}
	if linkedUser.ID != legacyMicrosoftUser.ID {
		t.Fatalf("expected microsoft identity to remain on its original provider account, got %s want %s", linkedUser.ID, legacyMicrosoftUser.ID)
	}
}
