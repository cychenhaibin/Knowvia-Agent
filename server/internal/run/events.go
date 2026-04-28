package run

import (
	"sync"
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

type EventBroker struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan domain.RunEvent]struct{}
}

func NewEventBroker() *EventBroker {
	return &EventBroker{
		subscribers: map[string]map[chan domain.RunEvent]struct{}{},
	}
}

func (b *EventBroker) Publish(runID string, eventType string, payload interface{}) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	event := domain.RunEvent{
		Type:      eventType,
		RunID:     runID,
		Timestamp: time.Now().UTC(),
		Payload:   payload,
	}
	for ch := range b.subscribers[runID] {
		select {
		case ch <- event:
		default:
		}
	}
}

func (b *EventBroker) Subscribe(runID string) (<-chan domain.RunEvent, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan domain.RunEvent, 32)
	if _, ok := b.subscribers[runID]; !ok {
		b.subscribers[runID] = map[chan domain.RunEvent]struct{}{}
	}
	b.subscribers[runID][ch] = struct{}{}

	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.subscribers[runID], ch)
		close(ch)
		if len(b.subscribers[runID]) == 0 {
			delete(b.subscribers, runID)
		}
	}
}
