package fx

import (
	"github.com/profause/kupapay/pkg/database"
	"github.com/profause/kupapay/pkg/env"
	"go.uber.org/fx"
)

var InfraModule = fx.Module("infra",
	fx.Provide(
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
	),
)
