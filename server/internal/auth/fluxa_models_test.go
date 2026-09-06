package auth

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/store"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func TestFluxAModelGroupsLogsSafeUpstreamFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/user/self/groups" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`do-not-log-me`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	var logs bytes.Buffer
	previousWriter := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previousWriter) })

	service, user := newFluxAModelGroupsService(t, server.URL, server.URL, server.Client())
	_, err := service.ListFluxAModelGroups(context.Background(), user.ID, FluxASitePaid)
	if !errors.Is(err, ErrFluxAUnavailable) {
		t.Fatalf("error = %v, want ErrFluxAUnavailable", err)
	}
	if !strings.Contains(logs.String(), "fluxa_model_groups_upstream_failure") || !strings.Contains(logs.String(), "route=/api/user/self/groups") || !strings.Contains(logs.String(), "status=502") {
		t.Fatalf("logs = %q, want safe route and status diagnostic", logs.String())
	}
	if strings.Contains(logs.String(), "upstream-token") || strings.Contains(logs.String(), "do-not-log-me") {
		t.Fatalf("logs exposed secret material: %q", logs.String())
	}
}

func TestFluxAModelGroupsLogsSafeParseFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/user/self/groups":
			_, _ = w.Write([]byte(`{"success":true,"data":[{"id":"model-a","name":42,"diagnostic":"do-not-log-me"}]}`))
		}
	}))
	t.Cleanup(server.Close)

	var logs bytes.Buffer
	previousWriter := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previousWriter) })

	service, user := newFluxAModelGroupsService(t, server.URL, server.URL, server.Client())
	_, err := service.ListFluxAModelGroups(context.Background(), user.ID, FluxASitePaid)
	if !errors.Is(err, ErrFluxAUnavailable) {
		t.Fatalf("error = %v, want ErrFluxAUnavailable", err)
	}
	if !strings.Contains(logs.String(), "fluxa_model_groups_parse_failure") || !strings.Contains(logs.String(), "route=/api/user/self/groups") || !strings.Contains(logs.String(), "classification=groups_payload_invalid") {
		t.Fatalf("logs = %q, want safe parse diagnostic", logs.String())
	}
	if strings.Contains(logs.String(), "upstream-token") || strings.Contains(logs.String(), "do-not-log-me") {
		t.Fatalf("logs exposed secret material: %q", logs.String())
	}
}

