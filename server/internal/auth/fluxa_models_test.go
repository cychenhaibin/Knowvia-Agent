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
		if r.URL.Path == "/api/token/" {
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
	if !strings.Contains(logs.String(), "fluxa_model_groups_upstream_failure") || !strings.Contains(logs.String(), "route=/api/token/?p=1&size=20") || !strings.Contains(logs.String(), "status=502") {
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
		case "/api/token/":
			_, _ = w.Write([]byte(`{"success":true,"data":{"items":[{"id":1,"group":"default"}]}}`))
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
	requests := make([]string, 0, 2)
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
		case "/api/token/":
			if r.URL.Query().Get("p") != "1" || r.URL.Query().Get("size") != "20" {
				t.Fatalf("unexpected token query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"success":true,"data":{"items":[{"id":1,"name":"Research key","group":"research"},{"id":2,"name":"Default key","group":"default"},{"id":3,"name":"Research key","group":"research"},{"id":4,"name":"User key","group":""}]}}`))
		case "/api/user/self/groups":
			_, _ = w.Write([]byte(`{"success":true,"data":{"default":{"desc":"Default models","ratio":0.1},"research":{"desc":"Research models","ratio":0.3},"unused":{"desc":"Unused","ratio":1}}}`))
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
		{ID: "1", Name: "Research key", Group: "research", Desc: "Research models", Ratio: 0.3},
		{ID: "2", Name: "Default key", Group: "default", Desc: "Default models", Ratio: 0.1},
		{ID: "3", Name: "Research key", Group: "research", Desc: "Research models", Ratio: 0.3},
		{ID: "4", Name: "User key"},
	}
	if !reflect.DeepEqual(groups, want) {
		t.Fatalf("groups = %#v, want %#v", groups, want)
	}
	if recordedAuthorization != "Bearer upstream-token" {
		t.Fatalf("authorization = %q, want upstream bearer token", recordedAuthorization)
	}
	if !reflect.DeepEqual(requests, []string{"/api/token/", "/api/user/self/groups"}) {
		t.Fatalf("requests = %#v, want token and group endpoints", requests)
	}
}

func TestListFluxAModelsUsesGroupQueryAndNormalizesNames(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/user/models" || r.URL.Query().Get("group") != "gpt 专用" {
			t.Fatalf("request = %s?%s, want group query", r.URL.Path, r.URL.RawQuery)
		}
		if r.Header.Get("Authorization") != "Bearer upstream-token" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":["gpt-4o"," ","claude-3-5-sonnet"]}`))
	}))
	t.Cleanup(server.Close)

	service, user := newFluxAModelGroupsService(t, server.URL, server.URL, server.Client())
	models, err := service.ListFluxAModels(context.Background(), user.ID, FluxASitePaid, "gpt 专用")
	if err != nil {
		t.Fatalf("list models: %v", err)
	}
	want := []FluxAModel{{ID: "gpt-4o", Name: "gpt-4o"}, {ID: "claude-3-5-sonnet", Name: "claude-3-5-sonnet"}}
	if !reflect.DeepEqual(models, want) {
		t.Fatalf("models = %#v, want %#v", models, want)
	}
}

func TestListFluxAModelGroupsAcceptsSingleAccountGroupString(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/user/self/groups":
			_, _ = w.Write([]byte(`{"success":true,"data":{"groups":[{"name":"premium","models":[{"id":"model-a","name":"Model A"}]}]}}`))
		case "/api/token/":
			_, _ = w.Write([]byte(`{"success":true,"data":{"items":[{"id":1,"name":"Premium key","group":"premium"}]}}`))
		}
	}))
	t.Cleanup(server.Close)

	service, user := newFluxAModelGroupsService(t, server.URL, server.URL, server.Client())
	groups, err := service.ListFluxAModelGroups(context.Background(), user.ID, FluxASitePaid)
	if err != nil {
		t.Fatalf("list groups: %v", err)
	}
	want := []FluxAModelGroup{{ID: "1", Name: "Premium key", Group: "premium", Models: []FluxAModel{{ID: "model-a", Name: "Model A"}}}}
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
		case "/api/token/":
			_, _ = w.Write([]byte(`{"success":true,"data":{"items":[{"id":1,"name":"GPT key","group":"11"},{"id":2,"name":"Claude key","group":"14"}]}}`))
		}
	}))
	t.Cleanup(server.Close)
	service, user := newFluxAModelGroupsService(t, server.URL, server.URL, server.Client())
	groups, err := service.ListFluxAModelGroups(context.Background(), user.ID, FluxASitePaid)
	if err != nil {
		t.Fatalf("list groups: %v", err)
	}
	want := []FluxAModelGroup{
		{ID: "1", Name: "GPT key", Group: "11", Models: []FluxAModel{{ID: "gpt-4o", Name: "gpt-4o"}}},
		{ID: "2", Name: "Claude key", Group: "14", Models: []FluxAModel{{ID: "claude-3", Name: "claude-3"}}},
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
		} else if r.URL.Path == "/api/token/" {
			_, _ = w.Write([]byte(`{"success":true,"data":{"items":[{"id":1,"name":"Comet","group":"cc_max"},{"id":2,"name":"Test","group":"公益组"}]}}`))
		}
	}))
	t.Cleanup(server.Close)
	service, user := newFluxAModelGroupsService(t, server.URL, server.URL, server.Client())
	groups, err := service.ListFluxAModelGroups(context.Background(), user.ID, FluxASitePaid)
	if err != nil {
		t.Fatalf("list groups: %v", err)
	}
	want := []FluxAModelGroup{{ID: "1", Name: "Comet", Group: "cc_max", Desc: "claude code max分组", Ratio: 0.3}, {ID: "2", Name: "Test", Group: "公益组", Ratio: 0.001}}
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
