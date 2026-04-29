package redis

import (
	"context"
	"fmt"
	"log"
	"strings"

	redigo "github.com/redis/go-redis/v9"
)

var promoteRetryScript = redigo.NewScript(`
local items = redis.call("ZRANGEBYSCORE", KEYS[1], "-inf", ARGV[1], "LIMIT", 0, ARGV[2])
for _, item in ipairs(items) do
  redis.call("ZREM", KEYS[1], item)
  redis.call("LPUSH", KEYS[2], item)
end
return items
`)

func queueKeys(queueName string) (queueKey, processingKey, retryKey, failedKey, dedupPrefix string) {
	base := strings.TrimSpace(queueName)
	if base == "" {
		base = defaultQueueName
	}
	return base, base + ":processing", base + ":retry", base + ":failed", base + ":dedupe"
}

func normalizeRedisOptions(options Options) Options {
	if strings.TrimSpace(options.QueueName) == "" {
		options.QueueName = defaultQueueName
	}
	if options.PollTimeout <= 0 {
		options.PollTimeout = defaultPollTimeout
	}
	if options.MaxAttempts <= 0 {
		options.MaxAttempts = defaultMaxAttempts
	}
	if options.WorkerConcurrency <= 0 {
		options.WorkerConcurrency = defaultWorkerConcurrency
	}
	if options.RetryBaseDelay <= 0 {
		options.RetryBaseDelay = defaultRetryBaseDelay
	}
	if options.RetryMaxDelay <= 0 {
		options.RetryMaxDelay = defaultRetryMaxDelay
	}
	if options.RetryBaseDelay > options.RetryMaxDelay {
		options.RetryBaseDelay = options.RetryMaxDelay
	}
	if options.MetricsLogInterval <= 0 {
		options.MetricsLogInterval = defaultMetricsLogInterval
	}
	if options.ShutdownTimeout <= 0 {
		options.ShutdownTimeout = defaultShutdownTimeout
	}
	if options.DedupTTL <= 0 {
		options.DedupTTL = defaultDedupTTL
	}
	if options.Logger == nil {
		options.Logger = log.Default()
	}
	return options
}

func reportWorkerError(errCh chan<- error, err error) {
	if err == nil {
		return
	}
	select {
	case errCh <- err:
	default:
	}
}

func newRedisClient(redisAddr string) (*redigo.Client, error) {
	var (
		options *redigo.Options
		err     error
	)
	if strings.Contains(redisAddr, "://") {
		options, err = redigo.ParseURL(redisAddr)
		if err != nil {
			return nil, fmt.Errorf("parse redis url: %w", err)
		}
	} else {
		options = &redigo.Options{Addr: redisAddr}
	}
	client := redigo.NewClient(options)
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("connect redis: %w", err)
	}
	return client, nil
}

func runTaskKey(runID string) string {
	return "run:" + strings.TrimSpace(runID)
}

func knowledgeTaskKey(connectionID, jobID string) string {
	return "knowledge:" + strings.TrimSpace(connectionID) + ":" + strings.TrimSpace(jobID)
}

func taskPayloadKey(payload taskPayload) string {
	switch payload.Type {
	case taskTypeRun:
		return runTaskKey(payload.RunID)
	case taskTypeKnowledgeSync:
		return knowledgeTaskKey(payload.ConnectionID, payload.JobID)
	default:
		return strings.TrimSpace(payload.Type)
	}
}

func dedupKey(prefix, key string) string {
	return strings.TrimSpace(prefix) + ":" + strings.TrimSpace(key)
}
