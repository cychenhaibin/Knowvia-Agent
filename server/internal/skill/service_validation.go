package skill

import "strings"

func hasSkillRevisionPatch(input UpdateInput) bool {
	return input.Slug != nil ||
		input.Kind != nil ||
		input.Title != nil ||
		input.Description != nil ||
		input.Prompt != nil ||
		input.Mode != nil ||
		input.Source != nil ||
		input.RepoURL != nil ||
		input.PlannerPolicy != nil ||
		input.ToolAllowlist != nil
}

func hasSkillInstallationPatch(input UpdateInput) bool {
	return input.Enabled != nil || input.Name != nil || input.IsDefault != nil
}

func isExistingDefinitionInstallationRequest(input CreateInput) bool {
	return strings.TrimSpace(input.DefinitionID) != "" || strings.TrimSpace(input.CurrentRevisionID) != ""
}

func hasNewSkillCreateFields(input CreateInput) bool {
	return strings.TrimSpace(input.Slug) != "" ||
		strings.TrimSpace(input.Kind) != "" ||
		strings.TrimSpace(input.Title) != "" ||
		strings.TrimSpace(input.Description) != "" ||
		strings.TrimSpace(input.Prompt) != "" ||
		strings.TrimSpace(input.Mode) != "" ||
		strings.TrimSpace(input.Source) != "" ||
		strings.TrimSpace(input.RepoURL) != "" ||
		input.PlannerPolicy != nil ||
		len(input.ToolAllowlist) > 0
}

func presentCreateInputFields(input CreateInput) []string {
	fields := make([]string, 0, 15)
	if strings.TrimSpace(input.DefinitionID) != "" {
		fields = append(fields, "definitionId")
	}
	if strings.TrimSpace(input.CurrentRevisionID) != "" {
		fields = append(fields, "currentRevisionId")
	}
	if strings.TrimSpace(input.Name) != "" {
		fields = append(fields, "name")
	}
	if input.IsDefault != nil {
		fields = append(fields, "isDefault")
	}
	if strings.TrimSpace(input.Slug) != "" {
		fields = append(fields, "slug")
	}
	if strings.TrimSpace(input.Kind) != "" {
		fields = append(fields, "kind")
	}
	if strings.TrimSpace(input.Title) != "" {
		fields = append(fields, "title")
	}
	if strings.TrimSpace(input.Description) != "" {
		fields = append(fields, "description")
	}
	if strings.TrimSpace(input.Prompt) != "" {
		fields = append(fields, "prompt")
	}
	if strings.TrimSpace(input.Mode) != "" {
		fields = append(fields, "mode")
	}
	if strings.TrimSpace(input.Source) != "" {
		fields = append(fields, "source")
	}
	if input.Enabled != nil {
		fields = append(fields, "enabled")
	}
	if strings.TrimSpace(input.RepoURL) != "" {
		fields = append(fields, "repoUrl")
	}
	if input.PlannerPolicy != nil {
		fields = append(fields, "plannerPolicy")
	}
	if len(input.ToolAllowlist) > 0 {
		fields = append(fields, "toolAllowlist")
	}
	return fields
}

func presentUpdateInputFields(input UpdateInput) []string {
	fields := make([]string, 0, 14)
	if input.CurrentRevisionID != nil {
		fields = append(fields, "currentRevisionId")
	}
	if input.Name != nil {
		fields = append(fields, "name")
	}
	if input.IsDefault != nil {
		fields = append(fields, "isDefault")
	}
	if input.Slug != nil {
		fields = append(fields, "slug")
	}
	if input.Kind != nil {
		fields = append(fields, "kind")
	}
	if input.Title != nil {
		fields = append(fields, "title")
	}
	if input.Description != nil {
		fields = append(fields, "description")
	}
	if input.Prompt != nil {
		fields = append(fields, "prompt")
	}
	if input.Mode != nil {
		fields = append(fields, "mode")
	}
	if input.Source != nil {
		fields = append(fields, "source")
	}
	if input.Enabled != nil {
		fields = append(fields, "enabled")
	}
	if input.RepoURL != nil {
		fields = append(fields, "repoUrl")
	}
	if input.PlannerPolicy != nil {
		fields = append(fields, "plannerPolicy")
	}
	if input.ToolAllowlist != nil {
		fields = append(fields, "toolAllowlist")
	}
	return fields
}

func supportedSkillUpdateFields() []string {
	return []string{
		"slug",
		"kind",
		"title",
		"description",
		"prompt",
		"mode",
		"source",
		"enabled",
		"repoUrl",
		"plannerPolicy",
		"toolAllowlist",
		"name",
		"isDefault",
	}
}

func supportedSkillInstallationUpdateFields() []string {
	return []string{
		"currentRevisionId",
		"slug",
		"kind",
		"title",
		"description",
		"prompt",
		"mode",
		"source",
		"enabled",
		"repoUrl",
		"plannerPolicy",
		"toolAllowlist",
		"name",
		"isDefault",
	}
}
