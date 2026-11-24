package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

type PostgresSQL struct {
	DB *bun.DB
}

type PostgresSQLConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	DBName          string
	SSLMode         string
	MaxConns        int
	MinConns        int
	MaxIdleConns    int
	MaxConnLifetime int
	PingTimeout     int
}

func NewPostgresSQL(config *PostgresSQLConfig) (*PostgresSQL, error) {
	// TODO: Implement PostgreSQL connection
	databaseDNS := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		config.User,
		config.Password,
		config.Host,
		config.Port,
		config.DBName,
		config.SSLMode,
	)
	databaseConnection := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(databaseDNS)))
	databaseConnection.SetMaxOpenConns(config.MaxConns)
	databaseConnection.SetMaxIdleConns(config.MaxIdleConns)
	databaseConnection.SetConnMaxLifetime(time.Duration(config.MaxConnLifetime) * time.Minute)

	bunDB := bun.NewDB(databaseConnection, pgdialect.New())

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.PingTimeout)*time.Second)
	defer cancel()

	if err := bunDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database %e", err)
	}

	return &PostgresSQL{DB: bunDB}, nil
}
