package tools

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/knowledge"
)

type WebPageExtractTool struct {
	httpClient *http.Client
}

func NewWebPageExtractTool() *WebPageExtractTool {
	return &WebPageExtractTool{
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
}

func (t *WebPageExtractTool) Extract(ctx context.Context, evidences []domain.Evidence, limit int) ([]domain.Evidence, error) {
	extracted := []domain.Evidence{}
	for _, evidence := range evidences {
		if evidence.URL == "" {
			continue
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, evidence.URL, nil)
		if err != nil {
			continue
		}
		resp, err := t.httpClient.Do(req)
		if err != nil {
			continue
		}
		bodyBytes := make([]byte, 4096)
		n, _ := resp.Body.Read(bodyBytes)
		resp.Body.Close()
		if n == 0 {
			continue
		}
		body := knowledge.NormalizeBody(string(bodyBytes[:n]))
		body = strings.TrimSpace(body)
		if body == "" {
			continue
		}
		evidence.Body = body
		evidence.Snippet = summarizeTitle(body)
		extracted = append(extracted, evidence)
		if limit > 0 && len(extracted) >= limit {
			break
		}
	}
	return extracted, nil
}
