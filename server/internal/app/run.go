package app

import (
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/gitrepo"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/run"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillresolver"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/tools"
)

type runBackend interface {
	run.RunStore
	run.RunStepStore
	run.RunArtifactStore
	run.RunSourceStore
	run.RunSkillStore
	run.TaskSessionStore
	skillresolver.InstallationRecordStore
}

func buildRunService(
	runStore runBackend,
	clients externalClients,
	broker *run.EventBroker,
	knowledgeTool tools.KnowledgeSearcher,
) *run.Service {
	service := run.NewService(
		run.ServiceDeps{
			Runs:         runStore,
			Steps:        runStore,
			Artifacts:    runStore,
			Sources:      runStore,
			Skills:       runStore,
			Selection:    runStore,
			TaskSessions: runStore,
		},
		nil,
		broker,
		run.NewPlanner(),
		knowledgeTool,
		tools.NewDuckDuckGoSearchTool(),
		tools.NewWebPageExtractTool(),
		tools.NewEvidenceMergeTool(clients.forward),
		tools.NewMarkdownReportWriter(clients.llm, clients.forward),
	)
	service.SetGitHubAnalyzer(gitrepo.NewAnalyzerWithGenerator("", clients.llm))
	return service
}
