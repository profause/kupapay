package fx

import (
	"context"
	"strings"
	"time"

	"github.com/profause/kupapay/pkg/cache"
	"github.com/profause/kupapay/pkg/config"
	"github.com/profause/kupapay/pkg/database"
	"github.com/profause/kupapay/pkg/env"
	"github.com/profause/kupapay/pkg/logger"
	"github.com/profause/kupapay/pkg/ratelimit"
	"github.com/profause/kupapay/pkg/validation"
	"go.uber.org/fx"
)

var InfraModule = fx.Module("infra",
	validation.Module,
	fx.Provide(
		// Config
		func() (*config.Config, error) {
			svc := config.NewService()
			if err := svc.Load(); err != nil {
				return nil, err
			}
			return svc.Get(), nil
		},

		// Config Service
		func() (config.Service, error) {
			svc := config.NewService()
			if err := svc.Load(); err != nil {
				return nil, err
			}
			return svc, nil
		},

		// Logger
		func(cfg *config.Config) (*logger.Logger, error) {
			opts := make([]logger.Option, 0, 2)
			if cfg.Logger.Caller {
				opts = append(opts, logger.WithCaller())
			}
			if cfg.Logger.RotateEnabled {
				opts = append(opts, logger.WithRotation(
					cfg.Logger.RotateMaxSizeMB,
					cfg.Logger.RotateMaxBackups,
					cfg.Logger.RotateMaxAgeDays,
					cfg.Logger.RotateCompressLog,
				))
			}

			svc, err := logger.NewService(
				cfg.Logger.Level,
				cfg.Logger.Format,
				cfg.Logger.Output,
				opts...,
			)
			if err != nil {
				return nil, err
			}
			return svc.Get(), nil
		},
		func() (*database.PostgresSQL, error) {
			dbConfig := &database.PostgresSQLConfig{
				Host:            env.GetString("DB_HOST", "localhost"),
				Port:            env.GetInt("DB_PORT", 5432),
				User:            env.GetString("DB_USER", "postgres"),
				Password:        env.GetString("DB_PASSWORD", "password"),
				DBName:          env.GetString("DB_NAME", "kupapay"),
				SSLMode:         env.GetString("DB_SSL_MODE", "disable"),
				MaxConns:        env.GetInt("DB_MAX_CONNS", 10),
				MinConns:        env.GetInt("DB_MIN_CONNS", 5),
				MaxIdleConns:    env.GetInt("DB_MAX_IDLE_CONNS", 5),
				MaxConnLifetime: env.GetInt("DB_MAX_CONN_LIFETIME", 30),
				PingTimeout:     env.GetInt("DB_PING_TIMEOUT", 5),
			}
			return database.NewPostgresSQL(dbConfig)
		},

		// Cache — backend chosen automatically: if Redis is configured and
		// reachable, use it; otherwise (or on Redis failure) fall back to an
		// in-memory cache. The selected backend flows through fx as
		// cache.Backend so Locker/RateLimiter pick matching impls.
		func(lc fx.Lifecycle, cfg *config.Config, log *logger.Logger) (cache.Cache, cache.Backend, error) {
			opts := cache.Options{
				Redis: &cache.RedisConfig{
					Host:               cfg.Redis.Host,
					Port:               cfg.Redis.Port,
					Username:           cfg.Redis.Username,
					Password:           cfg.Redis.Password,
					DB:                 cfg.Redis.DB,
					KeyPrefix:          cfg.Redis.KeyPrefix,
					ClusterEnabled:     cfg.Redis.ClusterEnabled,
					ClusterAddrs:       cfg.Redis.ClusterAddrs,
					TLSEnabled:         cfg.Redis.TLSEnabled,
					SentinelEnabled:    cfg.Redis.SentinelEnabled,
					SentinelMasterName: cfg.Redis.SentinelMasterName,
					SentinelAddrs:      cfg.Redis.SentinelAddrs,
				},
				Memory: &cache.MemoryCacheConfig{
					MaxSize:         cfg.Cache.MemoryMaxSize,
					CleanupInterval: time.Duration(cfg.Cache.MemoryCleanupSec) * time.Second,
					DefaultTTL:      time.Duration(cfg.Cache.MemoryDefaultTTLSec) * time.Second,
				},
				Logger: log,
			}
			c, backend, err := cache.NewCache(opts)
			if err != nil {
				return nil, "", err
			}

			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					if err := c.Close(); err != nil {
						log.Errorw("failed to close cache backend", "backend", string(backend), "error", err)
						return err
					}
					return nil
				},
			})

			return c, backend, nil
		},
		// Rate-limit middleware factory. ratelimit.New picks redis_rate when
		// the cache is Redis-backed and falls back to a fixed-window in-process
		// counter otherwise; handlers inject *ratelimit.Middleware and attach
		// per-route policies, cmd/api wires the global default.
		func(c cache.Cache, cfg *config.Config, log *logger.Logger) *ratelimit.Middleware {
			mw := ratelimit.New(c, log)
			if prefix := strings.TrimSpace(cfg.RateLimit.KeyPrefix); prefix != "" {
				mw = mw.WithKeyPrefix(prefix)
			}
			return mw
		},
	),
)
