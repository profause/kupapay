package cache

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/profause/kupapay/pkg/logger"
	"github.com/redis/go-redis/v9"
)

type Redis struct {
	client      redis.UniversalClient
	logger      *logger.Logger
	clusterMode bool
	keyPrefix   string
}

type RedisConfig struct {
	Host      string
	Port      int
	Username  string
	Password  string
	DB        int
	KeyPrefix string

	ClusterEnabled bool
	TLSEnabled     bool
	ClusterAddrs   []string

	SentinelEnabled    bool
	SentinelMasterName string
	SentinelAddrs      []string

	Logger *logger.Logger
}

const (
	MinRedisPort = 1
	MaxRedisPort = 65535
	MinRedisDB   = 0
	MaxRedisDB   = 15
)

func (c *RedisConfig) Validate() error {
	if c.ClusterEnabled {
		if len(c.ClusterAddrs) == 0 {
			return fmt.Errorf("ClusterAddrs must not be empty when ClusterEnabled is true")
		}
		return nil
	}

	if c.SentinelEnabled {
		if c.SentinelMasterName == "" {
			return fmt.Errorf("SentinelMasterName must not be empty when SentinelEnabled is true")
		}
		if len(c.SentinelAddrs) == 0 {
			return fmt.Errorf("SentinelAddrs must not be empty when SentinelEnabled is true")
		}
		return nil
	}

	if c.Host == "" {
		return fmt.Errorf("Host must not be empty")
	}
	if c.Port < MinRedisPort || c.Port > MaxRedisPort {
		return fmt.Errorf("Port must be between %d and %d (got %d)", MinRedisPort, MaxRedisPort, c.Port)
	}
	if c.DB < MinRedisDB || c.DB > MaxRedisDB {
		return fmt.Errorf("DB must be between %d and %d (got %d)", MinRedisDB, MaxRedisDB, c.DB)
	}
	return nil
}

func normalizeRedisKeyPrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return ""
	}
	if strings.HasSuffix(prefix, ":") {
		return prefix
	}
	return prefix + ":"
}

func (r *Redis) PrefixedKey(key string) string {
	if r == nil || r.keyPrefix == "" {
		return key
	}
	return r.keyPrefix + key
}

func (r *Redis) PrefixedKeys(keys []string) []string {
	if r == nil || r.keyPrefix == "" || len(keys) == 0 {
		return keys
	}

	prefixed := make([]string, 0, len(keys))
	for _, key := range keys {
		prefixed = append(prefixed, r.PrefixedKey(key))
	}
	return prefixed
}

func (r *Redis) prefixedPattern(pattern string) string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		if r == nil || r.keyPrefix == "" {
			return "*"
		}
		return r.keyPrefix + "*"
	}
	return r.PrefixedKey(pattern)
}

func (r *Redis) scanKeys(ctx context.Context, pattern string) ([]string, error) {
	if cluster, ok := r.client.(*redis.ClusterClient); ok {
		var (
			keys []string
			mu   sync.Mutex
		)

		err := cluster.ForEachMaster(ctx, func(ctx context.Context, master *redis.Client) error {
			iter := master.Scan(ctx, 0, pattern, 0).Iterator()
			local := make([]string, 0)
			for iter.Next(ctx) {
				local = append(local, iter.Val())
			}
			if err := iter.Err(); err != nil {
				return err
			}
			if len(local) > 0 {
				mu.Lock()
				keys = append(keys, local...)
				mu.Unlock()
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		return keys, nil
	}

	iter := r.client.Scan(ctx, 0, pattern, 0).Iterator()
	keys := make([]string, 0)
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	return keys, nil
}

func (r *Redis) deleteRawKeys(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	if _, ok := r.client.(*redis.ClusterClient); ok && len(keys) > 1 {
		for _, key := range keys {
			if err := r.client.Del(ctx, key).Err(); err != nil {
				return err
			}
		}
		return nil
	}

	return r.client.Del(ctx, keys...).Err()
}

func NewRedis(cfg *RedisConfig) (*Redis, error) {
	if cfg == nil {
		return nil, fmt.Errorf("redis config is required")
	}

	log := cfg.Logger
	if log == nil {
		log = logger.NewNop()
	}

	if err := cfg.Validate(); err != nil {
		log.Errorw("invalid_redis_config", "error", err.Error(), "cluster_enabled", cfg.ClusterEnabled)
		return nil, fmt.Errorf("invalid Redis configuration: %w", err)
	}

	keyPrefix := normalizeRedisKeyPrefix(cfg.KeyPrefix)
	var tlsConfig *tls.Config
	if cfg.TLSEnabled {
		tlsConfig = &tls.Config{InsecureSkipVerify: true}
	}

	var (
		client      redis.UniversalClient
		clusterMode bool
	)

	switch {
	case cfg.ClusterEnabled:
		clusterMode = true
		client = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:          cfg.ClusterAddrs,
			Username:       cfg.Username,
			Password:       cfg.Password,
			MaxRedirects:   3,
			RouteByLatency: true,
			TLSConfig:      tlsConfig,
		})
		log.Infow("initializing_redis_cluster", "addrs", cfg.ClusterAddrs, "key_prefix", keyPrefix)
	case cfg.SentinelEnabled:
		client = redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:    cfg.SentinelMasterName,
			SentinelAddrs: cfg.SentinelAddrs,
			Username:      cfg.Username,
			Password:      cfg.Password,
			DB:            cfg.DB,
		})
		log.Infow("initializing_redis_sentinel", "master", cfg.SentinelMasterName, "sentinels", cfg.SentinelAddrs, "db", cfg.DB, "key_prefix", keyPrefix)
	default:
		client = redis.NewClient(&redis.Options{
			Addr:      fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
			Username:  cfg.Username,
			Password:  cfg.Password,
			DB:        cfg.DB,
			TLSConfig: tlsConfig,
		})
		log.Infow("initializing_redis_standalone", "host", cfg.Host, "port", cfg.Port, "db", cfg.DB, "key_prefix", keyPrefix)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Errorw("redis_connection_failed", "cluster_mode", clusterMode, "error", err.Error())
		return nil, ErrConnectionFailed.Wrap(err)
	}

	return &Redis{
		client:      client,
		logger:      log,
		clusterMode: clusterMode,
		keyPrefix:   keyPrefix,
	}, nil
}

