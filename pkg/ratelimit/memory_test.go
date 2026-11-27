package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestMemoryRateLimiter_AllowUntilLimit(t *testing.T) {
	r := newMemoryRateLimiter()
	ctx := context.Background()
	limit := PerMinute(3)

	for i := 0; i < 3; i++ {
		res, err := r.Allow(ctx, "k", limit)
		if err != nil {
			t.Fatalf("Allow %d: %v", i, err)
		}
		if res.Allowed != 1 {
			t.Fatalf("Allow %d: Allowed=%d, want 1", i, res.Allowed)
		}
	}
	res, _ := r.Allow(ctx, "k", limit)
	if res.Allowed != 0 {
		t.Fatalf("4th Allow should be denied, got Allowed=%d", res.Allowed)
	}
	if res.RetryAfter <= 0 {
		t.Fatalf("denied result should have positive RetryAfter, got %v", res.RetryAfter)
	}
}

func TestMemoryRateLimiter_WindowRollover(t *testing.T) {
	r := newMemoryRateLimiter()
	ctx := context.Background()
	limit := Limit{Rate: 1, Burst: 1, Period: 30 * time.Millisecond}

	if res, _ := r.Allow(ctx, "k", limit); res.Allowed != 1 {
		t.Fatalf("first Allow denied")
	}
	if res, _ := r.Allow(ctx, "k", limit); res.Allowed != 0 {
		t.Fatalf("second Allow in same window should be denied")
	}
	time.Sleep(35 * time.Millisecond)
	if res, _ := r.Allow(ctx, "k", limit); res.Allowed != 1 {
		t.Fatalf("post-window Allow should succeed, got Allowed=%d", res.Allowed)
	}
}
