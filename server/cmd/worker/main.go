package main

import (
	"context"
	"errors"
	"log"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/queue"
	redisqueue "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/queue/redis"
	appcore "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/app"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/config"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()
	if strings.ToLower(strings.TrimSpace(cfg.QueueMode)) != taskqueue.ModeRedis {
		log.Fatalf("Knowvia worker requires QQA_QUEUE_MODE=%s, got %q", taskqueue.ModeRedis, cfg.QueueMode)
	}

	startupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	services, err := appcore.Bootstrap(startupCtx, cfg)
	if err != nil {
		log.Fatalf("bootstrap worker services: %v", err)
	}
	defer services.Close()

	worker, err := redisqueue.NewWorker(
		redisqueue.Options{
			Addr:               cfg.RedisAddr,
			QueueName:          cfg.QueueName,
			WorkerConcurrency:  cfg.QueueWorkers,
			MaxAttempts:        cfg.QueueMaxAttempts,
			PollTimeout:        cfg.QueuePollTimeout,
			RetryBaseDelay:     cfg.QueueRetryBaseDelay,
			RetryMaxDelay:      cfg.QueueRetryMaxDelay,
			MetricsLogInterval: cfg.QueueMetricsLogPeriod,
			ShutdownTimeout:    cfg.QueueShutdownTimeout,
			DedupTTL:           cfg.QueueDedupTTL,
		},
		func(ctx context.Context, runID string) error {
			return services.Run.ExecuteRun(ctx, runID)
		},
		func(ctx context.Context, connectionID, jobID string) error {
			return services.Knowledge.ExecuteSync(ctx, connectionID, jobID)
		},
	)
	if err != nil {
		log.Fatalf("create redis worker: %v", err)
	}
	defer func() { _ = worker.Close() }()

	runCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("Knowvia worker listening with queue mode=%s redis=%s queue=%s workers=%d", cfg.QueueMode, cfg.RedisAddr, cfg.QueueName, cfg.QueueWorkers)
	if err := worker.Run(runCtx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}
