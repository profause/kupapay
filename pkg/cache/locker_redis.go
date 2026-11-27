package cache

import (
	"context"
	"errors"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
)

// redisLocker implements Locker on top of redsync. It is cross-process safe
// when backed by a real Redis cluster.
type redisLocker struct {
	rs    *redsync.Redsync
	redis *Redis
}

func newRedisLocker(r *Redis) *redisLocker {
	pool := goredis.NewPool(r.Client())
	return &redisLocker{rs: redsync.New(pool), redis: r}
}

func (l *redisLocker) Lock(ctx context.Context, name string, ttl time.Duration) (Unlock, error) {
	return l.lock(ctx, name, ttl, 1, 0)
}

func (l *redisLocker) LockWithRetry(ctx context.Context, name string, ttl time.Duration, tries int, retryDelay time.Duration) (Unlock, error) {
	return l.lock(ctx, name, ttl, tries, retryDelay)
}

func (l *redisLocker) WithLock(ctx context.Context, name string, ttl time.Duration, fn func() error) error {
	return withLock(ctx, l.Lock, name, ttl, fn)
}

func (l *redisLocker) WithLockRetry(ctx context.Context, name string, ttl time.Duration, tries int, retryDelay time.Duration, fn func() error) error {
	return withLock(ctx, func(ctx context.Context, name string, ttl time.Duration) (Unlock, error) {
		return l.LockWithRetry(ctx, name, ttl, tries, retryDelay)
	}, name, ttl, fn)
}

func (l *redisLocker) lock(ctx context.Context, name string, ttl time.Duration, tries int, retryDelay time.Duration) (Unlock, error) {
	opts := []redsync.Option{redsync.WithExpiry(ttl)}
	if tries > 0 {
		opts = append(opts, redsync.WithTries(tries))
	}
	if retryDelay > 0 {
		opts = append(opts, redsync.WithRetryDelay(retryDelay))
	}
	mu := l.rs.NewMutex(l.redis.PrefixedKey(name), opts...)
	if err := mu.LockContext(ctx); err != nil {
		if errors.Is(err, redsync.ErrFailed) {
			return nil, ErrLockNotAcquired
		}
		return nil, err
	}
	return func(unlockCtx context.Context) error {
		ok, err := mu.UnlockContext(unlockCtx)
		// "not held" (ok=false) is treated as success: the lock either expired
		// or was released elsewhere; idempotent Unlock matches the memory impl.
		if err != nil && ok {
			return err
		}
		return nil
	}, nil
}

// withLock is the shared "acquire → fn → release" helper for both backends.
func withLock(ctx context.Context, acquire func(context.Context, string, time.Duration) (Unlock, error), name string, ttl time.Duration, fn func() error) error {
	unlock, err := acquire(ctx, name, ttl)
	if err != nil {
		return err
	}
	defer func() { _ = unlock(ctx) }()
	return fn()
}
