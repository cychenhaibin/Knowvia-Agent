package textutil

import (
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strings"
)

var htmlTagPattern = regexp.MustCompile(`<[^>]+>`)

type lakeTableColumnInfo struct {
	ID      string
	Name    string
	Options map[string]string
}

func NormalizeBody(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	if looksLikeStructuredBody(body) {
		if normalized, ok := normalizeStructuredBody(body); ok && normalized != "" {
			return normalized
		}
	}
	return cleanPlainText(body)
}

func NormalizeLines(text string) string {
	lines := strings.Split(text, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.Join(cleaned, "\n")
}

func cleanPlainText(text string) string {
	withoutTags := htmlTagPattern.ReplaceAllString(text, "\n")
	replacer := strings.NewReplacer(
		"\r\n", "\n",
		"\r", "\n",
		"\t", " ",
		"&nbsp;", " ",
	)
	return NormalizeLines(html.UnescapeString(replacer.Replace(withoutTags)))
}

func looksLikeStructuredBody(body string) bool {
	body = strings.TrimSpace(body)
	return strings.HasPrefix(body, "{") || strings.HasPrefix(body, "[")
}

func normalizeStructuredBody(body string) (string, bool) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return "", false
	}

	if format, _ := payload["format"].(string); format == "laketable" {
		text := normalizeLakeTable(payload)
		if text != "" {
			return text, true
		}
	}

	return "", false
}

func normalizeLakeTable(payload map[string]any) string {
	sheets, _ := payload["sheet"].([]any)
	if len(sheets) == 0 {
		return ""
	}

	sheet, _ := sheets[0].(map[string]any)
	if len(sheet) == 0 {
		return ""
	}

	columnOrder := make([]string, 0)
	columnByID := map[string]lakeTableColumnInfo{}
	columnOptions := []string{}

	if columns, _ := sheet["columns"].([]any); len(columns) > 0 {
		for _, rawColumn := range columns {
			column, _ := rawColumn.(map[string]any)
			if len(column) == 0 {
				continue
			}
			id, _ := column["id"].(string)
			name, _ := column["name"].(string)
			if name == "" {
				continue
			}

			info := lakeTableColumnInfo{
				ID:      id,
				Name:    name,
				Options: map[string]string{},
			}
			columnOrder = append(columnOrder, name)

			if options, _ := column["options"].([]any); len(options) > 0 {
				values := make([]string, 0, len(options))
				for _, rawOption := range options {
					option, _ := rawOption.(map[string]any)
					optionID, _ := option["id"].(string)
					value, _ := option["value"].(string)
					if value == "" {
						continue
					}
					if optionID != "" {
						info.Options[optionID] = value
					}
					values = append(values, value)
				}
				if len(values) > 0 {
					columnOptions = append(columnOptions, fmt.Sprintf("%s: %s", name, strings.Join(values, ", ")))
				}
			}

			if id != "" {
				columnByID[id] = info
			}
		}
	}

	lines := []string{}
	if len(columnOrder) > 0 {
		lines = append(lines, "表格列: "+strings.Join(columnOrder, ", "))
	}
	if len(columnOptions) > 0 {
		lines = append(lines, "字段选项:")
		for _, item := range columnOptions {
			lines = append(lines, "- "+item)
		}
	}

	groupLines := extractLakeTableGroups(sheet, columnByID)
	if len(groupLines) > 0 {
		lines = append(lines, "分组统计:")
		lines = append(lines, groupLines...)
	}

	return NormalizeLines(strings.Join(lines, "\n"))
}

func extractLakeTableGroups(sheet map[string]any, columnByID map[string]lakeTableColumnInfo) []string {
	rawViews, _ := sheet["views"].(map[string]any)
	if len(rawViews) == 0 {
		return nil
	}

	viewKeys := make([]string, 0, len(rawViews))
	for key := range rawViews {
		viewKeys = append(viewKeys, key)
	}
	sort.Strings(viewKeys)

	groupLines := []string{}
	seen := map[string]struct{}{}
	for _, viewKey := range viewKeys {
		view, _ := rawViews[viewKey].(map[string]any)
		groups, _ := view["groupData"].([]any)
		for _, rawGroup := range groups {
			group, _ := rawGroup.(map[string]any)
			if len(group) == 0 {
				continue
			}
			rows, _ := group["rows"].([]any)
			if len(rows) == 0 {
				continue
			}

			label := strings.TrimSpace(resolveLakeGroupLabel(group["titleValue"], group["groupBy"], columnByID))
			if label == "" {
				label = "未分组"
			}

			line := fmt.Sprintf("- %s: %d 条", label, len(rows))
			if _, ok := seen[line]; ok {
				continue
			}
			seen[line] = struct{}{}
			groupLines = append(groupLines, line)
		}
	}

	sort.Strings(groupLines)
	return groupLines
}

func resolveLakeGroupLabel(titleValue any, groupBy any, columnByID map[string]lakeTableColumnInfo) string {
	ids := flattenStrings(titleValue)
	if len(ids) == 0 {
		return ""
	}

	groupFieldID, _ := groupBy.(string)
	info, hasColumn := columnByID[groupFieldID]

	labels := make([]string, 0, len(ids))
	for _, id := range ids {
		label := id
		if hasColumn {
			if optionLabel, ok := info.Options[id]; ok {
				label = optionLabel
			}
		}
		labels = append(labels, label)
	}
	return strings.Join(labels, ", ")
}

func flattenStrings(value any) []string {
	switch typed := value.(type) {
	case string:
		if typed == "" {
			return nil
		}
		return []string{typed}
	case []any:
		items := []string{}
		for _, item := range typed {
			items = append(items, flattenStrings(item)...)
		}
		return items
	default:
		return nil
	}
}
