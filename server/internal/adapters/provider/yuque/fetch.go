package yuque

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
)

func (c *Client) ListDocs(ctx context.Context, token, namespace string) ([]DocMeta, error) {
	var response struct {
		Data []DocMeta `json:"data"`
	}
	if err := c.get(ctx, token, fmt.Sprintf("%s/repos/%s/docs", c.baseURL, namespace), &response); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (c *Client) GetDocBody(ctx context.Context, token, namespace, slug string) (string, error) {
	var response struct {
		Data struct {
			Body      string `json:"body"`
			UpdatedAt string `json:"updated_at"`
		} `json:"data"`
	}
	endpoint := fmt.Sprintf("%s/repos/%s/docs/%s", c.baseURL, namespace, url.PathEscape(slug))
	if err := c.get(ctx, token, endpoint, &response); err != nil {
		return "", err
	}
	return response.Data.Body, nil
}

func (c *Client) FetchRawPayload(
	ctx context.Context,
	token, groupLogin, namespace string,
) (map[string]any, error) {
	groupLogin = strings.TrimSpace(groupLogin)
	namespace = strings.Trim(strings.TrimSpace(namespace), "/")
	if token == "" {
		return nil, fmt.Errorf("yuque token is required")
	}
	if groupLogin == "" || namespace == "" {
		return nil, fmt.Errorf("yuque groupLogin and namespace are required")
	}

	heldGates := make([]struct {
		key  string
		lock *sync.Mutex
	}, 0, 2)
	for _, gateKey := range yuqueSyncKeys(token, groupLogin, namespace) {
		heldGates = append(heldGates, struct {
			key  string
			lock *sync.Mutex
		}{
			key:  gateKey,
			lock: acquireYuqueGate(gateKey),
		})
	}
	defer func() {
		for idx := len(heldGates) - 1; idx >= 0; idx-- {
			releaseYuqueGate(heldGates[idx].key, heldGates[idx].lock)
		}
	}()

	docsByNamespace := map[string]any{}
	bodiesByNamespace := map[string]any{}
	docs, err := c.ListDocs(ctx, token, namespace)
	if err != nil {
		return nil, err
	}
	docsByNamespace[namespace] = docs
	bodyMap := map[string]string{}
	for _, doc := range docs {
		slug := strings.TrimSpace(doc.Slug)
		if slug == "" {
			continue
		}
		body, err := c.GetDocBody(ctx, token, namespace, slug)
		if err != nil {
			return nil, err
		}
		bodyMap[slug] = body
	}
	bodiesByNamespace[namespace] = bodyMap

	return map[string]any{
		"group_login":         groupLogin,
		"namespace":           namespace,
		"repos":               []map[string]string{{"namespace": namespace}},
		"docs_by_namespace":   docsByNamespace,
		"bodies_by_namespace": bodiesByNamespace,
	}, nil
}
