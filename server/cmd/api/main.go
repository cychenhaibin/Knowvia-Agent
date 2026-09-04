package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/httpapi"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/queue"
	inlinequeue "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/queue/inline"
	redisqueue "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/queue/redis"
	appcore "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/app"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/config"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}
	startupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	services, err := appcore.Bootstrap(startupCtx, cfg)
	if err != nil {
		log.Fatalf("bootstrap services: %v", err)
	}
	defer services.Close()
	log.Printf("Knowvia store backend: %s", cfg.StoreBackend)

	if err := services.Auth.SeedDevUsers(context.Background()); err != nil {
		log.Fatalf("seed users: %v", err)
	}
	go services.Mirror.StartBackgroundRetry(context.Background())

	runFn := func(ctx context.Context, runID string) error {
		return services.Run.ExecuteRun(ctx, runID)
	}
	syncFn := func(ctx context.Context, connectionID, jobID string) error {
		return services.Knowledge.ExecuteSync(ctx, connectionID, jobID)
	}
	var dispatcher taskqueue.Dispatcher
	switch strings.ToLower(strings.TrimSpace(cfg.QueueMode)) {
	case "", taskqueue.ModeInline:
		dispatcher = inlinequeue.New(runFn, syncFn)
	case taskqueue.ModeRedis:
		dispatcher, err = redisqueue.NewDispatcher(redisqueue.Options{
			Addr:      cfg.RedisAddr,
			QueueName: cfg.QueueName,
			DedupTTL:  cfg.QueueDedupTTL,
		})
		if err != nil {
			log.Fatalf("build redis dispatcher: %v", err)
		}
	default:
		log.Fatalf("unsupported queue mode %q", cfg.QueueMode)
	}
	defer func() { _ = dispatcher.Close() }()
	services.SetDispatcher(dispatcher)

	router := httpapi.NewRouter(
		services.Auth,
		services.Run,
		services.Knowledge,
		services.Chat,
		services.Skill,
		services.SkillImport,
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
