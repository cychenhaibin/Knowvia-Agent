package taskqueue

import "context"

type Dispatcher interface {
	EnqueueRun(ctx context.Context, runID string) error
	EnqueueKnowledgeSync(ctx context.Context, connectionID, jobID string) error
}

type InlineDispatcher struct {
	runFn  func(context.Context, string) error
	syncFn func(context.Context, string, string) error
}

func NewInlineDispatcher(
	runFn func(context.Context, string) error,
	syncFn func(context.Context, string, string) error,
) *InlineDispatcher {
	return &InlineDispatcher{runFn: runFn, syncFn: syncFn}
}

func (d *InlineDispatcher) EnqueueRun(_ context.Context, runID string) error {
	go func() {
		_ = d.runFn(context.Background(), runID)
	}()
	return nil
}

func (d *InlineDispatcher) EnqueueKnowledgeSync(_ context.Context, connectionID, jobID string) error {
	go func() {
		_ = d.syncFn(context.Background(), connectionID, jobID)
	}()
	return nil
}
