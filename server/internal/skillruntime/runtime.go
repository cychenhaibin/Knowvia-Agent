package skillruntime

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

const runtimeSchemaVersion = "2026-04-21"

type RevisionManifest struct {
	PlannerPolicy map[string]string `json:"plannerPolicy,omitempty"`
	ToolAllowlist []string          `json:"toolAllowlist,omitempty"`
}

func BuildSpec(skill domain.Skill) domain.SkillRuntimeSpec {
	toolAllowlist := []string{
		"knowledge.search",
		"web.search",
		"web.extract",
		"report.write",
	}
	if len(skill.ToolAllowlist) > 0 {
		toolAllowlist = append([]string(nil), skill.ToolAllowlist...)
	}
	plannerPolicy := map[string]string{
		"step_template": "explicit_research_timeline",
	}
	for key, value := range skill.PlannerPolicy {
		plannerPolicy[key] = value
	}
	if skill.Kind == "" {
		skill.Kind = domain.SkillKindChatProfile
	}
	return domain.SkillRuntimeSpec{
		SchemaVersion: runtimeSchemaVersion,
		SkillKind:     skill.Kind,
		ResponseMode:  skill.Mode,
		Instructions:  skill.Prompt,
		PlannerPolicy: plannerPolicy,
		ToolPolicy: map[string][]string{
			"allowlist": toolAllowlist,
		},
		OutputPolicy: map[string]string{
			"chat": "stream_text",
			"run":  "markdown_report",
		},
		Metadata: map[string]string{
			"skill_id":       skill.ID,
			"definition_id":  skill.DefinitionID,
			"revision_id":    skill.RevisionID,
			"slug":           skill.Slug,
			"title":          skill.Title,
			"source":         string(skill.Source),
			"kind":           string(skill.Kind),
			"response_mode":  skill.Mode,
			"repo_url":       skill.RepoURL,
			"schema_version": runtimeSchemaVersion,
		},
	}
}

func BuildSnapshot(
	skill domain.Skill,
	scope domain.SkillRuntimeScope,
	scopeID string,
	snapshotID string,
	now time.Time,
) (domain.SkillRuntimeSnapshot, error) {
	spec := BuildSpec(skill)
	rawSpec, err := json.Marshal(spec)
	if err != nil {
		return domain.SkillRuntimeSnapshot{}, err
	}
	return domain.SkillRuntimeSnapshot{
		ID:              snapshotID,
		Scope:           scope,
		ScopeID:         scopeID,
		UserID:          skill.UserID,
		InstallationID:  skill.ID,
		DefinitionID:    skill.DefinitionID,
		RevisionID:      skill.RevisionID,
		Kind:            spec.SkillKind,
		Title:           skill.Title,
		Description:     skill.Description,
		Mode:            skill.Mode,
		Prompt:          skill.Prompt,
		RuntimeSpecJSON: string(rawSpec),
		CreatedAt:       now,
	}, nil
}

func ParseSpec(raw string) (domain.SkillRuntimeSpec, error) {
	var spec domain.SkillRuntimeSpec
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		return domain.SkillRuntimeSpec{}, err
	}
	return spec, nil
}

func EncodeRevisionManifest(skill domain.Skill) (string, error) {
	manifest := RevisionManifest{
		PlannerPolicy: clonePlannerPolicy(skill.PlannerPolicy),
		ToolAllowlist: append([]string(nil), skill.ToolAllowlist...),
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func ParseRevisionManifest(raw string) (RevisionManifest, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return RevisionManifest{}, nil
	}
	var manifest RevisionManifest
	if err := json.Unmarshal([]byte(raw), &manifest); err != nil {
		return RevisionManifest{}, err
	}
	return manifest, nil
}

func ApplyRevisionManifest(skill *domain.Skill, raw string) error {
	manifest, err := ParseRevisionManifest(raw)
	if err != nil {
		return err
	}
	skill.PlannerPolicy = clonePlannerPolicy(manifest.PlannerPolicy)
	skill.ToolAllowlist = append([]string(nil), manifest.ToolAllowlist...)
	return nil
}

func ApplyRevisionManifestToRevision(revision *domain.SkillRevision, raw string) error {
	manifest, err := ParseRevisionManifest(raw)
	if err != nil {
		return err
	}
	revision.PlannerPolicy = clonePlannerPolicy(manifest.PlannerPolicy)
	revision.ToolAllowlist = append([]string(nil), manifest.ToolAllowlist...)
	return nil
}

func clonePlannerPolicy(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