func TestListFluxAModelGroupsUsesConfiguredOriginAndNormalizesPayload(t *testing.T) {
	var mu sync.Mutex
	requests := make([]string, 0, 1)
	var recordedAuthorization string

	paidServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("paid origin received a request")
	}))
	t.Cleanup(paidServer.Close)
	freeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.URL.Path)
		recordedAuthorization = r.Header.Get("Authorization")
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/user/self/groups":
			_, _ = w.Write([]byte(`{"success":true,"data":[
				{"name":"default","models":[{"id":"gpt-4o","name":"GPT-4o"},{"id":"gpt-4o-mini","name":"GPT-4o mini"}]},
				{"name":"research","models":[{"id":"gpt-4o","name":"GPT-4o"}]}
			]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(freeServer.Close)

	service, user := newFluxAModelGroupsService(t, paidServer.URL, freeServer.URL, freeServer.Client())
	groups, err := service.ListFluxAModelGroups(context.Background(), user.ID, FluxASiteFree)
	if err != nil {
		t.Fatalf("list groups: %v", err)
	}
	want := []FluxAModelGroup{
		{Name: "default", Models: []FluxAModel{{ID: "gpt-4o", Name: "GPT-4o"}, {ID: "gpt-4o-mini", Name: "GPT-4o mini"}}},
		{Name: "research", Models: []FluxAModel{{ID: "gpt-4o", Name: "GPT-4o"}}},
	}
	if !reflect.DeepEqual(groups, want) {
		t.Fatalf("groups = %#v, want %#v", groups, want)
	}
	if recordedAuthorization != "Bearer upstream-token" {
		t.Fatalf("authorization = %q, want upstream bearer token", recordedAuthorization)
	}
	if !reflect.DeepEqual(requests, []string{"/api/user/self/groups"}) {
		t.Fatalf("requests = %#v, want selected group endpoint", requests)
	}
}

func TestListFluxAModelGroupsAcceptsSingleAccountGroupString(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/user/self/groups":
			_, _ = w.Write([]byte(`{"success":true,"data":{"groups":[{"name":"premium","models":[{"id":"model-a","name":"Model A"}]}]}}`))
		}
	}))
	t.Cleanup(server.Close)

	service, user := newFluxAModelGroupsService(t, server.URL, server.URL, server.Client())
	groups, err := service.ListFluxAModelGroups(context.Background(), user.ID, FluxASitePaid)
	if err != nil {
		t.Fatalf("list groups: %v", err)
	}
	want := []FluxAModelGroup{{Name: "premium", Models: []FluxAModel{{ID: "model-a", Name: "Model A"}}}}
	if !reflect.DeepEqual(groups, want) {
		t.Fatalf("groups = %#v, want %#v", groups, want)
	}
}

func TestListFluxAModelGroupsAcceptsGroupedModelMap(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/user/self/groups":
			_, _ = w.Write([]byte(`{"success":true,"data":{"11":["gpt-4o"],"14":["claude-3"]}}`))
		}
	}))
	t.Cleanup(server.Close)
	service, user := newFluxAModelGroupsService(t, server.URL, server.URL, server.Client())
	groups, err := service.ListFluxAModelGroups(context.Background(), user.ID, FluxASitePaid)
	if err != nil {
		t.Fatalf("list groups: %v", err)
	}
	want := []FluxAModelGroup{
		{Name: "11", Models: []FluxAModel{{ID: "gpt-4o", Name: "gpt-4o"}}},
		{Name: "14", Models: []FluxAModel{{ID: "claude-3", Name: "claude-3"}}},
	}
	if !reflect.DeepEqual(groups, want) {
		t.Fatalf("groups = %#v, want %#v", groups, want)
	}
}

func TestListFluxAModelGroupsAcceptsGroupMetadataMap(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/user/self/groups" {
			_, _ = w.Write([]byte(`{"success":true,"data":{"cc_max":{"desc":"claude code max分组","ratio":0.3},"公益组":{"desc":"","ratio":0.001}}}`))
		}
	}))
	t.Cleanup(server.Close)
	service, user := newFluxAModelGroupsService(t, server.URL, server.URL, server.Client())
	groups, err := service.ListFluxAModelGroups(context.Background(), user.ID, FluxASitePaid)
	if err != nil {
		t.Fatalf("list groups: %v", err)
	}
	want := []FluxAModelGroup{{Name: "cc_max", Desc: "claude code max分组", Ratio: 0.3}, {Name: "公益组", Ratio: 0.001}}
	if !reflect.DeepEqual(groups, want) {
		t.Fatalf("groups = %#v, want %#v", groups, want)
	}
}

func TestListFluxAModelGroupsMapsMissingCredentialAndExpiredUpstreamToken(t *testing.T) {
	service, user := newFluxAModelGroupsServiceWithCredentialSites(t, "https://paid.example", "https://free.example", nil, nil)
	_, err := service.ListFluxAModelGroups(context.Background(), user.ID, FluxASiteFree)
	if !errors.Is(err, ErrFluxANotConnected) {
		t.Fatalf("missing credential error = %v, want ErrFluxANotConnected", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("upstream-secret-diagnostic"))
	}))
	t.Cleanup(server.Close)
	service, user = newFluxAModelGroupsService(t, server.URL, server.URL, server.Client())
	_, err = service.ListFluxAModelGroups(context.Background(), user.ID, FluxASitePaid)
	if !errors.Is(err, ErrFluxAReauthenticationRequired) {
		t.Fatalf("expired credential error = %v, want ErrFluxAReauthenticationRequired", err)
	}
	if strings.Contains(err.Error(), "upstream-secret-diagnostic") || strings.Contains(err.Error(), "upstream-token") {
		t.Fatalf("error exposed upstream material: %v", err)
	}
}

func TestListFluxAModelGroupsRejectsMalformedUpstreamPayload(t *testing.T) {
	tests := []struct {
		name       string
		selfBody   string
		modelsBody string
	}{
		{
			name:       "non-string account group",
			selfBody:   `{"success":true,"data":{"group":42}}`,
			modelsBody: `{"success":true,"data":[]}`,
		},
		{
			name:       "model without identifier",
			selfBody:   `{"success":true,"data":{"group":"default"}}`,
			modelsBody: `{"success":true,"data":[{"id":" ","name":"Model without ID"}]}`,
		},
		{
			name:       "model with invalid groups",
			selfBody:   `{"success":true,"data":{"group":"default"}}`,
			modelsBody: `{"success":true,"data":[{"id":"model-a","name":"Model A","groups":42}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/user/self":
					_, _ = w.Write([]byte(tt.selfBody))
				case "/api/models":
					_, _ = w.Write([]byte(tt.modelsBody))
				}
			}))
			t.Cleanup(server.Close)

			service, user := newFluxAModelGroupsService(t, server.URL, server.URL, server.Client())
			_, err := service.ListFluxAModelGroups(context.Background(), user.ID, FluxASitePaid)
			if !errors.Is(err, ErrFluxAUnavailable) {
				t.Fatalf("error = %v, want ErrFluxAUnavailable", err)
			}
		})
	}
}

