package db

import (
	"context"
	"fmt"
	"goshop/internal/config"
	"goshop/internal/db/sqlc"
	"goshop/internal/utils"
	"goshop/pkg/logger"
	"goshop/pkg/pgx"
	"time"

	pgxv5 "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
)

var DB sqlc.Querier
var DBPool *pgxpool.Pool

func InitDB() error {
	connStr := config.NewConfig().DNS()

	conf, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return fmt.Errorf("error parsing DB config: %w", err)
	}

	sqlLogger := utils.NewLoggerWithPath("sql.log", "info")

	conf.ConnConfig.Tracer = &tracelog.TraceLog{
		Logger: &pgx.PgxZerologTracer{
			Logger:         *sqlLogger,
			SlowQueryLimit: 500 * time.Millisecond,
		},
		LogLevel: tracelog.LogLevelWarn,
	}

	// Dynamic database connection pool configuration via environment variables
	conf.MaxConns = int32(utils.GetIntEnv("DB_MAX_CONNS", 20))
	conf.MinConns = int32(utils.GetIntEnv("DB_MIN_CONNS", 5))
	conf.MaxConnLifetime = time.Duration(utils.GetIntEnv("DB_MAX_CONN_LIFETIME_MIN", 30)) * time.Minute
	conf.MaxConnIdleTime = time.Duration(utils.GetIntEnv("DB_MAX_CONN_IDLE_MIN", 5)) * time.Minute
	conf.HealthCheckPeriod = 1 * time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	DBPool, err = pgxpool.NewWithConfig(ctx, conf)
	if err != nil {
		return fmt.Errorf("error creating DB Pool: %w", err)
	}

	DB = sqlc.New(DBPool)

	if err = DBPool.Ping(ctx); err != nil {
		return fmt.Errorf("error pinging DB: %w", err)
	}

	logger.Log.Info().
		Int32("max_conns", conf.MaxConns).
		Int32("min_conns", conf.MinConns).
		Msg("✅ Connected to PostgreSQL database pool")

	return nil
}

func BeginTx(ctx context.Context) (pgxv5.Tx, error) {
	if DBPool == nil {
		return nil, fmt.Errorf("database connection pool is not initialized")
	}
	return DBPool.Begin(ctx)
}