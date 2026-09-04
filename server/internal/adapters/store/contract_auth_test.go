package store

import (
	"context"
	"testing"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func testAuthContract(t *testing.T, s contractStore) {
	t.Helper()

	ctx := context.Background()
	now := contractTime(0)
	user := domain.User{
		ID:           "user-contract-1",
		Username:     "contract-admin",
		DisplayName:  "Contract Admin",
		Email:        "contract@example.com",
		AvatarURL:    "https://example.com/avatar.png",
		PasswordHash: "hashed-password",
		CreatedAt:    now,
	}
	if err := s.UpsertUser(ctx, user); err != nil {
		t.Fatalf("UpsertUser failed: %v", err)
	}

	gotUserByUsername, err := s.GetUserByUsername(ctx, user.Username)
	if err != nil {
		t.Fatalf("GetUserByUsername failed: %v", err)
	}
	assertDeepEqual(t, "user by username", gotUserByUsername, user)

	gotUserByID, err := s.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	assertDeepEqual(t, "user by id", gotUserByID, user)

	identity := domain.AuthIdentity{
		ID:              "identity-contract-1",
		UserID:          user.ID,
		Provider:        domain.AuthProviderGoogle,
		ProviderSubject: "google-subject-123",
		Email:           user.Email,
		EmailVerified:   true,
		AvatarURL:       user.AvatarURL,
		CreatedAt:       contractTime(1),
		UpdatedAt:       contractTime(2),
	}
	if err := s.UpsertAuthIdentity(ctx, identity); err != nil {
		t.Fatalf("UpsertAuthIdentity failed: %v", err)
	}

	gotUserByIdentity, err := s.GetUserByAuthIdentity(ctx, identity.Provider, identity.ProviderSubject)
	if err != nil {
		t.Fatalf("GetUserByAuthIdentity failed: %v", err)
	}
	assertDeepEqual(t, "user by auth identity", gotUserByIdentity, user)

	session := domain.Session{
		ID:           "session-contract-1",
		UserID:       user.ID,
		AccessToken:  "access-contract-token",
		RefreshToken: "refresh-contract-token",
		ExpiresAt:    contractTime(3),
		CreatedAt:    contractTime(4),
	}
	if err := s.CreateSession(ctx, session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	gotSession, err := s.GetSessionByRefreshToken(ctx, session.RefreshToken)
	if err != nil {
		t.Fatalf("GetSessionByRefreshToken failed: %v", err)
	}
	assertDeepEqual(t, "session before revoke", gotSession, session)
	gotAccessSession, err := s.GetSessionByAccessToken(ctx, session.AccessToken)
	if err != nil {
		t.Fatalf("GetSessionByAccessToken failed: %v", err)
	}
	assertDeepEqual(t, "session by access token", gotAccessSession, session)

	if err := s.RevokeSession(ctx, session.ID); err != nil {
		t.Fatalf("RevokeSession failed: %v", err)
	}
	gotRevokedSession, err := s.GetSessionByRefreshToken(ctx, session.RefreshToken)
	if err != nil {
		t.Fatalf("GetSessionByRefreshToken after revoke failed: %v", err)
	}
	if gotRevokedSession.RevokedAt == nil {
		t.Fatalf("expected revoked session to have RevokedAt set")
	}
	if gotRevokedSession.RevokedAt.IsZero() {
		t.Fatalf("expected revoked session to have non-zero RevokedAt")
	}
	gotRevokedSession.RevokedAt = nil
	assertDeepEqual(t, "session after revoke", gotRevokedSession, session)

	consumable := domain.Session{
		ID:           "session-contract-consume",
		UserID:       user.ID,
		AccessToken:  "access-contract-consume",
		RefreshToken: "refresh-contract-consume",
		ExpiresAt:    time.Now().UTC().Add(time.Hour),
		CreatedAt:    time.Now().UTC(),
	}
	if err := s.CreateSession(ctx, consumable); err != nil {
		t.Fatalf("CreateSession for consume failed: %v", err)
	}
	if err := s.ConsumeSession(ctx, consumable.ID); err != nil {
		t.Fatalf("ConsumeSession failed: %v", err)
	}
	if err := s.ConsumeSession(ctx, consumable.ID); err == nil {
		t.Fatal("expected second ConsumeSession call to reject replay")
	}
}
