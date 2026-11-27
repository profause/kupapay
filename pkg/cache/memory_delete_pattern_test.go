package cache

import (
	"context"
	"testing"
	"time"
)

func TestMemoryCache_DeleteByPattern_PrefixOnly(t *testing.T) {
	mc, err := NewMemoryCache(MemoryCacheConfig{MaxSize: 100})
	if err != nil {
		t.Fatalf("NewMemoryCache: %v", err)
	}
	defer mc.Close()

	ctx := context.Background()
	keys := []string{"settings:a", "settings:b", "other:c", "settings:nested:x"}
	for _, k := range keys {
		if err := mc.Set(ctx, k, "v", time.Minute); err != nil {
			t.Fatalf("Set %s: %v", k, err)
		}
	}

	if err := mc.DeleteByPattern(ctx, "settings:*"); err != nil {
		t.Fatalf("DeleteByPattern: %v", err)
	}

	if _, err := mc.Get(ctx, "settings:a"); err == nil {
		t.Fatalf("settings:a should be gone")
	}
	if _, err := mc.Get(ctx, "settings:nested:x"); err == nil {
		t.Fatalf("settings:nested:x should be gone")
	}
	if v, err := mc.Get(ctx, "other:c"); err != nil || v != "v" {
		t.Fatalf("other:c should survive, got v=%q err=%v", v, err)
	}
}

func TestMemoryCache_DeleteByPattern_LiteralKey(t *testing.T) {
	mc, _ := NewMemoryCache(MemoryCacheConfig{MaxSize: 10})
	defer mc.Close()
	ctx := context.Background()
	_ = mc.Set(ctx, "exact", "v", time.Minute)

	// No trailing "*" → treated as literal key.
	if err := mc.DeleteByPattern(ctx, "exact"); err != nil {
		t.Fatalf("DeleteByPattern literal: %v", err)
	}
	if _, err := mc.Get(ctx, "exact"); err == nil {
		t.Fatalf("literal-key delete failed to remove")
	}
}
