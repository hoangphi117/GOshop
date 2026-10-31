// Package db is the shared PostgreSQL helper: connection pools for the
// primary, read-replica and identity databases plus thin re-exports of the
// transaction manager in pkg/pgx.
package db

import (
	"context"
	"fmt"
	"goshop/internal/config"
	"goshop/pkg/logger"
	"goshop/pkg/pgx"
	"goshop/pkg/utils"
	"time"

	pgxv5 "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
)

// Pools only. Each service generates its own sqlc package from its own schema,
// so the monolith no longer exposes a shared Querier.
var DBPool *pgxpool.Pool

var ReadDBPool *pgxpool.Pool

var IdentityDBPool *pgxpool.Pool

func GetReadDBPool() *pgxpool.Pool {
	if ReadDBPool != nil {
		return ReadDBPool
	}
	return DBPool
}

// TxManager is the transaction manager shared by every service.
type TxManager = pgx.TxManager

func NewTxManager(pool *pgxpool.Pool) TxManager {
	return pgx.NewTxManager(pool)
}

func InitDB(serviceName ...string) error {
	return InitDBWithConfig(config.NewConfig(), serviceName...)
}

func InitDBWithConfig(cfg *config.Config, serviceName ...string) error {
	if cfg == nil {
		cfg = config.NewConfig()
	}
	connStr := cfg.DNS()

	conf, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return fmt.Errorf("error parsing DB config: %w", err)
	}
	if len(serviceName) > 0 && serviceName[0] == "order" {
		conf.ConnConfig.RuntimeParams["search_path"] = "orders,public"
	}

	sqlLogFile := "sql.log"
	svc := ""
	if len(serviceName) > 0 && serviceName[0] != "" {
		svc = serviceName[0]
		sqlLogFile = fmt.Sprintf("%s_sql.log", svc)
	} else if svc = utils.ServiceName(); svc != "" {
		sqlLogFile = fmt.Sprintf("%s_sql.log", svc)
	}

	sqlLogger := utils.NewLoggerWithPath(svc, sqlLogFile, "info")

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

	if err = DBPool.Ping(ctx); err != nil {
		return fmt.Errorf("error pinging DB: %w", err)
	}

	logger.Log.Info().
		Int32("max_conns", conf.MaxConns).
		Int32("min_conns", conf.MinConns).
		Msg("✅ Connected to PostgreSQL database pool")

	// Services that own a single database and read every other domain over
	// gRPC (analytics, chat) must not open pools into foreign databases.
	singleDBOnly := len(serviceName) > 0 && (serviceName[0] == "analytics" || serviceName[0] == "chat")
	if singleDBOnly {
		ReadDBPool = DBPool
		IdentityDBPool = DBPool
		return nil
	}

	// Read Replica initialization (defaults to DBPool if identical connection string)
	if cfg.ReadDNS() == cfg.DNS() {
		ReadDBPool = DBPool
	} else {
		readConf, err := pgxpool.ParseConfig(cfg.ReadDNS())
		if err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to parse Read DB config, falling back to primary DB")
			ReadDBPool = DBPool
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
			} else if err = ReadDBPool.Ping(ctx); err != nil {
				logger.Log.Warn().Err(err).Msg("Failed to ping Read DB pool, falling back to primary DB")
				ReadDBPool.Close()
				ReadDBPool = DBPool
			} else {
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
	} else {
		identityConf, err := pgxpool.ParseConfig(identityDNS)
		if err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to parse Identity DB config, falling back to primary DB")
			IdentityDBPool = DBPool
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
			} else if err = IdentityDBPool.Ping(ctx); err != nil {
				logger.Log.Warn().Err(err).Msg("Failed to ping Identity DB pool, falling back to primary DB")
				IdentityDBPool.Close()
				IdentityDBPool = DBPool
			} else {
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

func GetIdentityDBPool() *pgxpool.Pool {
	if IdentityDBPool != nil {
		return IdentityDBPool
	}
	return DBPool
}

func BeginIdentityTx(ctx context.Context) (pgxv5.Tx, error) {
	pool := GetIdentityDBPool()
	if pool == nil {
		return nil, fmt.Errorf("identity database connection pool is not initialized")
	}
	return pool.Begin(ctx)
}

func GetTx(ctx context.Context) (pgxv5.Tx, bool) {
	return pgx.GetTx(ctx)
}

func InjectTx(ctx context.Context, tx pgxv5.Tx) context.Context {
	return pgx.InjectTx(ctx, tx)
}
