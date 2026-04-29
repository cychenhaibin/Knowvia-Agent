package feishu

import (
	"errors"
	"net"
	"strings"
)

func normalizeFeishuTransportError(err error) error {
	if err == nil {
		return nil
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return &APIError{
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
		return &APIError{
			Code:       "network_error",
			Message:    "飞书网络请求失败，请检查网络环境或代理设置后重试。",
			RawMessage: err.Error(),
		}
	}

	return &APIError{
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

	return &APIError{
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
