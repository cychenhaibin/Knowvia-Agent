package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/store"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/golang-jwt/jwt/v5"
)

func TestPasswordHashUsesAdaptiveSaltedHash(t *testing.T) {
	first := hashPassword("correct horse battery staple")
	second := hashPassword("correct horse battery staple")
	if first == second || !strings.HasPrefix(first, "$2") {
		t.Fatalf("expected distinct bcrypt hashes, got %q and %q", first, second)
	}
	if !verifyPassword("correct horse battery staple", first) || verifyPassword("wrong", first) {
		t.Fatal("adaptive password verification returned an unexpected result")
	}
}

func TestLoginMigratesLegacySHA256Password(t *testing.T) {
	mem := store.NewMemoryStore()
	legacy := sha256.Sum256([]byte("pw"))
	if err := mem.UpsertUser(context.Background(), domain.User{ID: "user-1", Username: "alice", PasswordHash: hex.EncodeToString(legacy[:])}); err != nil {
		t.Fatal(err)
	}
	service := NewService(ServiceDeps{Users: mem, Sessions: mem, ChatModels: mem}, testAuthConfig())
	if _, err := service.Login(context.Background(), "alice", "pw"); err != nil {
		t.Fatal(err)
	}
	user, err := mem.GetUserByUsername(context.Background(), "alice")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(user.PasswordHash, "$2") || !verifyPassword("pw", user.PasswordHash) {
		t.Fatalf("legacy password was not upgraded: %q", user.PasswordHash)
	}
}

func TestIssueSessionUsesConfiguredTTLsAndDistinctPurposes(t *testing.T) {
	mem := store.NewMemoryStore()
	service := NewService(ServiceDeps{Users: mem, Sessions: mem, ChatModels: mem}, testAuthConfig())
	if err := mem.UpsertUser(context.Background(), domain.User{ID: "user-1", Username: "alice", PasswordHash: hashPassword("pw")}); err != nil {
		t.Fatal(err)
	}
	pair, err := service.Login(context.Background(), "alice", "pw")
	if err != nil {
		t.Fatal(err)
	}
	parse := func(raw string) jwt.MapClaims {
		t.Helper()
		claims := jwt.MapClaims{}
		if _, err := jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (interface{}, error) { return []byte("test-secret"), nil }); err != nil {
			t.Fatal(err)
		}
		return claims
	}
	accessClaims, refreshClaims := parse(pair.AccessToken), parse(pair.RefreshToken)
	if accessClaims["token_use"] != "access" || refreshClaims["token_use"] != "refresh" {
		t.Fatalf("unexpected token purposes: access=%v refresh=%v", accessClaims["token_use"], refreshClaims["token_use"])
	}
	if accessClaims["jti"] == refreshClaims["jti"] {
		t.Fatal("access and refresh token JTIs must differ")
	}
	if pair.AccessTTL != testAuthConfig().AccessTTL || pair.RefreshTTL != testAuthConfig().RefreshTTL {
		t.Fatalf("unexpected TTLs: %#v", pair)
	}
	if _, ok := accessClaims["exp"]; !ok {
		t.Fatal("access token missing exp")
	}
	if _, ok := refreshClaims["exp"]; !ok {
		t.Fatal("refresh token missing exp")
	}
}

func TestRefreshTokenCannotAuthenticateAsAccessToken(t *testing.T) {
	mem := store.NewMemoryStore()
	service := NewService(ServiceDeps{Users: mem, Sessions: mem, ChatModels: mem}, testAuthConfig())
	if err := mem.UpsertUser(context.Background(), domain.User{ID: "user-1", Username: "alice", PasswordHash: hashPassword("pw")}); err != nil {
		t.Fatal(err)
	}
	pair, err := service.Login(context.Background(), "alice", "pw")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(pair.RefreshToken); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected refresh token rejection, got %v", err)
	}
}

func TestLogoutInvalidatesAccessToken(t *testing.T) {
	mem := store.NewMemoryStore()
	service := NewService(ServiceDeps{Users: mem, Sessions: mem, ChatModels: mem}, testAuthConfig())
	if err := mem.UpsertUser(context.Background(), domain.User{ID: "user-1", Username: "alice", PasswordHash: hashPassword("pw")}); err != nil {
		t.Fatal(err)
	}
	pair, err := service.Login(context.Background(), "alice", "pw")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Logout(context.Background(), pair.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(pair.AccessToken); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected revoked access token rejection, got %v", err)
	}
}

func TestAuthenticateRejectsExpiredAccessToken(t *testing.T) {
	service := NewService(ServiceDeps{Users: store.NewMemoryStore()}, testAuthConfig())
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user-1", "token_use": "access", "exp": time.Now().UTC().Add(-time.Minute).Unix(),
	}).SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected expired token rejection, got %v", err)
	}
}
