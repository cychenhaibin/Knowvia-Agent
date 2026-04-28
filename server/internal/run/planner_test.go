package run

import (
	"testing"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

func TestPlannerClassifiesInternalGoal(t *testing.T) {
	plan := NewPlanner().Build("结合语雀知识库总结团队内部规范", domain.RunModeAuto)
	if plan.EffectiveMode != domain.RunModeKBOnly {
		t.Fatalf("expected kb_only, got %s", plan.EffectiveMode)
	}
}

func TestPlannerClassifiesWebGoal(t *testing.T) {
	plan := NewPlanner().Build("搜索最近 AI agent 行业趋势", domain.RunModeAuto)
	if plan.EffectiveMode != domain.RunModeWeb {
		t.Fatalf("expected web_only, got %s", plan.EffectiveMode)
	}
}

func TestPlannerClassifiesHybridGoal(t *testing.T) {
	plan := NewPlanner().Build("结合公司语雀和最近新闻研究 Manus 类产品", domain.RunModeAuto)
	if plan.EffectiveMode != domain.RunModeHybrid {
		t.Fatalf("expected hybrid, got %s", plan.EffectiveMode)
	}
}

func TestPlannerBuildWithRuntimeSpecDisablesWebSteps(t *testing.T) {
	spec := &domain.SkillRuntimeSpec{
		ToolPolicy: map[string][]string{
			"allowlist": []string{"knowledge.search", "report.write"},
		},
	}
	plan := NewPlanner().BuildWithRuntimeSpec("结合公司语雀和最近新闻研究 Manus 类产品", domain.RunModeAuto, spec)
	if plan.EffectiveMode != domain.RunModeKBOnly {
		t.Fatalf("expected kb_only after tool clamp, got %s", plan.EffectiveMode)
	}
	for _, step := range plan.Steps {
		if step.Kind == domain.StepKindWebSearch || step.Kind == domain.StepKindWebExtract {
			t.Fatalf("expected web steps to be removed, found %s", step.Kind)
		}
	}
}

func TestPlannerBuildWithRuntimeSpecDisablesExtractOnly(t *testing.T) {
	spec := &domain.SkillRuntimeSpec{
		ToolPolicy: map[string][]string{
			"allowlist": []string{"knowledge.search", "web.search", "report.write"},
		},
	}
	plan := NewPlanner().BuildWithRuntimeSpec("搜索最近 AI agent 行业趋势", domain.RunModeAuto, spec)
	hasWebSearch := false
	for _, step := range plan.Steps {
		if step.Kind == domain.StepKindWebSearch {
			hasWebSearch = true
		}
		if step.Kind == domain.StepKindWebExtract {
			t.Fatalf("expected web extract step to be removed")
		}
	}
	if !hasWebSearch {
		t.Fatalf("expected web search step to remain")
	}
}

func TestPlannerBuildWithRuntimeSpecHonorsForcedMode(t *testing.T) {
	spec := &domain.SkillRuntimeSpec{
		PlannerPolicy: map[string]string{
			"force_mode": string(domain.RunModeWeb),
		},
		ToolPolicy: map[string][]string{
			"allowlist": []string{"web.search", "web.extract", "report.write"},
		},
	}
	plan := NewPlanner().BuildWithRuntimeSpec("结合语雀知识库总结团队内部规范", domain.RunModeAuto, spec)
	if plan.EffectiveMode != domain.RunModeWeb {
		t.Fatalf("expected forced web_only, got %s", plan.EffectiveMode)
	}
}

func TestPlannerBuildWithRuntimeSpecUsesGenericKnowledgeLabel(t *testing.T) {
	plan := NewPlanner().Build("结合语雀知识库总结团队内部规范", domain.RunModeAuto)
	for _, step := range plan.Steps {
		if step.Kind == domain.StepKindYuqueSearch {
			if step.Label != "检索知识库" {
				t.Fatalf("expected generic knowledge label, got %q", step.Label)
			}
			if step.Summary != "Query synced knowledge chunks and collect internal evidence." {
				t.Fatalf("expected generic knowledge summary, got %q", step.Summary)
			}
			return
		}
	}
	t.Fatalf("expected knowledge step to be present")
}

func TestPlannerBuildWithRuntimeSpecSkipsKnowledgeStepWithoutSelectedConnections(t *testing.T) {
	plan := NewPlanner().BuildWithRuntimeSpecAndKnowledge(
		"结合语雀知识库总结团队内部规范",
		domain.RunModeAuto,
		nil,
		false,
	)
	if plan.EffectiveMode != domain.RunModeWeb {
		t.Fatalf("expected web_only when knowledge is unavailable, got %s", plan.EffectiveMode)
	}
	for _, step := range plan.Steps {
		if step.Kind == domain.StepKindYuqueSearch {
			t.Fatalf("expected knowledge step to be skipped when no knowledge connections are selected")
		}
	}
}
