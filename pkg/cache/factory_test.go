package cache

import "testing"

// TestNewCache_NoRedisConfig_FallsBackToMemory ensures that when Redis is not
// configured, the factory returns a working memory cache.
func TestNewCache_NoRedisConfig_FallsBackToMemory(t *testing.T) {
	c, backend, err := NewCache(Options{})
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	defer c.Close()
	if backend != BackendMemory {
		t.Fatalf("backend = %q, want %q", backend, BackendMemory)
	}
	if _, ok := c.(*MemoryCache); !ok {
		t.Fatalf("cache = %T, want *MemoryCache", c)
	}
}

// TestNewCache_InvalidRedisConfig_FallsBackToMemory ensures the auto fallback
// fires when Redis config is present but fails validation (no host, no
// cluster addrs, no sentinel addrs).
func TestNewCache_InvalidRedisConfig_FallsBackToMemory(t *testing.T) {
	c, backend, err := NewCache(Options{
		Redis: &RedisConfig{Host: ""}, // fails Validate
	})
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	defer c.Close()
	if backend != BackendMemory {
		t.Fatalf("backend = %q, want %q", backend, BackendMemory)
	}
}

// TestNewCache_RedisUnreachable_FallsBackToMemory covers the case where
// validation passes but NewRedis fails to connect (host:port refused).
// We point at an unused localhost port; NewRedis fails its initial Ping and
// the factory returns memory.
func TestNewCache_RedisUnreachable_FallsBackToMemory(t *testing.T) {
	c, backend, err := NewCache(Options{
		Redis: &RedisConfig{Host: "127.0.0.1", Port: 1}, // port 1 is reserved/refused
	})
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	defer c.Close()
	if backend != BackendMemory {
		t.Fatalf("backend = %q, want %q (Redis should have failed Ping)", backend, BackendMemory)
	}
}
