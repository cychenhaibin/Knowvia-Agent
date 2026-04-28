package store

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

func tokenize(text string) []string {
	lowered := strings.ToLower(text)
	fields := strings.FieldsFunc(lowered, func(r rune) bool {
		return unicode.IsSpace(r) ||
			r == ',' || r == '，' ||
			r == '.' || r == '。' ||
			r == ':' || r == '：' ||
			r == ';' || r == '；' ||
			r == '!' || r == '！' ||
			r == '?' || r == '？' ||
			r == '(' || r == '（' ||
			r == ')' || r == '）'
	})
	seen := map[string]struct{}{}
	filtered := []string{}
	add := func(token string) {
		token = strings.TrimSpace(strings.ToLower(token))
		if token == "" {
			return
		}
		if _, ok := seen[token]; ok {
			return
		}
		seen[token] = struct{}{}
		filtered = append(filtered, token)
	}

	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		add(field)
		for _, part := range splitMixedToken(field) {
			add(part)
		}
		if containsHan(field) {
			for _, gram := range cjkNGrams(field, 2, 3) {
				add(gram)
			}
		}
	}
	return filtered
}

func lexicalScore(content string, tokens []string) float64 {
	lowered := strings.ToLower(content)
	score := 0.0
	for _, token := range tokens {
		if token == "" {
			continue
		}
		count := strings.Count(lowered, token)
		score += float64(count)
	}
	return score
}

func boostedScore(title, repo, content string, tokens []string) float64 {
	score := lexicalScore(content, tokens)
	score += lexicalScore(title, tokens) * 4
	score += lexicalScore(repo, tokens) * 2
	return score
}

func snippet(content string, tokens []string, limit int) string {
	content = strings.TrimSpace(content)
	runes := []rune(content)
	if len(runes) <= limit {
		return string(runes)
	}

	lowered := strings.ToLower(content)
	matchRuneIndex := -1
	for _, token := range tokens {
		if token == "" {
			continue
		}
		byteIndex := strings.Index(lowered, token)
		if byteIndex < 0 {
			continue
		}
		runeIndex := utf8.RuneCountInString(lowered[:byteIndex])
		if matchRuneIndex == -1 || runeIndex < matchRuneIndex {
			matchRuneIndex = runeIndex
		}
	}
	if matchRuneIndex < 0 {
		return string(runes[:limit]) + "..."
	}

	start := matchRuneIndex - limit/4
	if start < 0 {
		start = 0
	}
	end := start + limit
	if end > len(runes) {
		end = len(runes)
		start = max(0, end-limit)
	}

	var builder strings.Builder
	if start > 0 {
		builder.WriteString("...")
	}
	builder.WriteString(string(runes[start:end]))
	if end < len(runes) {
		builder.WriteString("...")
	}
	return builder.String()
}

func splitMixedToken(token string) []string {
	runes := []rune(token)
	if len(runes) <= 1 {
		return nil
	}

	parts := []string{}
	start := 0
	lastClass := runeClass(runes[0])
	for idx := 1; idx < len(runes); idx++ {
		class := runeClass(runes[idx])
		if class != lastClass {
			part := strings.TrimSpace(string(runes[start:idx]))
			if part != "" {
				parts = append(parts, part)
			}
			start = idx
			lastClass = class
		}
	}
	if start < len(runes) {
		part := strings.TrimSpace(string(runes[start:]))
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func runeClass(r rune) int {
	switch {
	case unicode.Is(unicode.Han, r):
		return 1
	case unicode.IsLetter(r) || unicode.IsDigit(r):
		return 2
	default:
		return 3
	}
}

func containsHan(token string) bool {
	for _, r := range token {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func cjkNGrams(token string, minN, maxN int) []string {
	runes := []rune(token)
	if len(runes) < minN {
		return nil
	}
	items := []string{}
	for size := minN; size <= maxN; size++ {
		if size > len(runes) {
			break
		}
		for start := 0; start+size <= len(runes); start++ {
			items = append(items, string(runes[start:start+size]))
		}
	}
	return items
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
