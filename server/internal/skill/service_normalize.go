package skill

import (
	"strings"

	"github.com/google/uuid"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func normalizeSkillSource(raw string) domain.SkillSource {
	switch domain.SkillSource(strings.ToLower(strings.TrimSpace(raw))) {
	case domain.SkillSourceGithub:
		return domain.SkillSourceGithub
	case domain.SkillSourceUpload:
		return domain.SkillSourceUpload
	default:
		return domain.SkillSourceManual
	}
}

func normalizeSkillKind(raw string) domain.SkillKind {
	switch domain.SkillKind(strings.ToLower(strings.TrimSpace(raw))) {
	case domain.SkillKindAgentWorkflow:
		return domain.SkillKindAgentWorkflow
	default:
		return domain.SkillKindChatProfile
	}
}

func normalizeStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeStringSlice(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	out := make([]string, 0, len(input))
	seen := map[string]struct{}{}
	for _, item := range input {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func fallbackSlug(raw, title string) string {
	if strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw)
	}
	candidate := strings.ToLower(strings.TrimSpace(title))
	var builder strings.Builder
	lastDash := false
	for _, r := range candidate {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || (r >= 0x4e00 && r <= 0x9fa5) {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteRune('-')
			lastDash = true
		}
	}
	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		return uuid.NewString()
	}
	return slug
}
