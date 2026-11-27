// Package eventbus is an in-process domain-event publisher. Domain services
// publish typed Event values; subscribers register handlers per event type
// and receive them asynchronously. The in-memory implementation fires each
// handler on its own goroutine with a panic-recovery and a per-handler
// timeout, so a slow / panicking subscriber can't block or crash the publisher.
//
// Why bother (over inline notifier/activity calls):
//   - Decouples request latency from subscriber health: a slow email notifier
//     no longer slows the login endpoint.
//   - Lets new side-effects subscribe to existing events without touching
//     the publishing service.
//   - Wire-format: a single Bus singleton instead of every service taking
//     direct notifier/activity dependencies.
//
// This is intentionally not durable. For exactly-once guarantees, drain a
// transactional outbox table behind the same Bus interface.
package eventbus

import (
	"context"
	"sync"
	"time"

	"github.com/profause/kupapay/pkg/logger"
)

// Event is anything domain-meaningful that has a stable type identifier.
type Event interface {
	EventType() string
}

// Handler reacts to an event. Long-running handlers should respect ctx.
type Handler func(ctx context.Context, e Event)

// Bus is the publish/subscribe surface.
type Bus interface {
	Publish(ctx context.Context, e Event)
	Subscribe(eventType string, h Handler)
}

// handlerTimeout bounds a single handler invocation. Tuned for emails /
// activity logs; raise per-subscriber if a specific consumer needs longer.
const handlerTimeout = 10 * time.Second

type inMemory struct {
	mu     sync.RWMutex
	subs   map[string][]Handler
	logger *logger.Logger
	sync   bool // when true, handlers run inline on Publish (test-only)
}

// New returns a goroutine-fan-out in-memory bus.
func New(log *logger.Logger) Bus {
	return &inMemory{subs: map[string][]Handler{}, logger: log}
}

// NewSync returns an in-memory bus that runs handlers inline on Publish.
// Use this from tests so assertions can run immediately after a publish
// without racing the async fan-out.
func NewSync(log *logger.Logger) Bus {
	return &inMemory{subs: map[string][]Handler{}, logger: log, sync: true}
}

func (b *inMemory) Subscribe(eventType string, h Handler) {
	if h == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs[eventType] = append(b.subs[eventType], h)
}

func (b *inMemory) Publish(ctx context.Context, e Event) {
	if e == nil {
		return
	}
	b.mu.RLock()
	handlers := append([]Handler(nil), b.subs[e.EventType()]...)
	b.mu.RUnlock()
	if len(handlers) == 0 {
		return
	}
	// Detach from request cancellation; the request can return long before
	// the side-effect finishes. Carry over values (auth principal, request
	// meta) via context.WithoutCancel.
	if b.sync {
		for _, h := range handlers {
			func() {
				defer b.recover(e.EventType())
				h(ctx, e)
			}()
		}
		return
	}
	parent := context.WithoutCancel(ctx)
	for _, h := range handlers {
		h := h
		go func() {
			defer b.recover(e.EventType())
			ctx, cancel := context.WithTimeout(parent, handlerTimeout)
			defer cancel()
			h(ctx, e)
		}()
	}
}

func (b *inMemory) recover(eventType string) {
	if r := recover(); r != nil && b.logger != nil {
		b.logger.Errorw("event handler panic", "eventType", eventType, "panic", r)
	}
}
