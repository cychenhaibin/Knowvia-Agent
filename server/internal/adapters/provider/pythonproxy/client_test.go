package pythonproxy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	neturl "net/url"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/provider"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestNormalizeTransportErrorRewritesProxyDialFailures(t *testing.T) {
	client := &Client{
		baseURL: "http://127.0.0.1:8000",
	}

	err := client.normalizeTransportError(&neturl.Error{
		Op:  "Post",
		URL: "http://127.0.0.1:8000/auth/login",
		Err: &net.OpError{
			Op:  "dial",
			Net: "tcp",
			Err: syscall.ECONNREFUSED,
		},
	})

	message := err.Error()
	if !strings.Contains(message, "python backend unavailable at http://127.0.0.1:8000") {
		t.Fatalf("expected rewritten proxy error, got %q", message)
	}
	if !strings.Contains(message, "quickque-agent/llm/run_server.sh") {
		t.Fatalf("expected startup hint, got %q", message)
	}
}

func TestNormalizeTransportErrorLeavesNonTransportErrorsUntouched(t *testing.T) {
	client := &Client{
		baseURL: "http://127.0.0.1:8000",
	}

	original := errors.New("boom")
	if got := client.normalizeTransportError(original); got != original {
		t.Fatalf("expected original error to be returned unchanged")
	}
}

func TestNewUsesLongerTimeoutForSync(t *testing.T) {
	client := New(config.Config{
		PythonProxyBaseURL: "http://127.0.0.1:8000",
	})
	if client == nil {
		t.Fatalf("expected client")
	}
	if client.httpClient.Timeout != 90*time.Second {
		t.Fatalf("expected default request timeout 90s, got %s", client.httpClient.Timeout)
	}
	if client.syncClient.Timeout != 10*time.Minute {
		t.Fatalf("expected sync timeout 10m, got %s", client.syncClient.Timeout)
	}
}

func TestRetrieveKnowledgeBuildsExpectedRequest(t *testing.T) {
	var captured provider.ForwardedRetrieveRequest
	client := &Client{
		baseURL:     "http://proxy.example",
		staticToken: "token-123",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPost {
					t.Fatalf("method = %s, want POST", req.Method)
				}
				if req.URL.Path != "/internal/v1/retrieve" {
					t.Fatalf("path = %s, want /internal/v1/retrieve", req.URL.Path)
				}
				if got := req.Header.Get("Authorization"); got != "Bearer token-123" {
					t.Fatalf("authorization = %q", got)
				}
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("read body: %v", err)
				}
				if err := json.Unmarshal(body, &captured); err != nil {
					t.Fatalf("decode request: %v", err)
				}
				return jsonResponse(t, http.StatusOK, provider.ForwardedRetrieveResult{
					Query: "search docs",
					Sources: []provider.ForwardedSource{
						{Title: "Doc A"},
					},
				}), nil
			}),
		},
		syncClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatalf("sync client should not be used for retrieve")
			return nil, nil
		})},
	}

	result, err := client.RetrieveKnowledge(context.Background(), provider.ForwardedRetrieveRequest{
		UserID:   "user-1",
		ScopeIDs: []string{"conn-1"},
		Query:    "search docs",
		TopK:     3,
	})
	if err != nil {
		t.Fatalf("retrieve knowledge: %v", err)
	}
	if captured.UserID != "user-1" || captured.Query != "search docs" || len(captured.ScopeIDs) != 1 {
		t.Fatalf("unexpected captured request: %#v", captured)
	}
	if result.Query != "search docs" || len(result.Sources) != 1 || result.Sources[0].Title != "Doc A" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestSyncKnowledgeSourceUsesSyncClient(t *testing.T) {
	var usedSyncClient bool
	client := &Client{
		baseURL:     "http://proxy.example",
		staticToken: "token-123",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				t.Fatalf("default client should not be used for sync requests")
				return nil, nil
			}),
		},
		syncClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				usedSyncClient = true
				if req.Method != http.MethodPost {
					t.Fatalf("method = %s, want POST", req.Method)
				}
				if req.URL.Path != "/internal/v1/index/sync-source" {
					t.Fatalf("path = %s, want /internal/v1/index/sync-source", req.URL.Path)
				}
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("read body: %v", err)
				}
				if !strings.Contains(string(body), "\"scope_id\":\"conn-1\"") {
					t.Fatalf("unexpected request body: %s", string(body))
				}
				return jsonResponse(t, http.StatusOK, provider.SyncedKnowledgeSourceResult{
					DocumentCount: 2,
					ChunkCount:    5,
				}), nil
			}),
		},
	}

	result, err := client.SyncKnowledgeSource(context.Background(), provider.SyncedKnowledgeSourceRequest{
		UserID:       "user-1",
		ConnectionID: "conn-1",
		Name:         "Yuque",
		Provider:     "yuque",
	})
	if err != nil {
		t.Fatalf("sync knowledge source: %v", err)
	}
	if !usedSyncClient {
		t.Fatalf("expected sync client to be used")
	}
	if result.DocumentCount != 2 || result.ChunkCount != 5 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func jsonResponse(t *testing.T, status int, payload any) *http.Response {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal response payload: %v", err)
	}
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(string(body))),
	}
}
