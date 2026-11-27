package cache

import (
	"context"
	"time"
)

// Cache defines the shared cache operations used by the application.
//
// DeleteByPattern accepts a glob pattern in the Redis sense ("foo:*",
// "user:*:cache"). The memory backend only honors trailing-wildcard prefix
// patterns ("foo:*"); anything more elaborate is silently treated as a literal
// key for memory but works fully against Redis. This matches every existing
// caller's usage (settings deletes "settings:*") and avoids implementing a
// general glob matcher we don't need.
type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	// GetDel atomically returns the value at key and removes it, so a value can
	// be observed by at most one caller. Returns ErrKeyNotFound if the key is
	// absent (or expired). Use it for single-use tokens where a Get-then-Delete
	// pair would race two concurrent consumers into both succeeding.
	GetDel(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, expiration time.Duration) error
	GetJSON(ctx context.Context, key string, dest any) error
	GetJSONWithRefresh(ctx context.Context, key string, dest any, extendTTL time.Duration) error
	SetJSON(ctx context.Context, key string, value any, expiration time.Duration) error
	Delete(ctx context.Context, keys ...string) error
	DeleteByPattern(ctx context.Context, pattern string) error
	SetIfAbsent(ctx context.Context, key string, value string, expiration time.Duration) (bool, error)
	Close() error
}

var _ Cache = (*Redis)(nil)
var _ Cache = (*MemoryCache)(nil)
