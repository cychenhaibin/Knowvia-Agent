package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/joho/godotenv"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/auth"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/chat"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/config"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/httpapi"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/knowledge"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/mirror"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/provider"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/run"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/skillimport"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/store"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/taskqueue"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/tools"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()
	startupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var st store.Store
	switch cfg.StoreBackend {
	case "postgres":
		postgresStore, err := store.NewPostgresStore(startupCtx, cfg.PostgresDSN)
		if err != nil {
			log.Fatalf("connect store backend postgres: %v", err)
		}
		defer postgresStore.Close()
		st = postgresStore
		log.Printf("Knowvia store backend: postgres")
	default:
		st = store.NewMemoryStore()
		log.Printf("Knowvia store backend: memory")
	}

	authService := auth.NewService(st, cfg)
	if err := authService.SeedDevUsers(context.Background()); err != nil {
		log.Fatalf("seed users: %v", err)
	}

	broker := run.NewEventBroker()
	llmClient := provider.NewOpenAICompatibleClient(cfg)
	forwardClient := provider.NewPythonForwardClient(cfg)
	mirrorService := mirror.NewService(st, forwardClient)
	knowledgeService := knowledge.NewService(st, forwardClient, forwardClient)
	knowledgeSearchTool := tools.NewKnowledgeSearchTool(st, forwardClient)
	chatService := chat.NewService(knowledgeSearchTool, llmClient, forwardClient)
	skillImporter := skillimport.NewService(nil)
	go mirrorService.StartBackgroundRetry(context.Background())

	var runService *run.Service
	dispatcher := taskqueue.NewInlineDispatcher(
		func(ctx context.Context, runID string) error {
			return runService.ExecuteRun(ctx, runID)
		},
		func(ctx context.Context, connectionID, jobID string) error {
			return knowledgeService.ExecuteSync(ctx, connectionID, jobID)
		},
	)

	runService = run.NewService(
		st,
		dispatcher,
		broker,
		run.NewPlanner(),
		knowledgeSearchTool,
		tools.NewDuckDuckGoSearchTool(),
		tools.NewWebPageExtractTool(),
		tools.NewEvidenceMergeTool(forwardClient),
		tools.NewMarkdownReportWriter(llmClient, forwardClient),
	)

	router := httpapi.NewRouter(
		authService,
		st,
		runService,
		knowledgeService,
		mirrorService,
		chatService,
		skillImporter,
		dispatcher,
		broker,
	)
	log.Printf("Knowvia API listening on %s", cfg.ServerAddr)
	listener, err := net.Listen("tcp4", cfg.ServerAddr)
	if err != nil {
		log.Fatal(err)
	}
	if err := http.Serve(listener, router); err != nil {
		log.Fatal(err)
	}
}
