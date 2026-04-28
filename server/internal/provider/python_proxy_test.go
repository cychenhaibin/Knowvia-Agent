package provider

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

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/config"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestNormalizeTransportErrorRewritesProxyDialFailures(t *testing.T) {
	client := &PythonForwardClient{
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
	client := &PythonForwardClient{
		baseURL: "http://127.0.0.1:8000",
	}

	original := errors.New("boom")
	if got := client.normalizeTransportError(original); got != original {
		t.Fatalf("expected original error to be returned unchanged")
	}
}

func TestNewPythonForwardClientUsesLongerTimeoutForSync(t *testing.T) {
	client := NewPythonForwardClient(config.Config{
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

func TestPythonForwardClientUsesV1IndexUpsertRoute(t *testing.T) {
	var gotPath string
	var gotPayload indexUpsertBatchRequest

	client := &PythonForwardClient{
		baseURL:     "http://python.test",
		staticToken: "test-token",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected default client request")
				return nil, nil
			}),
		},
		syncClient: &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				gotPath = r.URL.Path
				if r.Method != http.MethodPost {
					t.Fatalf("expected POST, got %s", r.Method)
				}
				if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
					t.Fatalf("decode payload: %v", err)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(
						`{"document_count":1,"chunk_count":2,"index_version":"v1","documents":[]}`,
					)),
					Request: r,
				}, nil
			}),
		},
	}

	_, err := client.UpsertKnowledge(context.Background(), MirroredKnowledgeUpsertRequest{
		UserID:       "u1",
		ConnectionID: "c1",
		ConnectionMeta: MirroredConnection{
			ID:   "c1",
			Name: "产品文档",
		},
		Documents: []MirroredKnowledgeDocument{
			{
				DocID:     "d1",
				Title:     "发布说明",
				Repo:      "产品文档",
				DocRef:    "release-note",
				SourceURL: "https://example.com/release-note",
				UpdatedAt: "2026-04-26T00:00:00Z",
				RawBody:   "body",
			},
		},
	})
	if err != nil {
		t.Fatalf("upsert knowledge: %v", err)
	}

	if gotPath != "/internal/v1/index/upsert-batch" {
		t.Fatalf("expected v1 upsert path, got %s", gotPath)
	}
	if gotPayload.ScopeID != "c1" {
		t.Fatalf("expected scope_id c1, got %q", gotPayload.ScopeID)
	}
	if len(gotPayload.Documents) != 1 || gotPayload.Documents[0].ExternalID != "d1" {
		t.Fatalf("expected transformed document payload, got %+v", gotPayload.Documents)
	}
}

func TestPythonForwardClientUsesV1ChatStreamRoute(t *testing.T) {
	var gotPath string
	var gotPayload ForwardedChatRequest

	client := &PythonForwardClient{
		baseURL:     "http://python.test",
		staticToken: "test-token",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				gotPath = r.URL.Path
				if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
					t.Fatalf("decode payload: %v", err)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
					Body: io.NopCloser(strings.NewReader(
						"data: {\"type\":\"retrieval\",\"sources\":[]}\n\n" +
							"data: {\"type\":\"done\",\"content\":\"ok\",\"sources\":[]}\n\n",
					)),
					Request: r,
				}, nil
			}),
		},
		syncClient: &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected sync client request")
				return nil, nil
			}),
		},
	}

	_, err := client.StreamKnowledgeChat(
		context.Background(),
		ForwardedChatRequest{
			UserID:        "u1",
			Message:       "四月新增了什么",
			ConnectionIDs: []string{"c1", "c2"},
			ModelProfile:  &ForwardedModelProfile{Purpose: "chat_fast", Provider: "ollama", ModelName: "qwen3:8b"},
			Trace:         &ForwardedTraceContext{MessageID: "msg-1", SkillID: "skill-1"},
		},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("stream knowledge chat: %v", err)
	}

	if gotPath != "/internal/v1/chat/stream" {
		t.Fatalf("expected v1 chat path, got %s", gotPath)
	}
	if len(gotPayload.ConnectionIDs) != 2 || gotPayload.ConnectionIDs[0] != "c1" {
		t.Fatalf("expected scope_ids payload, got %+v", gotPayload.ConnectionIDs)
	}
	if gotPayload.ModelProfile == nil || gotPayload.ModelProfile.Provider != "ollama" {
		t.Fatalf("expected model profile to be forwarded, got %+v", gotPayload.ModelProfile)
	}
	if gotPayload.Trace == nil || gotPayload.Trace.MessageID != "msg-1" {
		t.Fatalf("expected trace context to be forwarded, got %+v", gotPayload.Trace)
	}
}

