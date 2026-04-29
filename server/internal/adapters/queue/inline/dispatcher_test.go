package inline

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestInlineDispatcherInvokesRunAndSyncCallbacks(t *testing.T) {
	t.Parallel()

	var wg sync.WaitGroup
	wg.Add(2)

	runCalls := make(chan string, 1)
	syncCalls := make(chan [2]string, 1)

	dispatcher := New(
		func(_ context.Context, runID string) error {
			runCalls <- runID
			wg.Done()
			return nil
		},
		func(_ context.Context, connectionID, jobID string) error {
			syncCalls <- [2]string{connectionID, jobID}
			wg.Done()
			return nil
		},
	)

	if err := dispatcher.EnqueueRun(context.Background(), "run-123"); err != nil {
		t.Fatalf("EnqueueRun failed: %v", err)
	}
	if err := dispatcher.EnqueueKnowledgeSync(context.Background(), "conn-1", "job-9"); err != nil {
		t.Fatalf("EnqueueKnowledgeSync failed: %v", err)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for inline dispatcher callbacks")
	}

	if got := <-runCalls; got != "run-123" {
		t.Fatalf("run callback = %q, want run-123", got)
	}
	if got := <-syncCalls; got != [2]string{"conn-1", "job-9"} {
		t.Fatalf("sync callback = %#v, want %#v", got, [2]string{"conn-1", "job-9"})
	}
}
