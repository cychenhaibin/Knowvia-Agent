package feishu

import (
	"context"
	"fmt"
	"net/http"
	neturl "net/url"
)

func (c *Client) fetchDocxPayload(
	ctx context.Context,
	token, entryType, entryToken, documentID string,
) (map[string]any, error) {
	metadata, err := c.getDocumentMetadata(ctx, token, documentID)
	if err != nil {
		return nil, err
	}
	rawContent, err := c.getDocumentRawContent(ctx, token, documentID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"entry_type":        entryType,
		"entry_token":       entryToken,
		"document_metadata": map[string]any{documentID: metadata},
		"raw_contents":      map[string]any{documentID: rawContent},
	}, nil
}

func (c *Client) getDocumentMetadata(ctx context.Context, token, documentID string) (map[string]any, error) {
	url := fmt.Sprintf("%s/docx/v1/documents/%s", c.baseURL, neturl.PathEscape(documentID))
	var payload map[string]any
	if err := c.doJSON(ctx, http.MethodGet, url, token, nil, nil, &payload); err != nil {
		return nil, err
	}
	data, _ := payload["data"].(map[string]any)
	if data == nil {
		return map[string]any{}, nil
	}
	if document, ok := data["document"].(map[string]any); ok && document != nil {
		return document, nil
	}
	return data, nil
}

func (c *Client) getDocumentRawContent(ctx context.Context, token, documentID string) (map[string]any, error) {
	url := fmt.Sprintf("%s/docx/v1/documents/%s/raw_content", c.baseURL, neturl.PathEscape(documentID))
	var payload map[string]any
	if err := c.doJSON(ctx, http.MethodGet, url, token, nil, nil, &payload); err != nil {
		return nil, err
	}
	data, _ := payload["data"].(map[string]any)
	if data == nil {
		return map[string]any{}, nil
	}
	return data, nil
}
