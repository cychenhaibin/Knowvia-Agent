package app

import (
	"context"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/queue"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/auth"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/chat"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/config"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/knowledge"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/mirror"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/run"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skill"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/tools"
)

type Services struct {
	closeFn             func()
	Auth                *auth.Service
	Chat                *chat.Service
	Knowledge           *knowledge.Service
	Mirror              *mirror.Service
	Run                 *run.Service
	Skill               *skill.Service
	SkillImport         *skill.ImportService
	Broker              *run.EventBroker
	KnowledgeSearchTool tools.KnowledgeSearcher
}

func Bootstrap(ctx context.Context, cfg config.Config) (*Services, error) {
	switch cfg.StoreBackend {
	case "postgres":
		return bootstrapPostgresServices(ctx, cfg)
	default:
		return bootstrapMemoryServices(cfg)
	}
}

func (s *Services) Close() {
	if s != nil && s.closeFn != nil {
		s.closeFn()
	}
}

func (s *Services) SetDispatcher(dispatcher taskqueue.Dispatcher) {
	if s == nil {
		return
	}
	if s.Knowledge != nil {
		s.Knowledge.SetDispatcher(dispatcher)
	}
	if s.Run != nil {
		s.Run.SetDispatcher(dispatcher)
	}
}
