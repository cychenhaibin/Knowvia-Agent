package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"net/http"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestVerifyIDTokenReturnsGoogleKeysUnavailableWhenJWKSFetchFails(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": "google-user-1",
		"aud": "web-client-id",
		"iss": "https://accounts.google.com",
	})
	token.Header["kid"] = "kid-1"
	rawToken, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	verifier := &googleTokenVerifier{
		webClientID: "web-client-id",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("network down")
			}),
		},
	}

	_, err = verifier.VerifyIDToken(context.Background(), rawToken)
	if !errors.Is(err, ErrGoogleKeysUnavailable) {
		t.Fatalf("expected ErrGoogleKeysUnavailable, got %v", err)
	}
}
