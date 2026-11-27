package cache

import "github.com/profause/kupapay/pkg/logger"

// Backend identifies which Cache implementation NewCache produced.
// Locker and RateLimiter consult this to pick a Redis- or memory-backed impl.
type Backend string

const (
	BackendRedis  Backend = "redis"
	BackendMemory Backend = "memory"
)

// Options configures NewCache. The selection rule is implicit: if Redis is
// configured AND instantiation succeeds, use it; otherwise fall back to
// memory. There is no Type knob — wiring is driven by config presence.
type Options struct {
	// Redis is the Redis configuration. nil or empty (Validate fails) means
	// "no Redis configured" — NewCache returns a memory backend.
	Redis *RedisConfig

	// Memory is the memory-cache configuration used as the primary backend
	// (when Redis is absent) or as the fallback (when Redis fails to
	// instantiate). nil uses sensible defaults via NewMemoryCache.
	Memory *MemoryCacheConfig

	// Logger receives warnings when Redis is unavailable and we fall back to
	// memory. Optional — nil disables those warnings.
	Logger *logger.Logger
}

// NewCache picks the backend per Options. Selection order:
//  1. If opts.Redis is non-nil and Validate passes, try NewRedis.
//     Success → returns Redis. Failure → log warning, continue to step 2.
//  2. Return a memory cache (NewMemoryCache always succeeds with defaults).
//
// The returned Backend tells callers which impl they got, so Locker and
// RateLimiter constructors can pick the matching backend without
// re-inspecting the config.
func NewCache(opts Options) (Cache, Backend, error) {
	if opts.Redis != nil && opts.Redis.Validate() == nil {
		if opts.Logger != nil && opts.Redis.Logger == nil {
			opts.Redis.Logger = opts.Logger
		}
		redis, err := NewRedis(opts.Redis)
		if err == nil {
			return redis, BackendRedis, nil
		}
		if opts.Logger != nil {
			opts.Logger.Warnw("redis cache unavailable, falling back to memory", "error", err)
		}
	}

	mem, err := NewMemoryCache(memoryConfig(opts.Memory))
	if err != nil {
		return nil, "", err
	}
	return mem, BackendMemory, nil
}

func memoryConfig(cfg *MemoryCacheConfig) MemoryCacheConfig {
	if cfg == nil {
		return MemoryCacheConfig{}
	}
	return *cfg
}