func TestListFluxAModelGroupsRejectsOversizedUpstreamPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/user/self":
			_, _ = w.Write([]byte(`{"success":true,"data":{"group":"default"}}`))
			_, _ = w.Write([]byte(strings.Repeat(" ", maxFluxAModelsResponseBytes)))
		case "/api/models":
			_, _ = w.Write([]byte(`{"success":true,"data":[]}`))
		}
	}))
	t.Cleanup(server.Close)

	service, user := newFluxAModelGroupsService(t, server.URL, server.URL, server.Client())
	_, err := service.ListFluxAModelGroups(context.Background(), user.ID, FluxASitePaid)
	if !errors.Is(err, ErrFluxAUnavailable) {
		t.Fatalf("error = %v, want ErrFluxAUnavailable", err)
	}
}

func newFluxAModelGroupsService(t *testing.T, paidOrigin, freeOrigin string, baseClient *http.Client) (*Service, domain.User) {
	return newFluxAModelGroupsServiceWithCredentialSites(t, paidOrigin, freeOrigin, baseClient, []FluxASite{FluxASitePaid, FluxASiteFree})
}

func newFluxAModelGroupsServiceWithCredentialSites(t *testing.T, paidOrigin, freeOrigin string, baseClient *http.Client, credentialSites []FluxASite) (*Service, domain.User) {
	t.Helper()
	memory := store.NewMemoryStore()
	user := domain.User{ID: "model-groups-user", Username: "model-groups-user", DisplayName: "Model Groups User", CreatedAt: time.Now().UTC()}
	if err := memory.UpsertUser(context.Background(), user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	cipher, err := NewFluxACredentialCipher(make([]byte, 32))
	if err != nil {
		t.Fatalf("new credential cipher: %v", err)
	}
	for _, site := range credentialSites {
		ciphertext, err := cipher.Encrypt("upstream-token", fluxACredentialAdditionalData(user.ID, site))
		if err != nil {
			t.Fatalf("encrypt credential: %v", err)
		}
		if err := memory.UpsertFluxACredential(context.Background(), domain.FluxACredential{UserID: user.ID, Site: site, TokenCiphertext: ciphertext}); err != nil {
			t.Fatalf("upsert credential: %v", err)
		}
	}
	cfg := testAuthConfig()
	cfg.FluxAPaidOrigin = strings.Replace(paidOrigin, "http://", "https://", 1)
	cfg.FluxAFreeOrigin = strings.Replace(freeOrigin, "http://", "https://", 1)
	cfg.FluxACredentialsKey = make([]byte, 32)
	service := NewService(ServiceDeps{
		Users:            memory,
		Identities:       memory,
		Sessions:         memory,
		ChatModels:       memory,
		FluxACredentials: memory,
		FluxACipher:      cipher,
		FluxAModels: newFluxAModelGroupsFetcher(
			cfg.FluxAPaidOrigin,
			cfg.FluxAFreeOrigin,
			newTestFluxAModelsHTTPClient(baseClient),
		),
	}, cfg)
	return service, user
}

func newTestFluxAModelsHTTPClient(base *http.Client) *http.Client {
	client := &http.Client{}
	if base != nil {
		copy := *base
		client = &copy
	}
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	client.Transport = fluxATestTransport{base: transport}
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return client
}
