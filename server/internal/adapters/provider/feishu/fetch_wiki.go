package feishu

import (
	"context"
	"fmt"
	"net/http"
	neturl "net/url"
	"strings"
)

func (c *Client) fetchWikiSpacePayload(
	ctx context.Context,
	token, entryType, entryToken string,
) (map[string]any, error) {
	rootNode, err := c.getWikiNode(ctx, token, NormalizeWikiToken(entryToken))
	if err != nil {
		return nil, err
	}
	spaceID := strings.TrimSpace(stringValue(rootNode, "space_id", "spaceId"))
	rootNodeToken := strings.TrimSpace(stringValue(rootNode, "node_token", "nodeToken"))
	if spaceID == "" {
		return nil, fmt.Errorf("feishu wiki space_id is missing")
	}

	queue := []string{""}
	seenQueue := map[string]struct{}{"": {}}
	seenNodes := map[string]struct{}{}
	seenDocs := map[string]struct{}{}
	spaceNodes := make([]map[string]any, 0, 32)
	resolvedNodes := map[string]any{}
	documentMetadata := map[string]any{}
	rawContents := map[string]any{}

	listAndCollect := func(parentNodeToken string) error {
		pageToken := ""
		for {
			page, err := c.listWikiSpaceNodes(ctx, token, spaceID, parentNodeToken, pageToken, 50)
			if err != nil {
				return err
			}
			items, _ := page["items"].([]any)
			for _, item := range items {
				node, ok := item.(map[string]any)
				if !ok {
					continue
				}
				nodeToken := strings.TrimSpace(stringValue(node, "node_token", "nodeToken"))
				if nodeToken != "" {
					if _, exists := seenNodes[nodeToken]; exists {
						continue
					}
					seenNodes[nodeToken] = struct{}{}
				}
				spaceNodes = append(spaceNodes, node)
				if truthy(node["has_child"]) || truthy(node["hasChild"]) {
					if nodeToken != "" {
						if _, exists := seenQueue[nodeToken]; !exists {
							queue = append(queue, nodeToken)
							seenQueue[nodeToken] = struct{}{}
						}
					}
				}

				resolvedNode := node
				docToken, ok := docTokenFromNode(resolvedNode)
				if !ok && nodeToken != "" {
					fetched, err := c.getWikiNode(ctx, token, nodeToken)
					if err != nil {
						return err
					}
					resolvedNode = fetched
					resolvedNodes[nodeToken] = fetched
					docToken, ok = docTokenFromNode(resolvedNode)
				}
				if !ok || docToken == "" {
					continue
				}
				if _, exists := seenDocs[docToken]; exists {
					continue
				}
				seenDocs[docToken] = struct{}{}

				meta, err := c.getDocumentMetadata(ctx, token, docToken)
				if err != nil {
					return err
				}
				raw, err := c.getDocumentRawContent(ctx, token, docToken)
				if err != nil {
					return err
				}
				documentMetadata[docToken] = meta
				rawContents[docToken] = raw
			}

			pageToken = strings.TrimSpace(stringValue(page, "page_token", "pageToken"))
			if !truthy(page["has_more"]) && !truthy(page["hasMore"]) {
				break
			}
			if pageToken == "" {
				break
			}
		}
		return nil
	}

	if err := listAndCollect(""); err != nil {
		return nil, err
	}
	if len(spaceNodes) == 0 && rootNodeToken != "" {
		queue = append(queue, rootNodeToken)
		seenQueue[rootNodeToken] = struct{}{}
	}
	for len(queue) > 0 {
		parent := queue[0]
		queue = queue[1:]
		if parent == "" {
			continue
		}
		if err := listAndCollect(parent); err != nil {
			return nil, err
		}
	}

	return map[string]any{
		"entry_type":        entryType,
		"entry_token":       entryToken,
		"space_id":          spaceID,
		"root_node":         rootNode,
		"space_nodes":       spaceNodes,
		"resolved_nodes":    resolvedNodes,
		"document_metadata": documentMetadata,
		"raw_contents":      rawContents,
	}, nil
}

func (c *Client) getWikiNode(ctx context.Context, token, wikiToken string) (map[string]any, error) {
	url := c.baseURL + "/wiki/v2/spaces/get_node"
	params := neturl.Values{}
	params.Set("token", wikiToken)
	var payload map[string]any
	if err := c.doJSON(ctx, http.MethodGet, url, token, params, nil, &payload); err != nil {
		return nil, err
	}
	data, _ := payload["data"].(map[string]any)
	if data == nil {
		return nil, fmt.Errorf("feishu wiki node response is empty")
	}
	if node, ok := data["node"].(map[string]any); ok && node != nil {
		return node, nil
	}
	return data, nil
}

func (c *Client) listWikiSpaceNodes(
	ctx context.Context,
	token, spaceID, parentNodeToken, pageToken string,
	pageSize int,
) (map[string]any, error) {
	url := fmt.Sprintf("%s/wiki/v2/spaces/%s/nodes", c.baseURL, neturl.PathEscape(spaceID))
	params := neturl.Values{}
	params.Set("page_size", fmt.Sprintf("%d", pageSize))
	if strings.TrimSpace(parentNodeToken) != "" {
		params.Set("parent_node_token", parentNodeToken)
	}
	if strings.TrimSpace(pageToken) != "" {
		params.Set("page_token", pageToken)
	}
	var payload map[string]any
	if err := c.doJSON(ctx, http.MethodGet, url, token, params, nil, &payload); err != nil {
		return nil, err
	}
	data, _ := payload["data"].(map[string]any)
	if data == nil {
		return map[string]any{"items": []any{}}, nil
	}
	return data, nil
}
