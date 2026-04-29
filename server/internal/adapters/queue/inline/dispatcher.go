package inline

import (
	"context"
)

type Dispatcher struct {
	runFn  func(context.Context, string) error
	syncFn func(context.Context, string, string) error
}

func New(
	runFn func(context.Context, string) error,
	syncFn func(context.Context, string, string) error,
) *Dispatcher {
	return &Dispatcher{runFn: runFn, syncFn: syncFn}
}

func (d *Dispatcher) EnqueueRun(_ context.Context, runID string) error {
	go func() {
		_ = d.runFn(context.Background(), runID)
	}()
	return nil
}

func (d *Dispatcher) EnqueueKnowledgeSync(_ context.Context, connectionID, jobID string) error {
	go func() {
		_ = d.syncFn(context.Background(), connectionID, jobID)
	}()
	return nil
}

func (d *Dispatcher) Close() error {
	return nil
}
