package redis

import (
	"context"
	"time"
)

func (w *Worker) logMetricsLoop(ctx context.Context) {
	ticker := time.NewTicker(w.metricsLogInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snapshot, err := w.Snapshot(ctx)
			if err != nil {
				if w.logger != nil {
					w.logger.Printf("Redis worker metrics error: %v", err)
				}
				continue
			}
			if w.logger != nil {
				w.logger.Printf(
					"Redis worker metrics queue=%s dequeued=%d succeeded=%d retried=%d failed=%d recovered=%d invalid=%d inflight=%d queue_depth=%d processing_depth=%d retry_depth=%d failed_depth=%d",
					w.queueName,
					snapshot.Dequeued,
					snapshot.Succeeded,
					snapshot.Retried,
					snapshot.Failed,
					snapshot.Recovered,
					snapshot.InvalidPayloads,
					snapshot.InFlight,
					snapshot.QueueDepth,
					snapshot.ProcessingDepth,
					snapshot.RetryDepth,
					snapshot.FailedDepth,
				)
			}
		}
	}
}
