package feishu

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
}

type APIError struct {
	HTTPStatus int
	APICode    int
	Code       string
	Message    string
	LogID      string
	RawMessage string
}

func (e *APIError) Error() string {
	if strings.TrimSpace(e.Message) != "" {
		return e.Message
	}
	if strings.TrimSpace(e.RawMessage) != "" {
		return e.RawMessage
	}
	if e.HTTPStatus > 0 {
		return fmt.Sprintf("feishu api returned %d", e.HTTPStatus)
	}
	if e.APICode > 0 {
		return fmt.Sprintf("feishu api returned %d", e.APICode)
	}
	return "feishu request failed"
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 90 * time.Second},
		baseURL:    "https://open.feishu.cn/open-apis",
	}
}

func (c *Client) FetchRawPayload(
	ctx context.Context,
	appID, appSecret, entryType, entryToken string,
) (map[string]any, error) {
	token, err := c.tenantAccessToken(ctx, appID, appSecret)
	if err != nil {
		return nil, err
	}

	entryType = strings.TrimSpace(strings.ToLower(entryType))
	if entryType == "" {
		entryType = "docx"
	}
	if entryType == "docx" && TokenLooksLikeWiki(entryToken) {
		entryType = "wiki_node"
	}

	switch entryType {
	case "docx":
		documentID := NormalizeDocxToken(entryToken)
		if documentID == "" {
			return nil, fmt.Errorf("feishu document token is empty")
		}
		return c.fetchDocxPayload(ctx, token, entryType, entryToken, documentID)
	case "wiki_node":
		rootNode, err := c.getWikiNode(ctx, token, NormalizeWikiToken(entryToken))
		if err != nil {
			return nil, err
		}
		docToken, ok := docTokenFromNode(rootNode)
		if !ok {
			return nil, fmt.Errorf("feishu wiki node is not a docx document")
		}
		payload, err := c.fetchDocxPayload(ctx, token, entryType, entryToken, docToken)
		if err != nil {
			return nil, err
		}
		payload["root_node"] = rootNode
		return payload, nil
	case "wiki_space":
		return c.fetchWikiSpacePayload(ctx, token, entryType, entryToken)
	default:
		return nil, fmt.Errorf("unsupported feishu entry type %q", entryType)
	}
}
