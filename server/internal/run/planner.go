package run

import (
	"strings"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

type PlannedStep struct {
	Kind    domain.StepKind `json:"kind"`
	Label   string          `json:"label"`
	Summary string          `json:"summary"`
}

type Plan struct {
	EffectiveMode domain.RunMode `json:"effectiveMode"`
	Steps         []PlannedStep  `json:"steps"`
}

type Planner struct{}

func NewPlanner() Planner {
	return Planner{}
}

func (Planner) Build(goal string, requestedMode domain.RunMode) Plan {
	return Planner{}.buildWithInputs(goal, requestedMode, nil, true)
}

func (Planner) BuildWithRuntimeSpec(
	goal string,
	requestedMode domain.RunMode,
	spec *domain.SkillRuntimeSpec,
) Plan {
	return Planner{}.buildWithInputs(goal, requestedMode, spec, true)
}

func (Planner) BuildWithRuntimeSpecAndKnowledge(
	goal string,
	requestedMode domain.RunMode,
	spec *domain.SkillRuntimeSpec,
	hasKnowledgeConnections bool,
) Plan {
	return Planner{}.buildWithInputs(goal, requestedMode, spec, hasKnowledgeConnections)
}

func (Planner) buildWithInputs(
	goal string,
	requestedMode domain.RunMode,
	spec *domain.SkillRuntimeSpec,
	hasKnowledgeConnections bool,
) Plan {
	mode := classifyMode(goal, requestedMode, spec, hasKnowledgeConnections)
	allowKnowledge := hasKnowledgeConnections && toolAllowed(spec, "knowledge.search")
	allowWebSearch := toolAllowed(spec, "web.search")
	allowWebExtract := allowWebSearch && toolAllowed(spec, "web.extract")
	steps := []PlannedStep{
		{Kind: domain.StepKindPlanning, Label: "规划任务", Summary: "Classify the goal and build the execution timeline."},
	}

	if allowKnowledge && (mode == domain.RunModeKBOnly || mode == domain.RunModeHybrid) {
		steps = append(steps, PlannedStep{
			Kind:    domain.StepKindYuqueSearch,
			Label:   "检索知识库",
			Summary: "Query synced knowledge chunks and collect internal evidence.",
		})
	}

	if allowWebSearch && (mode == domain.RunModeWeb || mode == domain.RunModeHybrid) {
		steps = append(steps, PlannedStep{
			Kind:    domain.StepKindWebSearch,
			Label:   "执行网页搜索",
			Summary: "Search the web for fresh external references.",
		})
		if allowWebExtract {
			steps = append(steps, PlannedStep{
				Kind:    domain.StepKindWebExtract,
				Label:   "抽取页面正文",
				Summary: "Fetch up to 3 high-value pages and extract readable text.",
			})
		}
	}

	if hasEvidenceCollectionSteps(steps) {
		steps = append(steps, PlannedStep{
			Kind:    domain.StepKindEvidenceMerge,
			Label:   "整理证据",
			Summary: "Deduplicate findings and rank the strongest evidence.",
		})
	}

	steps = append(steps,
		PlannedStep{
			Kind:    domain.StepKindReportWriter,
			Label:   "生成结构化报告",
			Summary: "Produce a report with findings, citations, and actions.",
		},
		PlannedStep{
			Kind:    domain.StepKindFinalize,
			Label:   "交付最终结果",
			Summary: "Store final artifacts and mark the run complete.",
		},
	)

	return Plan{
		EffectiveMode: mode,
		Steps:         steps,
	}
}

func classifyMode(goal string, requestedMode domain.RunMode, spec *domain.SkillRuntimeSpec, hasKnowledgeConnections bool) domain.RunMode {
	forcedMode := parseRunMode(specValue(spec, "force_mode"))
	if forcedMode != "" && forcedMode != domain.RunModeAuto {
		return clampModeByAllowedTools(forcedMode, spec, hasKnowledgeConnections)
	}
	if requestedMode != "" && requestedMode != domain.RunModeAuto {
		return clampModeByAllowedTools(requestedMode, spec, hasKnowledgeConnections)
	}
	preferredMode := parseRunMode(specValue(spec, "preferred_mode"))
	if preferredMode != "" && preferredMode != domain.RunModeAuto {
		return clampModeByAllowedTools(preferredMode, spec, hasKnowledgeConnections)
	}

	lowered := strings.ToLower(goal)
	internalHints := []string{"语雀", "知识库", "内部", "公司", "repo", "wiki"}
	webHints := []string{"最新", "最近", "新闻", "趋势", "互联网", "搜索", "web", "网页"}

	hasInternal := containsAny(lowered, internalHints)
	hasWeb := containsAny(lowered, webHints)

	switch {
	case hasInternal && hasWeb:
		return clampModeByAllowedTools(domain.RunModeHybrid, spec, hasKnowledgeConnections)
	case hasInternal:
		return clampModeByAllowedTools(domain.RunModeKBOnly, spec, hasKnowledgeConnections)
	case hasWeb:
		return clampModeByAllowedTools(domain.RunModeWeb, spec, hasKnowledgeConnections)
	default:
		return clampModeByAllowedTools(domain.RunModeHybrid, spec, hasKnowledgeConnections)
	}
}

func containsAny(goal string, tokens []string) bool {
	for _, token := range tokens {
		if strings.Contains(goal, token) {
			return true
		}
	}
	return false
}

func toolAllowed(spec *domain.SkillRuntimeSpec, tool string) bool {
	if spec == nil || len(spec.ToolPolicy) == 0 {
		return true
	}
	allowlist := spec.ToolPolicy["allowlist"]
	if len(allowlist) == 0 {
		return true
	}
	for _, item := range allowlist {
		if strings.EqualFold(strings.TrimSpace(item), tool) {
			return true
		}
	}
	return false
}

func clampModeByAllowedTools(mode domain.RunMode, spec *domain.SkillRuntimeSpec, hasKnowledgeConnections bool) domain.RunMode {
	allowKnowledge := hasKnowledgeConnections && toolAllowed(spec, "knowledge.search")
	allowWeb := toolAllowed(spec, "web.search")
	switch mode {
	case domain.RunModeKBOnly:
		if allowKnowledge {
			return domain.RunModeKBOnly
		}
		if allowWeb {
			return domain.RunModeWeb
		}
	case domain.RunModeWeb:
		if allowWeb {
			return domain.RunModeWeb
		}
		if allowKnowledge {
			return domain.RunModeKBOnly
		}
	case domain.RunModeHybrid:
		if allowKnowledge && allowWeb {
			return domain.RunModeHybrid
		}
		if allowKnowledge {
			return domain.RunModeKBOnly
		}
		if allowWeb {
			return domain.RunModeWeb
		}
	default:
		if allowKnowledge && allowWeb {
			return domain.RunModeHybrid
		}
		if allowKnowledge {
			return domain.RunModeKBOnly
		}
		if allowWeb {
			return domain.RunModeWeb
		}
	}
	return domain.RunModeHybrid
}

func specValue(spec *domain.SkillRuntimeSpec, key string) string {
	if spec == nil || len(spec.PlannerPolicy) == 0 {
		return ""
	}
	return strings.TrimSpace(spec.PlannerPolicy[key])
}

func parseRunMode(value string) domain.RunMode {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case string(domain.RunModeKBOnly):
		return domain.RunModeKBOnly
	case string(domain.RunModeWeb):
		return domain.RunModeWeb
	case string(domain.RunModeHybrid):
		return domain.RunModeHybrid
	default:
		return domain.RunModeAuto
	}
}

func hasEvidenceCollectionSteps(steps []PlannedStep) bool {
	for _, step := range steps {
		switch step.Kind {
		case domain.StepKindYuqueSearch, domain.StepKindWebSearch, domain.StepKindWebExtract:
			return true
		}
	}
	return false
}
