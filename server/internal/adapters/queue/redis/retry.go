package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	redigo "github.com/redis/go-redis/v9"
)

func (w *Worker) recoverProcessing(ctx context.Context) error {
	recovered := 0
	for {
		raw, err := w.client.RPopLPush(ctx, w.processingKey, w.queueKey).Result()
		if err == redigo.Nil {
			break
		}
		if err != nil {
			return fmt.Errorf("recover processing queue: %w", err)
		}
		if raw != "" {
			recovered++
		}
	}
	if recovered > 0 {
		w.recovered.Add(int64(recovered))
		if w.logger != nil {
			w.logger.Printf("Redis worker recovered %d in-flight task(s)", recovered)
		}
	}
	return nil
}

func (w *Worker) promoteRetryLoop(ctx context.Context) error {
	ticker := time.NewTicker(defaultRetryPromotionInterval)
	defer ticker.Stop()

	for {
		if err := w.promoteReadyRetries(ctx); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (w *Worker) promoteReadyRetries(ctx context.Context) error {
	now := time.Now().UTC().UnixMilli()
	for {
		items, err := promoteRetryScript.Run(ctx, w.client, []string{w.retryKey, w.queueKey}, now, 1).Result()
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return fmt.Errorf("promote retry task: %w", err)
		}
		promoted, ok := items.([]any)
		if !ok || len(promoted) == 0 {
			return nil
		}
	}
}

func (w *Worker) ackRawTask(ctx context.Context, raw string) error {
	if err := w.client.LRem(ctx, w.processingKey, 1, raw).Err(); err != nil {
		return fmt.Errorf("ack task: %w", err)
	}
	return nil
}

func (w *Worker) scheduleRetry(ctx context.Context, raw string, payload taskPayload) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal retry payload: %w", err)
	}
	delay := w.backoffDelay(payload.Attempts)
	payload.RetryAfter = time.Now().UTC().Add(delay)
	encoded, err = json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal retry payload with retry time: %w", err)
	}
	score := float64(payload.RetryAfter.UnixMilli())
	moved, err := moveProcessingToRetryScript.Run(ctx, w.client, []string{w.processingKey, w.retryKey}, raw, string(encoded), score).Int()
	if err != nil {
		return fmt.Errorf("schedule retry task: %w", err)
	}
	if moved != 1 {
		return errors.New("schedule retry task: processing task not found")
	}
	if w.logger != nil {
		w.logger.Printf("Redis worker scheduled retry for %s attempt %d after %s", payload.Type, payload.Attempts, delay)
	}
	return nil
}

func (w *Worker) backoffDelay(attempt int) time.Duration {
	if attempt <= 1 {
		return w.retryBaseDelay
	}
	delay := w.retryBaseDelay
	for i := 1; i < attempt; i++ {
		if delay >= w.retryMaxDelay/2 {
			return w.retryMaxDelay
		}
		delay *= 2
	}
	if delay > w.retryMaxDelay {
		return w.retryMaxDelay
	}
	return delay
}

func (w *Worker) pushFailedTask(ctx context.Context, raw string, payload taskPayload) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal failed payload: %w", err)
	}
	moved, err := moveProcessingToFailedScript.Run(ctx, w.client, []string{w.processingKey, w.failedKey}, raw, string(encoded)).Int()
	if err != nil {
		return fmt.Errorf("push failed task: %w", err)
	}
	if moved != 1 {
		return errors.New("push failed task: processing task not found")
	}
	if w.logger != nil {
		w.logger.Printf("Redis worker moved %s to failed queue after %d attempts: %s", payload.Type, payload.Attempts, payload.LastError)
	}
	return nil
}

func (w *Worker) failRawTask(ctx context.Context, raw string, err error) error {
	payload := taskPayload{
		Type:       "invalid",
		Attempts:   w.maxAttempts,
		LastError:  err.Error(),
		EnqueuedAt: time.Now().UTC(),
		RetryAfter: time.Time{},
	}
	return w.pushFailedTask(ctx, raw, payload)
}

func (w *Worker) releaseIdempotencyKey(ctx context.Context, payload taskPayload) error {
	if strings.TrimSpace(payload.IdempotencyKey) == "" {
		return nil
	}
	if err := w.client.Del(ctx, dedupKey(w.dedupPrefix, payload.IdempotencyKey)).Err(); err != nil {
		return fmt.Errorf("release task idempotency key: %w", err)
	}
	return nil
}
