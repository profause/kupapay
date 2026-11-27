package ratelimit

import (
	"context"

	"github.com/go-redis/redis_rate/v10"
	"github.com/profause/kupapay/pkg/cache"
)

// redisRateLimiter implements RateLimiter on top of redis_rate. Conversion
// between our Limit/Result types and redis_rate's stops at this file — no
// other package imports redis_rate.
type redisRateLimiter struct {
	limiter *redis_rate.Limiter
	redis   *cache.Redis
}

func newRedisRateLimiter(r *cache.Redis) *redisRateLimiter {
	return &redisRateLimiter{
		limiter: redis_rate.NewLimiter(r.Client()),
		redis:   r,
	}
}

func (r *redisRateLimiter) Allow(ctx context.Context, key string, limit Limit) (*Result, error) {
	res, err := r.limiter.Allow(ctx, r.redis.PrefixedKey(key), toRedisLimit(limit))
	if err != nil {
		return nil, err
	}
	return fromRedisResult(res), nil
}

func toRedisLimit(l Limit) redis_rate.Limit {
	return redis_rate.Limit{Rate: l.Rate, Burst: l.Burst, Period: l.Period}
}

func fromRedisResult(r *redis_rate.Result) *Result {
	if r == nil {
		return nil
	}
	return &Result{
		Allowed:    r.Allowed,
		Remaining:  r.Remaining,
		RetryAfter: r.RetryAfter,
		ResetAfter: r.ResetAfter,
	}
}
