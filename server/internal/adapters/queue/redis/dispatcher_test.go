package redis

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	redigo "github.com/redis/go-redis/v9"
)

func TestDispatcherAndWorkerProcessTasks(t *testing.T) {
	mr := miniredis.RunT(t)
	dispatcher, err := NewDispatcher(Options{Addr: mr.Addr(), QueueName: "test:tasks"})
	if err != nil {
		t.Fatalf("new redis dispatcher: %v", err)
	}

	var runCount int32
	var syncCount int32
	worker, err := NewWorker(
		Options{
			Addr:               mr.Addr(),
			QueueName:          "test:tasks",
			PollTimeout:        time.Second,
			MetricsLogInterval: time.Hour,
			ShutdownTimeout:    3 * time.Second,
		},
		func(_ context.Context, runID string) error {
			if runID != "run-1" {
				t.Fatalf("unexpected run id %s", runID)
			}
			atomic.AddInt32(&runCount, 1)
			return nil
		},
		func(_ context.Context, connectionID, jobID string) error {
			if connectionID != "conn-1" || jobID != "job-1" {
				t.Fatalf("unexpected knowledge sync ids %s %s", connectionID, jobID)
			}
			atomic.AddInt32(&syncCount, 1)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("new redis worker: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- worker.Run(ctx)
	}()

	if err := dispatcher.EnqueueRun(context.Background(), "run-1"); err != nil {
		t.Fatalf("enqueue run: %v", err)
	}
	if err := dispatcher.EnqueueKnowledgeSync(context.Background(), "conn-1", "job-1"); err != nil {
		t.Fatalf("enqueue knowledge sync: %v", err)
	}

	waitForCondition(t, func() bool {
		return atomic.LoadInt32(&runCount) == 1 && atomic.LoadInt32(&syncCount) == 1
	})

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("worker exited with error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop")
	}
}

func TestDispatcherDeduplicatesQueuedTasks(t *testing.T) {
	mr := miniredis.RunT(t)
	dispatcher, err := NewDispatcher(Options{Addr: mr.Addr(), QueueName: "dedup:tasks"})
	if err != nil {
		t.Fatalf("new redis dispatcher: %v", err)
	}
	defer func() { _ = dispatcher.Close() }()

	var runCount int32
	worker, err := NewWorker(
		Options{
			Addr:               mr.Addr(),
			QueueName:          "dedup:tasks",
			PollTimeout:        time.Second,
			MetricsLogInterval: time.Hour,
			ShutdownTimeout:    3 * time.Second,
		},
		func(_ context.Context, runID string) error {
			if runID != "run-1" {
				t.Fatalf("unexpected run id %s", runID)
			}
			atomic.AddInt32(&runCount, 1)
			return nil
		},
		func(_ context.Context, _, _ string) error { return nil },
	)
	if err != nil {
		t.Fatalf("new redis worker: %v", err)
	}
	defer func() { _ = worker.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- worker.Run(ctx)
	}()

	if err := dispatcher.EnqueueRun(context.Background(), "run-1"); err != nil {
		t.Fatalf("enqueue first run: %v", err)
	}
	if err := dispatcher.EnqueueRun(context.Background(), "run-1"); err != nil {
		t.Fatalf("enqueue duplicate run: %v", err)
	}

	waitForCondition(t, func() bool {
		return atomic.LoadInt32(&runCount) == 1
	})

	queueKey, _, _, _, dedupPrefix := queueKeys("dedup:tasks")
	if mr.Exists(queueKey) {
		got, err := mr.List(queueKey)
		if err != nil {
			t.Fatalf("read queue contents: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("expected empty queue after processing, got %d item(s)", len(got))
		}
	}
	if mr.Exists(dedupKey(dedupPrefix, runTaskKey("run-1"))) {
		t.Fatal("expected dedup key to be released after successful processing")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("worker exited with error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop")
	}
}

func TestWorkerMovesFailedTaskAfterRetries(t *testing.T) {
	mr := miniredis.RunT(t)
	dispatcher, err := NewDispatcher(Options{Addr: mr.Addr(), QueueName: "test:tasks"})
	if err != nil {
		t.Fatalf("new redis dispatcher: %v", err)
	}
	defer func() { _ = dispatcher.Close() }()
	worker, err := NewWorker(
		Options{
			Addr:               mr.Addr(),
			QueueName:          "test:tasks",
			MaxAttempts:        1,
			PollTimeout:        time.Second,
			MetricsLogInterval: time.Hour,
			ShutdownTimeout:    3 * time.Second,
		},
		func(_ context.Context, _ string) error { return assertErr{} },
		func(_ context.Context, _, _ string) error { return nil },
	)
	if err != nil {
		t.Fatalf("new redis worker: %v", err)
	}
	defer func() { _ = worker.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- worker.Run(ctx)
	}()

	if err := dispatcher.EnqueueRun(context.Background(), "run-1"); err != nil {
		t.Fatalf("enqueue run: %v", err)
	}

	waitForCondition(t, func() bool {
		_, _, _, failedKey, _ := queueKeys("test:tasks")
		return mr.Exists(failedKey)
	})

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("worker exited with error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop")
	}
}

func TestWorkerRetriesWithBackoffAndPromotesDelayedTask(t *testing.T) {
	mr := miniredis.RunT(t)
	dispatcher, err := NewDispatcher(Options{Addr: mr.Addr(), QueueName: "test:tasks"})
	if err != nil {
		t.Fatalf("new redis dispatcher: %v", err)
	}
	defer func() { _ = dispatcher.Close() }()

	var attempts int32
	worker, err := NewWorker(
		Options{
			Addr:               mr.Addr(),
			QueueName:          "test:tasks",
			MaxAttempts:        3,
			PollTimeout:        time.Second,
			RetryBaseDelay:     100 * time.Millisecond,
			RetryMaxDelay:      100 * time.Millisecond,
			MetricsLogInterval: time.Hour,
			ShutdownTimeout:    3 * time.Second,
			WorkerConcurrency:  1,
		},
		func(_ context.Context, _ string) error {
			n := atomic.AddInt32(&attempts, 1)
			if n == 1 {
				return assertErr{}
			}
			return nil
		},
		func(_ context.Context, _, _ string) error { return nil },
	)
	if err != nil {
		t.Fatalf("new redis worker: %v", err)
	}
	defer func() { _ = worker.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- worker.Run(ctx)
	}()

	if err := dispatcher.EnqueueRun(context.Background(), "run-1"); err != nil {
		t.Fatalf("enqueue run: %v", err)
	}

	waitForCondition(t, func() bool {
		return atomic.LoadInt32(&attempts) == 1
	})

	_, _, retryKey, _, _ := queueKeys("test:tasks")
	if !mr.Exists(retryKey) {
		t.Fatal("expected retry task to be scheduled")
	}

	mr.FastForward(200 * time.Millisecond)

	waitForCondition(t, func() bool {
		return atomic.LoadInt32(&attempts) == 2
	})

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("worker exited with error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop")
	}
}

