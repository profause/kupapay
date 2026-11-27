// Package ratelimit provides a Fiber middleware on top of pkg/cache's
// RateLimiter. It exists so handlers can attach the same redis-backed
// rate-limit primitive to a single route, a whole surface, or the whole app
// without reimplementing the policy plumbing each time.
//
// Design notes:
//   - The package is domain-free. It depends on pkg/cache and pkg/apiresponse
//     only; principal- or auth-aware key strategies are supplied by callers as
//     KeyFunc values (see Middleware.Limit examples in cmd/api/routes.go and
//     internal/auth/handler.go).
//   - Fail-open: if the underlying limiter errors (e.g. transient Redis
//     outage), requests are allowed through and the failure is logged. The
//     alternative — failing closed — would convert a cache outage into a
//     full-API outage, which is strictly worse than briefly skipping the
//     limit.
//   - Backend selection happens upstream in pkg/cache.NewCache; Redis vs.
//     memory is transparent here.
package ratelimit

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/profause/kupapay/pkg/httpx"
	"github.com/profause/kupapay/pkg/apiresponse"
	"github.com/profause/kupapay/pkg/cache"
	"github.com/profause/kupapay/pkg/logger"
)

// DefaultKeyPrefix is the namespace applied to every limiter key so rate-limit
// state cannot collide with other cache entries sharing a Redis database.
const DefaultKeyPrefix = "rate_limit:"

// ErrUnnamedPolicy is returned by Middleware.Limit when a policy is missing a
// Name — names are required because they are part of the bucket key.
var ErrUnnamedPolicy = errors.New("ratelimit: policy.Name is required")

// KeyFunc derives the bucket key for a request. Returning "" tells the
// middleware to fall back to the client IP, so a misconfigured KeyFunc never
// silently disables the limit.
type KeyFunc func(fiber.Ctx) string

// Policy describes a single rate-limit rule. Name is part of the bucket key,
// so distinct policies with the same key strategy (e.g. two endpoints both
// keyed by IP) do not share state.
type Policy struct {
	// Name namespaces the bucket. Required.
	Name string
	// Limit is the rate-limit window. See Limit / PerMinute / PerHour.
	Limit Limit
	// Key derives the per-request bucket. nil falls back to client IP.
	Key KeyFunc
	// Message overrides the 429 response body. Empty means "Too many requests".
	Message string
}

// Middleware builds Fiber handlers that enforce Policy values against a
// single shared backend chosen from the cache.Cache it was constructed with.
type Middleware struct {
	limiter limiter
	log     *logger.Logger
	prefix  string
}

// New constructs a Middleware backed by the given cache.Cache. When c is a
// *cache.Redis the limiter uses redis_rate (cross-process GCRA); otherwise it
// falls back to an in-process fixed-window counter suitable for single-node
// deployments and tests. Buckets are shared across every handler returned by
// the same Middleware.
func New(c cache.Cache, log *logger.Logger) *Middleware {
	return &Middleware{limiter: pickLimiter(c), log: log, prefix: DefaultKeyPrefix}
}

func pickLimiter(c cache.Cache) limiter {
	if r, ok := c.(*cache.Redis); ok {
		return newRedisRateLimiter(r)
	}
	return newMemoryRateLimiter()
}

// WithKeyPrefix overrides the default "rate_limit:" namespace. Useful when
// multiple services share a Redis instance and need disjoint key spaces.
func (m *Middleware) WithKeyPrefix(prefix string) *Middleware {
	if m == nil {
		return nil
	}
	clone := *m
	clone.prefix = prefix
	return &clone
}

// Limit returns a Fiber handler enforcing the policy. A nil receiver or nil
// limiter returns a pass-through handler so wiring code can stay uniform even
// in tests or when rate limiting is disabled.
func (m *Middleware) Limit(p Policy) fiber.Handler {
	if m == nil || m.limiter == nil {
		return passThrough()
	}
	name := strings.TrimSpace(p.Name)
	if name == "" {
		// Surface the misconfiguration on first request rather than panicking
		// at wiring time — easier to spot in tests and harder to nil-deref
		// in production.
		return func(c fiber.Ctx) error {
			return apiresponse.Failure(c, fiber.StatusInternalServerError, "Rate limit misconfigured")
		}
	}
	key := p.Key
	if key == nil {
		key = KeyByIP()
	}
	msg := p.Message
	if msg == "" {
		msg = "Too many requests"
	}
	limit := p.Limit

	return func(c fiber.Ctx) error {
		bucket := key(c)
		if bucket == "" {
			bucket = httpx.ClientIP(c)
		}
		fullKey := m.prefix + name + ":" + bucket

		res, err := m.limiter.Allow(c.Context(), fullKey, limit)
		if err != nil {
			if m.log != nil {
				m.log.Warnw("rate limit check failed; allowing request",
					"policy", name, "key", bucket, "error", err)
			}
			return c.Next()
		}

		ceiling := limit.Burst
		if ceiling < limit.Rate {
			ceiling = limit.Rate
		}
		remaining := res.Remaining
		if remaining < 0 {
			remaining = 0
		}
		c.Set("X-RateLimit-Limit", strconv.Itoa(ceiling))
		c.Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

		if res.Allowed == 0 {
			retry := int(res.RetryAfter.Seconds())
			if retry < 1 {
				retry = 1
			}
			c.Set("Retry-After", strconv.Itoa(retry))
			return apiresponse.Failure(c, fiber.StatusTooManyRequests, msg)
		}
		return c.Next()
	}
}

func passThrough() fiber.Handler {
	return func(c fiber.Ctx) error { return c.Next() }
}
