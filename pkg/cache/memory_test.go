package cache

import (
	"context"
	"testing"
	"time"
)

func setString(t *testing.T, mc *MemoryCache, ctx context.Context, key, value string, ttl time.Duration) {
	t.Helper()
	if err := mc.Set(ctx, key, value, ttl); err != nil {
		t.Fatalf("set %s: %v", key, err)
	}
}

func getString(t *testing.T, mc *MemoryCache, ctx context.Context, key string) string {
	t.Helper()
	value, err := mc.Get(ctx, key)
	if err != nil {
		t.Fatalf("get %s: %v", key, err)
	}
	return value
}

func TestLRUEviction(t *testing.T) {
	// Create cache with max size of 3
	mc, err := NewMemoryCache(MemoryCacheConfig{
		MaxSize:         3,
		CleanupInterval: 1 * time.Minute,
		DefaultTTL:      1 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer mc.Close()

	ctx := context.Background()

	// Add 3 items to fill the cache
	setString(t, mc, ctx, "key1", "value1", 1*time.Minute)
	setString(t, mc, ctx, "key2", "value2", 1*time.Minute)
	setString(t, mc, ctx, "key3", "value3", 1*time.Minute)

	// Verify all 3 items exist
	if mc.Size() != 3 {
		t.Errorf("Expected size 3, got %d", mc.Size())
	}

	// Access key1 to make it recently used
	_, err = mc.Get(ctx, "key1")
	if err != nil {
		t.Errorf("Failed to get key1: %v", err)
	}

	// Small delay to allow LRU update to process (async)
	time.Sleep(10 * time.Millisecond)

	// Add key4 - should evict key2 (least recently used)
	setString(t, mc, ctx, "key4", "value4", 1*time.Minute)

	// Verify size is still 3
	if mc.Size() != 3 {
		t.Errorf("Expected size 3 after eviction, got %d", mc.Size())
	}

	// key2 should be evicted (LRU)
	_, err = mc.Get(ctx, "key2")
	if err == nil {
		t.Error("key2 should have been evicted")
	}

	// key1, key3, key4 should still exist
	if _, err := mc.Get(ctx, "key1"); err != nil {
		t.Error("key1 should still exist")
	}
	if _, err := mc.Get(ctx, "key3"); err != nil {
		t.Error("key3 should still exist")
	}
	if _, err := mc.Get(ctx, "key4"); err != nil {
		t.Error("key4 should still exist")
	}
}

func TestLRUUpdateMovesToFront(t *testing.T) {
	// Create cache with max size of 2
	mc, err := NewMemoryCache(MemoryCacheConfig{
		MaxSize:         2,
		CleanupInterval: 1 * time.Minute,
		DefaultTTL:      1 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer mc.Close()

	ctx := context.Background()

	// Add 2 items
	setString(t, mc, ctx, "key1", "value1", 1*time.Minute)
	setString(t, mc, ctx, "key2", "value2", 1*time.Minute)

	// Update key1 (should move to front)
	setString(t, mc, ctx, "key1", "value1_updated", 1*time.Minute)

	// Add key3 - should evict key2 (not key1)
	setString(t, mc, ctx, "key3", "value3", 1*time.Minute)

	// key2 should be evicted
	_, err = mc.Get(ctx, "key2")
	if err == nil {
		t.Error("key2 should have been evicted")
	}

	// key1 and key3 should exist
	val := getString(t, mc, ctx, "key1")
	if val != "value1_updated" {
		t.Errorf("Expected 'value1_updated', got %q", val)
	}

	if _, err := mc.Get(ctx, "key3"); err != nil {
		t.Error("key3 should exist")
	}
}

func TestLRUWithExpiration(t *testing.T) {
	// Create cache with max size of 3 and short TTL
	mc, err := NewMemoryCache(MemoryCacheConfig{
		MaxSize:         3,
		CleanupInterval: 1 * time.Second,
		DefaultTTL:      2 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer mc.Close()

	ctx := context.Background()

	// Add 3 items
	setString(t, mc, ctx, "key1", "value1", 2*time.Second)
	setString(t, mc, ctx, "key2", "value2", 2*time.Second)
	setString(t, mc, ctx, "key3", "value3", 2*time.Second)

	// Wait for expiration
	time.Sleep(3 * time.Second)

	// All items should be expired (not accessible)
	if _, err := mc.Get(ctx, "key1"); err == nil {
		t.Error("key1 should be expired")
	}

	// Add new item - should not trigger LRU eviction since expired items exist
	setString(t, mc, ctx, "key4", "value4", 1*time.Minute)

	// Wait for cleanup to run
	time.Sleep(2 * time.Second)

	// After cleanup, only key4 should remain
	if mc.Size() != 1 {
		t.Errorf("Expected size 1 after cleanup, got %d", mc.Size())
	}
}

func TestLRUStatistics(t *testing.T) {
	mc, err := NewMemoryCache(MemoryCacheConfig{
		MaxSize:         2,
		CleanupInterval: 1 * time.Minute,
		DefaultTTL:      1 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer mc.Close()

	ctx := context.Background()

	// Add items
	setString(t, mc, ctx, "key1", "value1", 1*time.Minute)
	setString(t, mc, ctx, "key2", "value2", 1*time.Minute)

	// Hit
	mc.Get(ctx, "key1")
	stats := mc.GetStatsSnapshot()
	if stats.Hits != 1 {
		t.Errorf("Expected 1 hit, got %d", stats.Hits)
	}

	// Miss
	mc.Get(ctx, "key3")
	stats = mc.GetStatsSnapshot()
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}

	// Eviction (add key3 to trigger eviction)
	setString(t, mc, ctx, "key3", "value3", 1*time.Minute)
	stats = mc.GetStatsSnapshot()
	if stats.Evictions != 1 {
		t.Errorf("Expected 1 eviction, got %d", stats.Evictions)
	}

	// Sets
	if stats.Sets != 3 {
		t.Errorf("Expected 3 sets, got %d", stats.Sets)
	}
}

func TestLRUClear(t *testing.T) {
	mc, err := NewMemoryCache(MemoryCacheConfig{
		MaxSize:         3,
		CleanupInterval: 1 * time.Minute,
		DefaultTTL:      1 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer mc.Close()

	ctx := context.Background()

	// Add items
	setString(t, mc, ctx, "key1", "value1", 1*time.Minute)
	setString(t, mc, ctx, "key2", "value2", 1*time.Minute)
	setString(t, mc, ctx, "key3", "value3", 1*time.Minute)

	if mc.Size() != 3 {
		t.Errorf("Expected size 3, got %d", mc.Size())
	}

	// Clear
	mc.Clear()

	if mc.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", mc.Size())
	}

	// Verify LRU list is also cleared
	if mc.lru.size != 0 {
		t.Errorf("Expected LRU size 0 after clear, got %d", mc.lru.size)
	}
}

func TestLRUDelete(t *testing.T) {
	mc, err := NewMemoryCache(MemoryCacheConfig{
		MaxSize:         3,
		CleanupInterval: 1 * time.Minute,
		DefaultTTL:      1 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer mc.Close()

	ctx := context.Background()

	// Add items
	setString(t, mc, ctx, "key1", "value1", 1*time.Minute)
	setString(t, mc, ctx, "key2", "value2", 1*time.Minute)
	setString(t, mc, ctx, "key3", "value3", 1*time.Minute)

	initialLRUSize := mc.lru.size
	if initialLRUSize != 3 {
		t.Errorf("Expected LRU size 3, got %d", initialLRUSize)
	}

	// Delete key2
	mc.Delete(ctx, "key2")

	if mc.Size() != 2 {
		t.Errorf("Expected size 2 after delete, got %d", mc.Size())
	}

	// Verify LRU list size decreased
	if mc.lru.size != 2 {
		t.Errorf("Expected LRU size 2 after delete, got %d", mc.lru.size)
	}

	// Verify key2 doesn't exist
	if _, err := mc.Get(ctx, "key2"); err == nil {
		t.Error("key2 should not exist after delete")
	}
}