func (r *Redis) Get(ctx context.Context, key string) (string, error) {
	if strings.TrimSpace(key) == "" {
		return "", WithKey(ErrInvalidKey, key)
	}

	value, err := r.client.Get(ctx, r.PrefixedKey(key)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", WithKey(ErrKeyNotFound, key)
		}
		return "", WithKey(ErrConnectionFailed.Wrap(err), key)
	}
	return value, nil
}

func (r *Redis) GetDel(ctx context.Context, key string) (string, error) {
	if strings.TrimSpace(key) == "" {
		return "", WithKey(ErrInvalidKey, key)
	}

	value, err := r.client.GetDel(ctx, r.PrefixedKey(key)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", WithKey(ErrKeyNotFound, key)
		}
		return "", WithKey(ErrConnectionFailed.Wrap(err), key)
	}
	return value, nil
}

func (r *Redis) GetJSON(ctx context.Context, key string, dest any) error {
	if dest == nil {
		return fmt.Errorf("destination cannot be nil")
	}

	data, err := r.Get(ctx, key)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(data), dest); err != nil {
		return WithKey(ErrDeserializationFailed.Wrap(err), key)
	}
	return nil
}

func (r *Redis) GetJSONWithRefresh(ctx context.Context, key string, dest any, extendTTL time.Duration) error {
	if strings.TrimSpace(key) == "" {
		return WithKey(ErrInvalidKey, key)
	}
	if dest == nil {
		return fmt.Errorf("destination cannot be nil")
	}
	if extendTTL <= 0 {
		return r.GetJSON(ctx, key, dest)
	}

	value, err := r.client.GetEx(ctx, r.PrefixedKey(key), extendTTL).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return WithKey(ErrKeyNotFound, key)
		}
		return WithKey(ErrConnectionFailed.Wrap(err), key)
	}
	if err := json.Unmarshal(value, dest); err != nil {
		return WithKey(ErrDeserializationFailed.Wrap(err), key)
	}
	return nil
}

func (r *Redis) Set(ctx context.Context, key string, value string, expiration time.Duration) error {
	if strings.TrimSpace(key) == "" {
		return WithKey(ErrInvalidKey, key)
	}

	if err := r.client.Set(ctx, r.PrefixedKey(key), value, expiration).Err(); err != nil {
		return WithKey(ErrConnectionFailed.Wrap(err), key)
	}
	return nil
}

func (r *Redis) SetJSON(ctx context.Context, key string, value any, expiration time.Duration) error {
	if value == nil {
		return WithKey(ErrNilValue, key)
	}

	payload, err := json.Marshal(value)
	if err != nil {
		return WithKey(ErrSerializationFailed.Wrap(err), key)
	}
	return r.Set(ctx, key, string(payload), expiration)
}

func (r *Redis) Delete(ctx context.Context, keys ...string) error {
	prefixedKeys := r.PrefixedKeys(keys)
	if err := r.deleteRawKeys(ctx, prefixedKeys...); err != nil {
		return ErrConnectionFailed.Wrap(err)
	}
	return nil
}

func (r *Redis) DeleteByPattern(ctx context.Context, pattern string) error {
	keys, err := r.scanKeys(ctx, r.prefixedPattern(pattern))
	if err != nil {
		return ErrConnectionFailed.Wrap(err)
	}
	if err := r.deleteRawKeys(ctx, keys...); err != nil {
		return ErrConnectionFailed.Wrap(err)
	}
	return nil
}

func (r *Redis) SetIfAbsent(ctx context.Context, key string, value string, expiration time.Duration) (bool, error) {
	if strings.TrimSpace(key) == "" {
		return false, WithKey(ErrInvalidKey, key)
	}

	ok, err := r.client.SetNX(ctx, r.PrefixedKey(key), value, expiration).Result()
	if err != nil {
		return false, WithKey(ErrConnectionFailed.Wrap(err), key)
	}
	return ok, nil
}

func (r *Redis) Close() error {
	return r.client.Close()
}

func (r *Redis) Client() redis.UniversalClient {
	return r.client
}

func (r *Redis) IsClusterMode() bool {
	return r.clusterMode
}
