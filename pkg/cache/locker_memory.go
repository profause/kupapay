package cache

import (
	"context"
	"sync"
	"time"
)

// memoryLocker is a single-process Locker implementation. It is suitable for
// dev/test (and any deployment that runs exactly one Atlas instance). Multiple
// instances on the memory backend will NOT coordinate — switch to Redis.
type memoryLocker struct {
	mu    sync.Mutex
	locks map[string]*memoryLockEntry
}

type memoryLockEntry struct {
	holder   chan struct{} // closed when the holder releases (used to wake waiters)
	expires  time.Time
	released bool
}

func newMemoryLocker() *memoryLocker {
	return &memoryLocker{locks: make(map[string]*memoryLockEntry)}
}

func (l *memoryLocker) Lock(ctx context.Context, name string, ttl time.Duration) (Unlock, error) {
	return l.lock(ctx, name, ttl, 1, 0)
}

func (l *memoryLocker) LockWithRetry(ctx context.Context, name string, ttl time.Duration, tries int, retryDelay time.Duration) (Unlock, error) {
	return l.lock(ctx, name, ttl, tries, retryDelay)
}

func (l *memoryLocker) WithLock(ctx context.Context, name string, ttl time.Duration, fn func() error) error {
	return withLock(ctx, l.Lock, name, ttl, fn)
}

func (l *memoryLocker) WithLockRetry(ctx context.Context, name string, ttl time.Duration, tries int, retryDelay time.Duration, fn func() error) error {
	return withLock(ctx, func(ctx context.Context, name string, ttl time.Duration) (Unlock, error) {
		return l.LockWithRetry(ctx, name, ttl, tries, retryDelay)
	}, name, ttl, fn)
}

func (l *memoryLocker) lock(ctx context.Context, name string, ttl time.Duration, tries int, retryDelay time.Duration) (Unlock, error) {
	if tries < 1 {
		tries = 1
	}
	for attempt := 0; attempt < tries; attempt++ {
		if attempt > 0 {
			if err := sleep(ctx, retryDelay); err != nil {
				return nil, err
			}
		}
		entry, ok := l.acquire(name, ttl)
		if ok {
			return l.makeUnlock(name, entry), nil
		}
	}
	return nil, ErrLockNotAcquired
}

// acquire tries once to take the lock. Returns the entry on success.
func (l *memoryLocker) acquire(name string, ttl time.Duration) (*memoryLockEntry, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	if existing, held := l.locks[name]; held {
		// Expired holder — reclaim.
		if !existing.released && now.Before(existing.expires) {
			return nil, false
		}
		if !existing.released {
			close(existing.holder)
			existing.released = true
		}
	}
	entry := &memoryLockEntry{
		holder:  make(chan struct{}),
		expires: now.Add(ttl),
	}
	l.locks[name] = entry
	return entry, true
}

func (l *memoryLocker) makeUnlock(name string, entry *memoryLockEntry) Unlock {
	var once sync.Once
	return func(ctx context.Context) error {
		once.Do(func() {
			l.mu.Lock()
			defer l.mu.Unlock()
			if !entry.released {
				close(entry.holder)
				entry.released = true
			}
			// Drop from map only if this entry is still the current holder.
			if current, ok := l.locks[name]; ok && current == entry {
				delete(l.locks, name)
			}
		})
		return nil
	}
}

// sleep waits for d or until ctx is canceled.
func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
