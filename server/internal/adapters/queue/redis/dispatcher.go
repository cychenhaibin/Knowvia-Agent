package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	defaultQueueName              = "knowvia:tasks"
	defaultPollTimeout            = 5 * time.Second
	defaultMaxAttempts            = 5
	defaultWorkerConcurrency      = 4
	defaultRetryBaseDelay         = 2 * time.Second
	defaultRetryMaxDelay          = 2 * time.Minute
	defaultMetricsLogInterval     = 30 * time.Second
	defaultShutdownTimeout        = 15 * time.Second
	defaultRetryPromotionInterval = 1 * time.Second
	defaultDedupTTL               = 6 * time.Hour

	taskTypeRun           = "run.execute"
	taskTypeKnowledgeSync = "knowledge.sync"
)

type Options struct {
	Addr               string
	QueueName          string
	PollTimeout        time.Duration
	MaxAttempts        int
	WorkerConcurrency  int
	RetryBaseDelay     time.Duration
	RetryMaxDelay      time.Duration
	MetricsLogInterval time.Duration
	ShutdownTimeout    time.Duration
	DedupTTL           time.Duration
	Logger             *log.Logger
}

type taskPayload struct {
	Type           string    `json:"type"`
	RunID          string    `json:"runId,omitempty"`
	ConnectionID   string    `json:"connectionId,omitempty"`
	JobID          string    `json:"jobId,omitempty"`
	IdempotencyKey string    `json:"idempotencyKey,omitempty"`
	Attempts       int       `json:"attempts"`
	EnqueuedAt     time.Time `json:"enqueuedAt"`
	LastError      string    `json:"lastError,omitempty"`
	RetryAfter     time.Time `json:"retryAfter,omitempty"`
}

type Dispatcher struct {
	client      *redis.Client
	queueKey    string
	queueName   string
	dedupPrefix string
	dedupTTL    time.Duration
}

type WorkerMetricsSnapshot struct {
	Dequeued        int64
	Succeeded       int64
	Retried         int64
	Failed          int64
	Recovered       int64
	InvalidPayloads int64
	InFlight        int64
	QueueDepth      int64
	ProcessingDepth int64
	RetryDepth      int64
	FailedDepth     int64
}

type Worker struct {
	client             *redis.Client
	queueName          string
	queueKey           string
	processingKey      string
	retryKey           string
	failedKey          string
	dedupPrefix        string
	pollTimeout        time.Duration
	maxAttempts        int
	workerConcurrency  int
	retryBaseDelay     time.Duration
	retryMaxDelay      time.Duration
	metricsLogInterval time.Duration
	shutdownTimeout    time.Duration
	dedupTTL           time.Duration
	runFn              func(context.Context, string) error
	syncFn             func(context.Context, string, string) error
	logger             *log.Logger
	dequeued           atomic.Int64
	succeeded          atomic.Int64
	retried            atomic.Int64
	failed             atomic.Int64
	recovered          atomic.Int64
	invalidPayloads    atomic.Int64
	inFlight           atomic.Int64
}

func NewDispatcher(options Options) (*Dispatcher, error) {
	options = normalizeRedisOptions(options)
	client, err := newRedisClient(options.Addr)
	if err != nil {
		return nil, err
	}
	queueKey, _, _, _, dedupPrefix := queueKeys(options.QueueName)
	return &Dispatcher{
		client:      client,
		queueKey:    queueKey,
		queueName:   options.QueueName,
		dedupPrefix: dedupPrefix,
		dedupTTL:    options.DedupTTL,
	}, nil
}

func (d *Dispatcher) EnqueueRun(ctx context.Context, runID string) error {
	return d.enqueue(ctx, taskPayload{
		Type:           taskTypeRun,
		RunID:          runID,
		IdempotencyKey: runTaskKey(runID),
		EnqueuedAt:     time.Now().UTC(),
	})
}

func (d *Dispatcher) EnqueueKnowledgeSync(ctx context.Context, connectionID, jobID string) error {
	return d.enqueue(ctx, taskPayload{
		Type:           taskTypeKnowledgeSync,
		ConnectionID:   connectionID,
		JobID:          jobID,
		IdempotencyKey: knowledgeTaskKey(connectionID, jobID),
		EnqueuedAt:     time.Now().UTC(),
	})
}

func (d *Dispatcher) enqueue(ctx context.Context, payload taskPayload) error {
	if strings.TrimSpace(payload.IdempotencyKey) == "" {
		payload.IdempotencyKey = taskPayloadKey(payload)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal queue payload: %w", err)
	}
	if payload.IdempotencyKey != "" {
		enqueued, err := d.client.SetNX(ctx, dedupKey(d.dedupPrefix, payload.IdempotencyKey), "1", d.dedupTTL).Result()
		if err != nil {
			return fmt.Errorf("set queue idempotency key: %w", err)
		}
		if !enqueued {
			return nil
		}
	}
	if err := d.client.LPush(ctx, d.queueKey, encoded).Err(); err != nil {
		if payload.IdempotencyKey != "" {
			_ = d.client.Del(ctx, dedupKey(d.dedupPrefix, payload.IdempotencyKey)).Err()
		}
		return fmt.Errorf("enqueue queue payload: %w", err)
	}
	return nil
}

func (d *Dispatcher) Close() error {
	if d == nil || d.client == nil {
		return nil
	}
	return d.client.Close()
}

func NewWorker(
	options Options,
	runFn func(context.Context, string) error,
	syncFn func(context.Context, string, string) error,
) (*Worker, error) {
	options = normalizeRedisOptions(options)
	client, err := newRedisClient(options.Addr)
	if err != nil {
		return nil, err
	}
	queueKey, processingKey, retryKey, failedKey, dedupPrefix := queueKeys(options.QueueName)
	return &Worker{
		client:             client,
		queueName:          options.QueueName,
		queueKey:           queueKey,
		processingKey:      processingKey,
		retryKey:           retryKey,
		failedKey:          failedKey,
		dedupPrefix:        dedupPrefix,
		pollTimeout:        options.PollTimeout,
		maxAttempts:        options.MaxAttempts,
		workerConcurrency:  options.WorkerConcurrency,
		retryBaseDelay:     options.RetryBaseDelay,
		retryMaxDelay:      options.RetryMaxDelay,
		metricsLogInterval: options.MetricsLogInterval,
		shutdownTimeout:    options.ShutdownTimeout,
		dedupTTL:           options.DedupTTL,
		runFn:              runFn,
		syncFn:             syncFn,
		logger:             options.Logger,
	}, nil
}