func TestWorkerPromotesRetryAtomically(t *testing.T) {
	mr := miniredis.RunT(t)
	worker, err := NewWorker(
		Options{
			Addr:               mr.Addr(),
			QueueName:          "atomic:tasks",
			PollTimeout:        time.Second,
			MetricsLogInterval: time.Hour,
			ShutdownTimeout:    3 * time.Second,
		},
		func(_ context.Context, _ string) error { return nil },
		func(_ context.Context, _, _ string) error { return nil },
	)
	if err != nil {
		t.Fatalf("new redis worker: %v", err)
	}
	defer func() { _ = worker.Close() }()

	_, _, retryKey, _, _ := queueKeys("atomic:tasks")
	payload := taskPayload{Type: taskTypeRun, RunID: "run-1", IdempotencyKey: runTaskKey("run-1"), EnqueuedAt: time.Now().UTC()}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if err := worker.client.ZAdd(context.Background(), retryKey, redigo.Z{
		Score:  float64(time.Now().UTC().Add(-time.Second).UnixMilli()),
		Member: encoded,
	}).Err(); err != nil {
		t.Fatalf("seed retry task: %v", err)
	}

	if err := worker.promoteReadyRetries(context.Background()); err != nil {
		t.Fatalf("promote ready retries: %v", err)
	}

	queueKey, _, _, _, _ := queueKeys("atomic:tasks")
	got, err := mr.List(queueKey)
	if err != nil {
		t.Fatalf("read promoted queue contents: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected promoted item in queue, got %d item(s)", len(got))
	}
	card, err := worker.client.ZCard(context.Background(), retryKey).Result()
	if err != nil {
		t.Fatalf("read retry depth: %v", err)
	}
	if card != 0 {
		t.Fatalf("expected retry zset to be empty, got %d", card)
	}
}

func TestWorkerSnapshotIncludesConfiguredQueueDepths(t *testing.T) {
	mr := miniredis.RunT(t)
	dispatcher, err := NewDispatcher(Options{Addr: mr.Addr(), QueueName: "metrics:tasks"})
	if err != nil {
		t.Fatalf("new redis dispatcher: %v", err)
	}
	defer func() { _ = dispatcher.Close() }()
	if err := dispatcher.EnqueueRun(context.Background(), "run-1"); err != nil {
		t.Fatalf("enqueue run: %v", err)
	}

	worker, err := NewWorker(
		Options{
			Addr:               mr.Addr(),
			QueueName:          "metrics:tasks",
			MetricsLogInterval: time.Hour,
		},
		func(_ context.Context, _ string) error { return nil },
		func(_ context.Context, _, _ string) error { return nil },
	)
	if err != nil {
		t.Fatalf("new redis worker: %v", err)
	}
	defer func() { _ = worker.Close() }()

	snapshot, err := worker.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snapshot.QueueDepth != 1 {
		t.Fatalf("expected queue depth 1, got %d", snapshot.QueueDepth)
	}
}

type assertErr struct{}

func (assertErr) Error() string { return "boom" }

func waitForCondition(t *testing.T, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}