func TestInferForwardedModelProfileForwardsTemperature(t *testing.T) {
	profile := InferForwardedModelProfile(domain.ChatRuntimeConfig{
		BaseURL:     "http://127.0.0.1:11434/v1",
		APIKey:      "ollama",
		ModelName:   "gemma3n:e4b",
		Temperature: 0.05,
	}, "chat_fast")
	if profile == nil {
		t.Fatalf("expected forwarded profile")
	}
	if profile.Provider != "ollama" {
		t.Fatalf("expected ollama provider, got %q", profile.Provider)
	}
	if profile.Temperature != 0.05 {
		t.Fatalf("expected temperature 0.05, got %v", profile.Temperature)
	}
}

func TestPythonForwardClientParsesStructuredStreamErrors(t *testing.T) {
	client := &PythonForwardClient{
		baseURL:     "http://python.test",
		staticToken: "test-token",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
					Body: io.NopCloser(strings.NewReader(
						"data: {\"type\":\"error\",\"traceId\":\"trace-1\",\"error\":{\"code\":\"MODEL_TIMEOUT\",\"message\":\"generation timeout\",\"retryable\":true}}\n\n",
					)),
					Request: r,
				}, nil
			}),
		},
		syncClient: &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected sync client request")
				return nil, nil
			}),
		},
	}

	_, err := client.StreamKnowledgeChat(
		context.Background(),
		ForwardedChatRequest{UserID: "u1", Message: "test"},
		nil,
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "generation timeout") {
		t.Fatalf("expected structured error to surface message, got %v", err)
	}
}

func TestPythonForwardClientUsesV1RetrieveRoute(t *testing.T) {
	var gotPath string
	var gotPayload ForwardedRetrieveRequest

	client := &PythonForwardClient{
		baseURL:     "http://python.test",
		staticToken: "test-token",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				gotPath = r.URL.Path
				if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
					t.Fatalf("decode payload: %v", err)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(
						`{"query":"timeline","sources":[{"type":"knowledge_base","provider":"yuque","connection_id":"c1","document_id":"d1","title":"发布说明","snippet":"新增 timeline","score":1.5}]}`,
					)),
					Request: r,
				}, nil
			}),
		},
		syncClient: &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected sync client request")
				return nil, nil
			}),
		},
	}

	result, err := client.RetrieveKnowledge(
		context.Background(),
		ForwardedRetrieveRequest{
			UserID:   "u1",
			ScopeIDs: []string{"c1", "c2"},
			Query:    "timeline",
			TopK:     4,
			Trace:    &ForwardedTraceContext{RunID: "run-1", SkillID: "skill-1"},
		},
	)
	if err != nil {
		t.Fatalf("retrieve knowledge: %v", err)
	}

	if gotPath != "/internal/v1/retrieve" {
		t.Fatalf("expected v1 retrieve path, got %s", gotPath)
	}
	if gotPayload.TopK != 4 {
		t.Fatalf("expected top_k 4, got %d", gotPayload.TopK)
	}
	if gotPayload.Trace == nil || gotPayload.Trace.RunID != "run-1" {
		t.Fatalf("expected retrieve trace context, got %+v", gotPayload.Trace)
	}
	if len(result.Sources) != 1 || result.Sources[0].ConnectionID != "c1" {
		t.Fatalf("expected retrieval sources, got %+v", result.Sources)
	}
}

