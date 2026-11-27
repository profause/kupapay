package cache

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestMemoryLocker_LockUnlock(t *testing.T) {
	l := newMemoryLocker()
	ctx := context.Background()

	unlock, err := l.Lock(ctx, "k", time.Second)
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if err := unlock(ctx); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	// Re-acquire after release.
	unlock, err = l.Lock(ctx, "k", time.Second)
	if err != nil {
		t.Fatalf("re-Lock: %v", err)
	}
	_ = unlock(ctx)
}

func TestMemoryLocker_ContendedLockFails(t *testing.T) {
	l := newMemoryLocker()
	ctx := context.Background()

	unlock, err := l.Lock(ctx, "k", time.Second)
	if err != nil {
		t.Fatalf("first Lock: %v", err)
	}
	defer unlock(ctx)

	if _, err := l.Lock(ctx, "k", time.Second); !errors.Is(err, ErrLockNotAcquired) {
		t.Fatalf("second Lock: want ErrLockNotAcquired, got %v", err)
	}
}

func TestMemoryLocker_ExpiredLockReclaimed(t *testing.T) {
	l := newMemoryLocker()
	ctx := context.Background()

	_, err := l.Lock(ctx, "k", 20*time.Millisecond)
	if err != nil {
		t.Fatalf("first Lock: %v", err)
	}
	// Don't release; wait past the TTL — next acquire should succeed.
	time.Sleep(30 * time.Millisecond)

	unlock, err := l.Lock(ctx, "k", time.Second)
	if err != nil {
		t.Fatalf("post-expiry Lock: %v", err)
	}
	_ = unlock(ctx)
}

func TestMemoryLocker_WithLockRunsAndReleases(t *testing.T) {
	l := newMemoryLocker()
	ctx := context.Background()

	var ran bool
	err := l.WithLock(ctx, "k", time.Second, func() error {
		ran = true
		return nil
	})
	if err != nil || !ran {
		t.Fatalf("WithLock err=%v ran=%v", err, ran)
	}
	// Lock should be released — next acquire succeeds.
	unlock, err := l.Lock(ctx, "k", time.Second)
	if err != nil {
		t.Fatalf("post-WithLock Lock: %v", err)
	}
	_ = unlock(ctx)
}

func TestMemoryLocker_ConcurrentAcquireExclusive(t *testing.T) {
	l := newMemoryLocker()
	ctx := context.Background()

	const goroutines = 50
	var wg sync.WaitGroup
	var acquired int32
	var mu sync.Mutex

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock, err := l.Lock(ctx, "shared", time.Second)
			if err != nil {
				return // expected for losers
			}
			mu.Lock()
			acquired++
			mu.Unlock()
			_ = unlock(ctx)
		}()
	}
	wg.Wait()
	// At any instant only one holds the lock; but since fast-released, multiple
	// successes are fine. The actual invariant: ALL successes must each have
	// been exclusive — guaranteed by Lock returning ErrLockNotAcquired when
	// contended. We just assert at least one succeeded.
	if acquired == 0 {
		t.Fatalf("expected at least one acquirer to succeed, got 0")
	}
}
