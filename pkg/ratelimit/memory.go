package ratelimit

import (
	"context"
	"sync"
	"time"
)

// memoryRateLimiter is a per-key fixed-window counter. It approximates the
// GCRA semantics of the redis backend well enough for dev/test: window
// granularity is per Limit.Period, so two callers near the window edge can
// briefly see 2×Rate events. For exact GCRA semantics, use Redis.
//
// State for inactive keys is reclaimed lazily on the next Allow* call; there
// is no janitor goroutine. Keys are cheap (small struct), and the lazy reset
// is invoked exactly when state would otherwise be observed stale.
type memoryRateLimiter struct {
	mu    sync.Mutex
	state map[string]*memoryRateWindow
}

type memoryRateWindow struct {
	count       int
	windowStart time.Time
}

func newMemoryRateLimiter() *memoryRateLimiter {
	return &memoryRateLimiter{state: make(map[string]*memoryRateWindow)}
}

func (r *memoryRateLimiter) Allow(ctx context.Context, key string, limit Limit) (*Result, error) {
	return r.consume(key, limit, 1, false), nil
}

// consume is the shared logic for AllowN and AllowAtMost. If atMost is true,
// the call grants min(n, remaining) tokens and never denies entirely (unless
// remaining is 0).
func (r *memoryRateLimiter) consume(key string, limit Limit, n int, atMost bool) *Result {
	ceiling := limit.Burst
	if ceiling < limit.Rate {
		ceiling = limit.Rate
	}
	now := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()

	w, ok := r.state[key]
	if !ok || now.Sub(w.windowStart) >= limit.Period {
		w = &memoryRateWindow{windowStart: now}
		r.state[key] = w
	}
	remaining := ceiling - w.count
	if remaining < 0 {
		remaining = 0
	}
	resetAfter := w.windowStart.Add(limit.Period).Sub(now)

	if atMost {
		grant := n
		if grant > remaining {
			grant = remaining
		}
		w.count += grant
		return &Result{
			Allowed:    grant,
			Remaining:  ceiling - w.count,
			RetryAfter: 0,
			ResetAfter: resetAfter,
		}
	}

	if n <= remaining {
		w.count += n
		return &Result{
			Allowed:    n,
			Remaining:  ceiling - w.count,
			RetryAfter: 0,
			ResetAfter: resetAfter,
		}
	}
	// Denied: caller must wait until the window resets.
	return &Result{
		Allowed:    0,
		Remaining:  remaining,
		RetryAfter: resetAfter,
		ResetAfter: resetAfter,
	}
}