func TestPythonForwardClientUsesV1EvidenceMergeRoute(t *testing.T) {
	var gotPath string
	var gotPayload ForwardedEvidenceMergeRequest

	client := &PythonForwardClient{
		baseURL:     "http://python.test",
		staticToken: "test-token",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				gotPath = r.URL.Path
				if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
					t.Fatalf("decode payload: %v", err)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(
						`{"goal":"总结","mode":"kb_only","duplicate_count":1,"group_count":1,"conflict_count":1,"warnings":["conflict"],"evidences":[{"provider":"yuque","connection_id":"c1","title":"发布说明","snippet":"新增 timeline","score":1.1}]}`,
					)),
					Request: r,
				}, nil
			}),
		},
		syncClient: &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected sync client request")
				return nil, nil
			}),
		},
	}

	result, err := client.MergeEvidence(
		context.Background(),
		ForwardedEvidenceMergeRequest{
			UserID: "u1",
			Goal:   "总结",
			Mode:   "kb_only",
			Evidences: []ForwardedEvidence{
				{Provider: "yuque", ConnectionID: "c1", Title: "发布说明", Snippet: "新增 timeline"},
			},
			TopK: 3,
		},
	)
	if err != nil {
		t.Fatalf("merge evidence: %v", err)
	}

	if gotPath != "/internal/v1/evidence/merge" {
		t.Fatalf("expected v1 evidence merge path, got %s", gotPath)
	}
	if gotPayload.TopK != 3 {
		t.Fatalf("expected top_k 3, got %d", gotPayload.TopK)
	}
	if len(result.Evidences) != 1 || result.Evidences[0].ConnectionID != "c1" {
		t.Fatalf("expected merged evidences, got %+v", result.Evidences)
	}
	if result.ConflictCount != 1 || result.DuplicateCount != 1 {
		t.Fatalf("expected merge diagnostics, got %+v", result)
	}
}

func TestPythonForwardClientUsesV1ReportRoute(t *testing.T) {
	var gotPath string
	var gotPayload ForwardedReportRequest

	client := &PythonForwardClient{
		baseURL:     "http://python.test",
		staticToken: "test-token",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected default client request")
				return nil, nil
			}),
		},
		syncClient: &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				gotPath = r.URL.Path
				if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
					t.Fatalf("decode payload: %v", err)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(
						`{"summary":"summary from llm","outline_markdown":"# Outline","draft_markdown":"# Draft","retrieval_markdown":"# Grounding Trace","report_markdown":"# Report","sources":[]}`,
					)),
					Request: r,
				}, nil
			}),
		},
	}

	result, err := client.GenerateReport(
		context.Background(),
		ForwardedReportRequest{
			UserID:   "u1",
			Goal:     "总结",
			ScopeIDs: []string{"c1"},
			Mode:     "kb_only",
			ModelProfile: &ForwardedModelProfile{
				Purpose:   "report_writer",
				Provider:  "openai_compatible",
				ModelName: "gpt-test",
			},
			Trace: &ForwardedTraceContext{RunID: "run-1", SkillID: "skill-1"},
			Evidences: []ForwardedEvidence{
				{Provider: "yuque", ConnectionID: "c1", Title: "发布说明", Snippet: "新增 timeline"},
			},
			EvidenceDiagnostics: &ForwardedEvidenceDiagnostics{
				DuplicateCount: 1,
				GroupCount:     1,
				ConflictCount:  1,
				GroupLabels:    []string{"发布说明"},
				Warnings:       []string{"conflict"},
			},
		},
	)
	if err != nil {
		t.Fatalf("generate report: %v", err)
	}

	if gotPath != "/internal/v1/report/generate" {
		t.Fatalf("expected v1 report path, got %s", gotPath)
	}
	if gotPayload.EvidenceDiagnostics == nil || gotPayload.EvidenceDiagnostics.ConflictCount != 1 {
		t.Fatalf("expected report diagnostics to be forwarded, got %+v", gotPayload.EvidenceDiagnostics)
	}
	if gotPayload.ModelProfile == nil || gotPayload.ModelProfile.Purpose != "report_writer" {
		t.Fatalf("expected report model profile to be forwarded, got %+v", gotPayload.ModelProfile)
	}
	if gotPayload.Trace == nil || gotPayload.Trace.RunID != "run-1" {
		t.Fatalf("expected report trace context to be forwarded, got %+v", gotPayload.Trace)
	}
	if result.OutlineMarkdown != "# Outline" || result.DraftMarkdown != "# Draft" || result.RetrievalMarkdown != "# Grounding Trace" || result.ReportMarkdown != "# Report" {
		t.Fatalf("expected staged report response, got %+v", result)
	}
}

func TestPythonForwardClientUsesV1SkillRoute(t *testing.T) {
	var gotPath string

	client := &PythonForwardClient{
		baseURL:     "http://python.test",
		staticToken: "test-token",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				gotPath = r.URL.Path
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("")),
					Request:    r,
				}, nil
			}),
		},
		syncClient: &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected sync client request")
				return nil, nil
			}),
		},
	}

	err := client.UpsertSkill(context.Background(), domain.Skill{ID: "s1", UserID: "u1"})
	if err != nil {
		t.Fatalf("upsert skill: %v", err)
	}
	if gotPath != "/internal/v1/skills/upsert" {
		t.Fatalf("expected v1 skill path, got %s", gotPath)
	}
}
