package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	neturl "net/url"
	"strings"
	"time"
)

type feishuClient struct {
	httpClient *http.Client
	baseURL    string
}

type feishuAPIError struct {
	HTTPStatus int
	APICode    int
	Code       string
	Message    string
	LogID      string
	RawMessage string
}

func (e *feishuAPIError) Error() string {
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

func newFeishuClient() *feishuClient {
	return &feishuClient{
		httpClient: &http.Client{Timeout: 90 * time.Second},
		baseURL:    "https://open.feishu.cn/open-apis",
	}
}

func (c *feishuClient) FetchRawPayload(
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
	if entryType == "docx" && tokenLooksLikeFeishuWiki(entryToken) {
		entryType = "wiki_node"
	}

	switch entryType {
	case "docx":
		documentID := normalizeFeishuDocxToken(entryToken)
		if documentID == "" {
			return nil, fmt.Errorf("feishu document token is empty")
		}
		return c.fetchDocxPayload(ctx, token, entryType, entryToken, documentID)
	case "wiki_node":
		rootNode, err := c.getWikiNode(ctx, token, normalizeFeishuWikiToken(entryToken))
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

func tokenLooksLikeFeishuWiki(value string) bool {
	candidate := strings.TrimSpace(value)
	if candidate == "" {
		return false
	}
	if strings.HasPrefix(candidate, "wiki:") {
		return true
	}
	if !strings.Contains(candidate, "://") {
		return false
	}
	parsed, err := neturl.Parse(candidate)
	if err != nil {
		return false
	}
	if queryValue := strings.TrimSpace(parsed.Query().Get("wiki")); queryValue != "" {
		return true
	}
	segments := [][2]string{}
	pathSegments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for index, segment := range pathSegments {
		if index+1 >= len(pathSegments) {
			continue
		}
		segments = append(segments, [2]string{segment, pathSegments[index+1]})
	}
	for _, segment := range segments {
		if strings.TrimSpace(segment[0]) == "wiki" && strings.TrimSpace(segment[1]) != "" {
			return true
		}
	}
	return false
}

func (c *feishuClient) tenantAccessToken(ctx context.Context, appID, appSecret string) (string, error) {
	url := c.baseURL + "/auth/v3/tenant_access_token/internal"
	reqBody := map[string]string{
		"app_id":     appID,
		"app_secret": appSecret,
	}
	var payload struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
	}
	if err := c.doJSON(ctx, http.MethodPost, url, "", nil, reqBody, &payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.TenantAccessToken) == "" {
		return "", fmt.Errorf("feishu tenant_access_token missing in response")
	}
	return strings.TrimSpace(payload.TenantAccessToken), nil
}

func (c *feishuClient) fetchDocxPayload(
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

func (c *feishuClient) fetchWikiSpacePayload(
	ctx context.Context,
	token, entryType, entryToken string,
) (map[string]any, error) {
	rootNode, err := c.getWikiNode(ctx, token, normalizeFeishuWikiToken(entryToken))
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

func (c *feishuClient) getWikiNode(ctx context.Context, token, wikiToken string) (map[string]any, error) {
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

func (c *feishuClient) listWikiSpaceNodes(
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

func (c *feishuClient) getDocumentMetadata(ctx context.Context, token, documentID string) (map[string]any, error) {
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

func (c *feishuClient) getDocumentRawContent(ctx context.Context, token, documentID string) (map[string]any, error) {
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

func (c *feishuClient) doJSON(
	ctx context.Context,
	method, url, bearer string,
	params neturl.Values,
	body any,
	target any,
) error {
	var reqBody []byte
	var err error
	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	if len(params) > 0 {
		url += "?" + params.Encode()
	}
	log.Printf(
		"feishu http request method=%s url=%s query=%s body=%s bearer=%s",
		method,
		url,
		redactQuery(params),
		redactBody(reqBody),
		redactBearer(bearer),
	)
	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(string(reqBody)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("feishu http transport error method=%s url=%s error=%v", method, url, err)
		return normalizeFeishuTransportError(err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("feishu http read body error method=%s url=%s error=%v", method, url, err)
		return normalizeFeishuTransportError(err)
	}
	log.Printf(
		"feishu http response method=%s url=%s status=%d body=%s",
		method,
		url,
		resp.StatusCode,
		redactBody(bodyBytes),
	)

	var payload map[string]any
	if len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			return err
		}
	}
	if payload == nil {
		payload = map[string]any{}
	}
	if resp.StatusCode >= 300 {
		return normalizeFeishuAPIError(resp.StatusCode, payload)
	}
	if code, ok := payload["code"].(float64); ok && int(code) != 0 {
		return normalizeFeishuAPIError(resp.StatusCode, payload)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

func normalizeFeishuDocxToken(value string) string {
	candidate := strings.TrimSpace(value)
	if !strings.Contains(candidate, "://") {
		return candidate
	}
	parsed, err := neturl.Parse(candidate)
	if err != nil {
		return candidate
	}
	if documentID := parsed.Query().Get("document_id"); strings.TrimSpace(documentID) != "" {
		return strings.TrimSpace(documentID)
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for idx, segment := range segments {
		if segment == "docx" && idx+1 < len(segments) {
			if token := strings.TrimSpace(segments[idx+1]); token != "" {
				return token
			}
		}
	}
	return candidate
}

func normalizeFeishuWikiToken(value string) string {
	candidate := strings.TrimSpace(value)
	if strings.HasPrefix(candidate, "wiki:") {
		candidate = strings.TrimSpace(strings.TrimPrefix(candidate, "wiki:"))
	}
	if !strings.Contains(candidate, "://") {
		return candidate
	}
	parsed, err := neturl.Parse(candidate)
	if err != nil {
		return candidate
	}
	if wiki := parsed.Query().Get("wiki"); strings.TrimSpace(wiki) != "" {
		return strings.TrimSpace(wiki)
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for idx, segment := range segments {
		if segment == "wiki" && idx+1 < len(segments) {
			if token := strings.TrimSpace(segments[idx+1]); token != "" {
				return token
			}
		}
	}
	return candidate
}

func docTokenFromNode(node map[string]any) (string, bool) {
	objType := strings.TrimSpace(strings.ToLower(stringValue(node, "obj_type", "objType")))
	objToken := strings.TrimSpace(stringValue(node, "obj_token", "objToken"))
	if (objType == "doc" || objType == "docx") && objToken != "" {
		return objToken, true
	}
	return "", false
}

func stringValue(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := m[key]; ok {
			switch typed := value.(type) {
			case string:
				if strings.TrimSpace(typed) != "" {
					return typed
				}
			case fmt.Stringer:
				text := typed.String()
				if strings.TrimSpace(text) != "" {
					return text
				}
			case float64:
				return fmt.Sprintf("%.0f", typed)
			}
		}
	}
	return ""
}

func truthy(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	case float64:
		return typed != 0
	default:
		return false
	}
}

func redactBearer(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "***" + token[len(token)-4:]
}

func normalizeFeishuTransportError(err error) error {
	if err == nil {
		return nil
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return &feishuAPIError{
			Code:       "network_timeout",
			Message:    "飞书同步超时，这个知识库可能比较大，或接口响应较慢。请稍后重试。",
			RawMessage: err.Error(),
		}
	}

	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "connection reset by peer") ||
		strings.Contains(lower, "proxyerror") ||
		strings.Contains(lower, "ssl") ||
		strings.Contains(lower, "eof occurred in violation of protocol") {
		return &feishuAPIError{
			Code:       "network_error",
			Message:    "飞书网络请求失败，请检查网络环境或代理设置后重试。",
			RawMessage: err.Error(),
		}
	}

	return &feishuAPIError{
		Code:       "network_error",
		Message:    "飞书请求失败，请检查网络、应用权限或文档访问范围后重试。",
		RawMessage: err.Error(),
	}
}

func normalizeFeishuAPIError(httpStatus int, payload map[string]any) error {
	apiCode := intNumber(payload["code"])
	rawMessage := strings.TrimSpace(stringValue(payload, "msg", "message"))
	logID := strings.TrimSpace(findNestedString(payload, "log_id", "logId"))
	lowerMessage := strings.ToLower(rawMessage)

	code := "api_error"
	message := rawMessage

	switch {
	case strings.Contains(lowerMessage, "wiki space permission denied") ||
		(strings.Contains(lowerMessage, "tenant needs read permission") && strings.Contains(lowerMessage, "wiki")):
		code = "wiki_space_permission_denied"
		message = "飞书知识空间暂无读取权限，请先将当前应用加入该知识空间成员或授予租户读取权限后重试。"
	case strings.Contains(lowerMessage, "node permission denied"):
		code = "wiki_node_permission_denied"
		message = "飞书文档节点暂无读取权限，请确认当前应用对该节点具备读取权限后重试。"
	case strings.Contains(lowerMessage, "no source parent node permission"):
		code = "source_parent_node_permission_denied"
		message = "飞书原父节点容器暂无访问权限，请确认当前应用具备原父节点容器的编辑权限后重试。"
	case strings.Contains(lowerMessage, "no destination parent node permission"):
		code = "destination_parent_node_permission_denied"
		message = "飞书目标父节点容器暂无访问权限，请确认当前应用具备目标父节点容器的编辑权限后重试。"
	case strings.Contains(lowerMessage, "field validation failed"):
		code = "field_validation_failed"
		message = "飞书请求参数校验失败，请检查知识空间链接、节点类型或文档 token 是否匹配。"
	case strings.Contains(lowerMessage, "permission denied"):
		code = "permission_denied"
		message = "飞书应用缺少读取权限，请检查应用 scope 和知识空间访问范围后重试。"
	case apiCode == 131006:
		code = "wiki_space_permission_denied"
		message = "飞书知识空间暂无读取权限，请先将当前应用加入该知识空间成员或授予租户读取权限后重试。"
	case apiCode == 99991672:
		code = "permission_denied"
		message = "飞书应用缺少读取权限，请检查应用 scope 和知识空间访问范围后重试。"
	}

	if strings.TrimSpace(message) == "" {
		message = "飞书请求失败，请稍后重试。"
	}

	return &feishuAPIError{
		HTTPStatus: httpStatus,
		APICode:    apiCode,
		Code:       code,
		Message:    message,
		LogID:      logID,
		RawMessage: rawMessage,
	}
}

func intNumber(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case int64:
		return int(typed)
	default:
		return 0
	}
}

func findNestedString(value any, keys ...string) string {
	switch typed := value.(type) {
	case map[string]any:
		if direct := stringValue(typed, keys...); strings.TrimSpace(direct) != "" {
			return direct
		}
		for _, nested := range typed {
			if resolved := findNestedString(nested, keys...); strings.TrimSpace(resolved) != "" {
				return resolved
			}
		}
	case []any:
		for _, nested := range typed {
			if resolved := findNestedString(nested, keys...); strings.TrimSpace(resolved) != "" {
				return resolved
			}
		}
	}
	return ""
}

func redactQuery(values neturl.Values) string {
	if len(values) == 0 {
		return ""
	}
	cloned := neturl.Values{}
	for key, items := range values {
		copyItems := append([]string(nil), items...)
		lower := strings.ToLower(strings.TrimSpace(key))
		if strings.Contains(lower, "token") || strings.Contains(lower, "secret") {
			for idx := range copyItems {
				copyItems[idx] = "***"
			}
		}
		cloned[key] = copyItems
	}
	return cloned.Encode()
}

func redactBody(raw []byte) string {
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return ""
	}
	var payload any
	if err := json.Unmarshal(raw, &payload); err == nil {
		redacted := redactJSONValue(payload)
		if normalized, marshalErr := json.Marshal(redacted); marshalErr == nil {
			return string(normalized)
		}
	}
	if len(text) > 1200 {
		return text[:1200] + "...(truncated)"
	}
	return text
}

func redactJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, item := range typed {
			lower := strings.ToLower(strings.TrimSpace(key))
			if strings.Contains(lower, "secret") || strings.Contains(lower, "token") {
				out[key] = "***"
				continue
			}
			out[key] = redactJSONValue(item)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, redactJSONValue(item))
		}
		return out
	default:
		return value
	}
}
