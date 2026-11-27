package ratelimit

import (
	"context"
	"time"
)

// Limit describes a rate-limit policy: `Rate` events per `Period`, with up to
// `Burst` events tolerated in short bursts. Mirrors the GCRA-style semantics
// of redis_rate so the redis backend is a thin wrapper, but lives as its own
// type so consumers never import redis_rate directly.
type Limit struct {
	Rate   int
	Burst  int
	Period time.Duration
}

// PerSecond returns a Limit allowing n events per second (Burst == Rate).
func PerSecond(n int) Limit { return Limit{Rate: n, Burst: n, Period: time.Second} }

// PerMinute returns a Limit allowing n events per minute (Burst == Rate).
func PerMinute(n int) Limit { return Limit{Rate: n, Burst: n, Period: time.Minute} }

// PerHour returns a Limit allowing n events per hour (Burst == Rate).
func PerHour(n int) Limit { return Limit{Rate: n, Burst: n, Period: time.Hour} }

// Result is the outcome of a rate-limit check.
//   - Allowed: events actually granted by this call (0 means denied).
//   - Remaining: events left in the current window.
//   - RetryAfter: how long the caller should wait before retrying (0 when
//     allowed).
//   - ResetAfter: how long until the window fully refills.
type Result struct {
	Allowed    int
	Remaining  int
	RetryAfter time.Duration
	ResetAfter time.Duration
}

// limiter is the internal dispatch surface: middleware enforces policies via
// Allow, and the package picks a redis_rate-backed or in-memory implementation
// from the cache.Cache it was handed. Intentionally unexported — consumers
// configure a *Middleware with New, they do not see the limiter directly.
type limiter interface {
	Allow(ctx context.Context, key string, limit Limit) (*Result, error)
}
