package skillimport

import (
	"encoding/json"
	"fmt"
	"strings"
)

func parseManifest(fileName string, raw []byte) (manifestDocument, error) {
	switch strings.ToLower(pathExt(fileName)) {
	case ".json":
		var doc manifestDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return manifestDocument{}, fmt.Errorf("%w: %v", ErrInvalidManifest, err)
		}
		return doc, nil
	case ".yaml", ".yml":
		return parseSimpleYAMLManifest(raw)
	default:
		return manifestDocument{}, fmt.Errorf("%w: unsupported manifest file %s", ErrInvalidManifest, fileName)
	}
}

func parseSimpleYAMLManifest(raw []byte) (manifestDocument, error) {
	lines := strings.Split(string(raw), "\n")
	doc := manifestDocument{
		PlannerPolicy: map[string]string{},
		ToolAllowlist: []string{},
	}
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasSuffix(trimmed, ":") {
			key := strings.TrimSuffix(trimmed, ":")
			switch normalizeYAMLKey(key) {
			case "plannerpolicy":
				for i+1 < len(lines) {
					next := strings.TrimRight(lines[i+1], "\r")
					if strings.TrimSpace(next) == "" {
						i++
						continue
					}
					if leadingIndent(next) <= leadingIndent(line) {
						break
					}
					part := strings.TrimSpace(next)
					kv := strings.SplitN(part, ":", 2)
					if len(kv) == 2 {
						doc.PlannerPolicy[strings.TrimSpace(kv[0])] = unquoteYAMLValue(kv[1])
					}
					i++
				}
			case "toolallowlist":
				for i+1 < len(lines) {
					next := strings.TrimRight(lines[i+1], "\r")
					if strings.TrimSpace(next) == "" {
						i++
						continue
					}
					if leadingIndent(next) <= leadingIndent(line) {
						break
					}
					part := strings.TrimSpace(next)
					if strings.HasPrefix(part, "-") {
						value := strings.TrimSpace(strings.TrimPrefix(part, "-"))
						if value != "" {
							doc.ToolAllowlist = append(doc.ToolAllowlist, unquoteYAMLValue(value))
						}
					}
					i++
				}
			}
			continue
		}
		kv := strings.SplitN(trimmed, ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := normalizeYAMLKey(kv[0])
		value := unquoteYAMLValue(kv[1])
		switch key {
		case "slug":
			doc.Slug = value
		case "kind":
			doc.Kind = value
		case "title":
			doc.Title = value
		case "description":
			doc.Description = value
		case "prompt":
			doc.Prompt = value
		case "mode":
			doc.Mode = value
		}
	}
	return doc, nil
}

func normalizeYAMLKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.ReplaceAll(key, "_", "")
	key = strings.ReplaceAll(key, "-", "")
	return key
}

func unquoteYAMLValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	return strings.TrimSpace(value)
}

func leadingIndent(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}
