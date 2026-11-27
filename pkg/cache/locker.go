package cache

import (
	"context"
	"errors"
	"time"
)

// Locker is the distributed lock surface used by the application. The Redis
// backend uses redsync (cross-process); the memory backend uses sync.Mutex
// (single-process — suitable for dev/test, not multi-instance prod).
type Locker interface {
	// Lock acquires a single-attempt lock named name, held for at most ttl,
	// and returns an Unlock that releases it. Errors if the lock is held.
	Lock(ctx context.Context, name string, ttl time.Duration) (Unlock, error)

	// LockWithRetry tries up to `tries` times, sleeping retryDelay between
	// attempts, before giving up.
	LockWithRetry(ctx context.Context, name string, ttl time.Duration, tries int, retryDelay time.Duration) (Unlock, error)

	// WithLock acquires a lock, runs fn, and releases on return. The lock is
	// released even if fn panics.
	WithLock(ctx context.Context, name string, ttl time.Duration, fn func() error) error

	// WithLockRetry combines LockWithRetry + WithLock semantics.
	WithLockRetry(ctx context.Context, name string, ttl time.Duration, tries int, retryDelay time.Duration, fn func() error) error
}

// Unlock releases a previously-acquired lock. Idempotent: calling Unlock on
// a lock that's already released or expired returns nil (the redis backend
// signals "not held" as an error from redsync; we swallow that).
type Unlock func(ctx context.Context) error

// ErrLockNotAcquired is returned when a lock cannot be obtained.
var ErrLockNotAcquired = errors.New("cache: lock not acquired")

// NewLocker returns the Locker matching the cache backend. Callers pass in
// the Cache and Backend produced by NewCache.
func NewLocker(c Cache, backend Backend) Locker {
	if backend == BackendRedis {
		if r, ok := c.(*Redis); ok {
			return newRedisLocker(r)
		}
	}
	return newMemoryLocker()
}
