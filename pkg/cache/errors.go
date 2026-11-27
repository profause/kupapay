package cache

import (
	"time"

	"github.com/profause/kupapay/pkg/customerror"
)

// Cache-specific errors
var (
	// ErrKeyNotFound is returned when a cache key does not exist
	ErrKeyNotFound = customerror.New(customerror.ErrCodeNotFound, "cache key not found")

	// ErrKeyExpired is returned when a cache key has expired
	ErrKeyExpired = customerror.New(customerror.ErrCodeExpired, "cache key expired")

	// ErrSerializationFailed is returned when value serialization fails
	ErrSerializationFailed = customerror.New(customerror.ErrCodeSerialization, "failed to serialize value")

	// ErrDeserializationFailed is returned when value deserialization fails
	ErrDeserializationFailed = customerror.New(customerror.ErrCodeDeserialization, "failed to deserialize value")

	// ErrTypeMismatch is returned when cached value type doesn't match expected type
	ErrTypeMismatch = customerror.New(customerror.ErrCodeTypeMismatch, "cached value type mismatch")

	// ErrLockFailed is returned when distributed lock acquisition fails
	ErrLockFailed = customerror.New(customerror.ErrCodeLockFailed, "failed to acquire lock")

	// ErrLockNotHeld is returned when trying to release a lock that is not held
	ErrLockNotHeld = customerror.New(customerror.ErrCodeLockFailed, "lock not held")

	// ErrRateLimitExceeded is returned when rate limit is exceeded
	ErrRateLimitExceeded = customerror.New(customerror.ErrCodeRateLimited, "rate limit exceeded")

	// ErrMaxSizeExceeded is returned when cache item count exceeds maximum
	ErrMaxSizeExceeded = customerror.New(customerror.ErrCodeMaxSize, "maximum cache size exceeded")

	// ErrMemoryLimitExceeded is returned when memory usage exceeds limit
	ErrMemoryLimitExceeded = customerror.New(customerror.ErrCodeMemory, "memory limit exceeded")

	// ErrConnectionFailed is returned when cache connection fails
	ErrConnectionFailed = customerror.New(customerror.ErrCodeConnection, "cache connection failed")

	// ErrInvalidValue is returned when cache value is invalid
	ErrInvalidValue = customerror.New(customerror.ErrCodeInvalidInput, "invalid cache value")

	// ErrNilValue is returned when attempting to cache a nil value
	ErrNilValue = customerror.New(customerror.ErrCodeInvalidInput, "cannot cache nil value")

	// ErrInvalidKey is returned when cache key is invalid
	ErrInvalidKey = customerror.New(customerror.ErrCodeInvalidInput, "invalid cache key")
)

// WithKey adds the cache key to error context
func WithKey(err *customerror.Error, key string) *customerror.Error {
	return err.WithKey(key)
}

// WithExpiration adds expiration time to error context
func WithExpiration(err *customerror.Error, expiration time.Time) *customerror.Error {
	return err.WithContext("expiration", expiration.Format(time.RFC3339))
}

// WithTTL adds time-to-live to error context
func WithTTL(err *customerror.Error, ttl time.Duration) *customerror.Error {
	return err.WithContext("ttl_seconds", ttl.Seconds())
}

// WithSize adds size information to error context
func WithSize(err *customerror.Error, size int64) *customerror.Error {
	return err.WithContext("size_bytes", size)
}

// WithMaxSize adds maximum size to error context
func WithMaxSize(err *customerror.Error, maxSize int) *customerror.Error {
	return err.WithContext("max_size", maxSize)
}

// WithCurrentSize adds current size to error context
func WithCurrentSize(err *customerror.Error, currentSize int) *customerror.Error {
	return err.WithContext("current_size", currentSize)
}

// WithMemoryUsage adds memory usage information to error context
func WithMemoryUsage(err *customerror.Error, current, max int64) *customerror.Error {
	return err.
		WithContext("current_memory_bytes", current).
		WithContext("max_memory_bytes", max)
}

// WithLockKey adds lock key to error context
func WithLockKey(err *customerror.Error, lockKey string) *customerror.Error {
	return err.WithContext("lock_key", lockKey)
}

// WithRateLimit adds rate limit information to error context
func WithRateLimit(err *customerror.Error, limit, window int) *customerror.Error {
	return err.
		WithContext("rate_limit", limit).
		WithContext("window_seconds", window)
}
