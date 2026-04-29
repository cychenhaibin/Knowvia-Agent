package feishu

import (
	"fmt"
	neturl "net/url"
	"strings"
)

func TokenLooksLikeWiki(value string) bool {
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

func NormalizeDocxToken(value string) string {
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

func NormalizeWikiToken(value string) string {
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
