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

var ReadDB sqlc.Querier
var ReadDBPool *pgxpool.Pool

var IdentityDB sqlc.Querier
var IdentityDBPool *pgxpool.Pool

func GetReadDB() sqlc.Querier {
	if ReadDB != nil {
		return ReadDB
	}
	return DB
}

func GetIdentityDB() sqlc.Querier {
	if IdentityDB != nil {
		return IdentityDB
	}
	return DB
}

func InitDB() error {
	cfg := config.NewConfig()
	connStr := cfg.DNS()

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

	// Read Replica initialization (defaults to DBPool if identical connection string)
	if cfg.ReadDNS() == cfg.DNS() {
		ReadDBPool = DBPool
		ReadDB = DB
	} else {
		readConf, err := pgxpool.ParseConfig(cfg.ReadDNS())
		if err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to parse Read DB config, falling back to primary DB")
			ReadDBPool = DBPool
			ReadDB = DB
		} else {
			readConf.MaxConns = int32(utils.GetIntEnv("DB_READ_MAX_CONNS", utils.GetIntEnv("DB_MAX_CONNS", 20)))
			readConf.MinConns = int32(utils.GetIntEnv("DB_READ_MIN_CONNS", utils.GetIntEnv("DB_MIN_CONNS", 5)))
			readConf.MaxConnLifetime = time.Duration(utils.GetIntEnv("DB_MAX_CONN_LIFETIME_MIN", 30)) * time.Minute
			readConf.MaxConnIdleTime = time.Duration(utils.GetIntEnv("DB_MAX_CONN_IDLE_MIN", 5)) * time.Minute
			readConf.HealthCheckPeriod = 1 * time.Minute

			ReadDBPool, err = pgxpool.NewWithConfig(ctx, readConf)
			if err != nil {
				logger.Log.Warn().Err(err).Msg("Failed to connect to Read DB pool, falling back to primary DB")
				ReadDBPool = DBPool
				ReadDB = DB
			} else {
				ReadDB = sqlc.New(ReadDBPool)
				logger.Log.Info().
					Str("host", readConf.ConnConfig.Host).
					Str("database", readConf.ConnConfig.Database).
					Int32("max_conns", readConf.MaxConns).
					Msg("✅ Connected to PostgreSQL Read-Replica database pool")
			}
		}
	}

	// Identity DB initialization (defaults to DBPool if identical or unset)
	identityDNS := cfg.IdentityDNS()
	if identityDNS == "" || identityDNS == cfg.DNS() {
		IdentityDBPool = DBPool
		IdentityDB = DB
	} else {
		identityConf, err := pgxpool.ParseConfig(identityDNS)
		if err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to parse Identity DB config, falling back to primary DB")
			IdentityDBPool = DBPool
			IdentityDB = DB
		} else {
			identityConf.MaxConns = int32(utils.GetIntEnv("DB_IDENTITY_MAX_CONNS", 15))
			identityConf.MinConns = int32(utils.GetIntEnv("DB_IDENTITY_MIN_CONNS", 3))
			identityConf.MaxConnLifetime = time.Duration(utils.GetIntEnv("DB_MAX_CONN_LIFETIME_MIN", 30)) * time.Minute
			identityConf.MaxConnIdleTime = time.Duration(utils.GetIntEnv("DB_MAX_CONN_IDLE_MIN", 5)) * time.Minute
			identityConf.HealthCheckPeriod = 1 * time.Minute

			IdentityDBPool, err = pgxpool.NewWithConfig(ctx, identityConf)
			if err != nil {
				logger.Log.Warn().Err(err).Msg("Failed to connect to Identity DB pool, falling back to primary DB")
				IdentityDBPool = DBPool
				IdentityDB = DB
			} else {
				IdentityDB = sqlc.New(IdentityDBPool)
				logger.Log.Info().
					Str("host", identityConf.ConnConfig.Host).
					Str("database", identityConf.ConnConfig.Database).
					Int32("max_conns", identityConf.MaxConns).
					Msg("✅ Connected to PostgreSQL Identity database pool")
			}
		}
	}

	return nil
}

func BeginTx(ctx context.Context) (pgxv5.Tx, error) {
	if DBPool == nil {
		return nil, fmt.Errorf("database connection pool is not initialized")
	}
	return DBPool.Begin(ctx)
}