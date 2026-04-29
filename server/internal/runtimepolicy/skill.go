package runtimepolicy

import "strings"

const (
	SkillModeAnswer  = "answer"
	SkillModeSummary = "summary"
	SkillModeActions = "actions"
)

func NormalizeSkillMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case SkillModeSummary:
		return SkillModeSummary
	case SkillModeActions:
		return SkillModeActions
	default:
		return SkillModeAnswer
	}
}
