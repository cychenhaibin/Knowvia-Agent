package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	redigo "github.com/redis/go-redis/v9"
)

func (w *Worker) Run(ctx context.Context) error {
	if err := w.recoverProcessing(ctx); err != nil {
		return err
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 1)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := w.promoteRetryLoop(runCtx); err != nil {
			reportWorkerError(errCh, err)
			cancel()
		}
	}()

	if w.metricsLogInterval > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.logMetricsLoop(runCtx)
		}()
	}

	for workerIndex := 0; workerIndex < w.workerConcurrency; workerIndex++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := w.consumeLoop(runCtx); err != nil {
				reportWorkerError(errCh, err)
				cancel()
			}
		}()
	}

	waitDone := make(chan struct{})
	go func() {
		defer close(waitDone)
		wg.Wait()
	}()

	select {
	case err := <-errCh:
		<-waitDone
		return err
	case <-ctx.Done():
		cancel()
		timer := time.NewTimer(w.shutdownTimeout)
		defer timer.Stop()
		select {
		case <-waitDone:
			return nil
		case <-timer.C:
			return fmt.Errorf("redis worker shutdown timed out after %s", w.shutdownTimeout)
		}
	}
}

func (w *Worker) Snapshot(ctx context.Context) (WorkerMetricsSnapshot, error) {
	queueDepth, err := w.client.LLen(ctx, w.queueKey).Result()
	if err != nil {
		return WorkerMetricsSnapshot{}, fmt.Errorf("read queue depth: %w", err)
	}
	processingDepth, err := w.client.LLen(ctx, w.processingKey).Result()
	if err != nil {
		return WorkerMetricsSnapshot{}, fmt.Errorf("read processing depth: %w", err)
	}
	retryDepth, err := w.client.ZCard(ctx, w.retryKey).Result()
	if err != nil {
		return WorkerMetricsSnapshot{}, fmt.Errorf("read retry depth: %w", err)
	}
	failedDepth, err := w.client.LLen(ctx, w.failedKey).Result()
	if err != nil {
		return WorkerMetricsSnapshot{}, fmt.Errorf("read failed depth: %w", err)
	}

	return WorkerMetricsSnapshot{
		Dequeued:        w.dequeued.Load(),
		Succeeded:       w.succeeded.Load(),
		Retried:         w.retried.Load(),
		Failed:          w.failed.Load(),
		Recovered:       w.recovered.Load(),
		InvalidPayloads: w.invalidPayloads.Load(),
		InFlight:        w.inFlight.Load(),
		QueueDepth:      queueDepth,
		ProcessingDepth: processingDepth,
		RetryDepth:      retryDepth,
		FailedDepth:     failedDepth,
	}, nil
}

func (w *Worker) Close() error {
	if w == nil || w.client == nil {
		return nil
	}
	return w.client.Close()
}

func (w *Worker) consumeLoop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		raw, err := w.client.BRPopLPush(ctx, w.queueKey, w.processingKey, w.pollTimeout).Result()
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			if err == redigo.Nil {
				continue
			}
			return fmt.Errorf("dequeue task: %w", err)
		}
		w.dequeued.Add(1)
		if err := w.handleRawTask(ctx, raw); err != nil {
			return err
		}
	}
}

func (w *Worker) handleRawTask(ctx context.Context, raw string) error {
	var payload taskPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		w.invalidPayloads.Add(1)
		w.failed.Add(1)
		return w.failRawTask(ctx, raw, fmt.Errorf("decode task payload: %w", err))
	}

	w.inFlight.Add(1)
	defer w.inFlight.Add(-1)

	err := w.processTask(ctx, payload)
	if err == nil {
		w.succeeded.Add(1)
		if err := w.ackRawTask(ctx, raw); err != nil {
			return err
		}
		return w.releaseIdempotencyKey(ctx, payload)
	}

	payload.Attempts++
	payload.LastError = err.Error()
	if payload.Attempts >= w.maxAttempts {
		w.failed.Add(1)
		if err := w.pushFailedTask(ctx, raw, payload); err != nil {
			return err
		}
		return w.releaseIdempotencyKey(ctx, payload)
	}
	w.retried.Add(1)
	return w.scheduleRetry(ctx, raw, payload)
}

func (w *Worker) processTask(ctx context.Context, payload taskPayload) error {
	switch payload.Type {
	case taskTypeRun:
		if strings.TrimSpace(payload.RunID) == "" {
			return errors.New("missing run id")
		}
		return w.runFn(ctx, payload.RunID)
	case taskTypeKnowledgeSync:
		if strings.TrimSpace(payload.ConnectionID) == "" || strings.TrimSpace(payload.JobID) == "" {
			return errors.New("missing knowledge sync identifiers")
		}
		return w.syncFn(ctx, payload.ConnectionID, payload.JobID)
	default:
		return fmt.Errorf("unknown task type %q", payload.Type)
	}
}
