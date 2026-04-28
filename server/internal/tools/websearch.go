package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

type DuckDuckGoSearchTool struct {
	httpClient *http.Client
}

func NewDuckDuckGoSearchTool() *DuckDuckGoSearchTool {
	return &DuckDuckGoSearchTool{
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
}

func (t *DuckDuckGoSearchTool) Search(ctx context.Context, query string, limit int) ([]domain.Evidence, error) {
	endpoint := "https://api.duckduckgo.com/?format=json&no_redirect=1&no_html=1&q=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var payload struct {
		AbstractText  string `json:"AbstractText"`
		AbstractURL   string `json:"AbstractURL"`
		RelatedTopics []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"RelatedTopics"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	results := []domain.Evidence{}
	if payload.AbstractText != "" {
		results = append(results, domain.Evidence{
			Provider: domain.ProviderWeb,
			Title:    summarizeTitle(query),
			URL:      payload.AbstractURL,
			Snippet:  payload.AbstractText,
			Score:    1,
		})
	}
	for _, topic := range payload.RelatedTopics {
		if topic.Text == "" || topic.FirstURL == "" {
			continue
		}
		results = append(results, domain.Evidence{
			Provider: domain.ProviderWeb,
			Title:    summarizeTitle(topic.Text),
			URL:      topic.FirstURL,
			Snippet:  topic.Text,
			Score:    0.8,
		})
		if limit > 0 && len(results) >= limit {
			break
		}
	}
	return results, nil
}

func summarizeTitle(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "Web Result"
	}
	runes := []rune(raw)
	if len(runes) <= 48 {
		return raw
	}
	return string(runes[:48]) + "..."
}
