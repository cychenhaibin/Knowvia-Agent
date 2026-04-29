package taskqueue

import "context"

const (
	ModeInline = "inline"
	ModeRedis  = "redis"
)

type Dispatcher interface {
	EnqueueRun(ctx context.Context, runID string) error
	EnqueueKnowledgeSync(ctx context.Context, connectionID, jobID string) error
	Close() error
}
