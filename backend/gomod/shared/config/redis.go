package config

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"goshop/shared/pkg/logger"
	"goshop/shared/pkg/utils"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Address  string
	Username string
	Password string
	DB       int
}

// newRedisOptions builds the connection options from the environment.
//
// Two ways in, because managed providers publish different shapes:
//   - REDIS_URL: a full redis:// or rediss:// URL. Parsed by go-redis itself,
//     so credentials and TLS come along without hand-splitting the string.
//     Preferred when set (Render Key Value's external URL, Upstash, Heroku).
//   - REDIS_ADDR / _USERNAME / _PASSWORD / _DB: host:port plus separate fields,
//     for the plaintext internal URL that needs no credentials.
//
// rediss:// requires TLS. Before this, the scheme in REDIS_URL was silently
// treated as part of the hostname, which surfaces as a DNS failure or, once
// the host happens to resolve, as "write: broken pipe" - the server drops a
// plaintext handshake it will never accept.
func newRedisOptions() (*redis.Options, string, error) {
	if raw := strings.TrimSpace(utils.GetEnv("REDIS_URL", "")); raw != "" {
		return redisOptionsFromURL(raw, true)
	}

	// No localhost fallback: in a container that address is the container
	// itself, and dialling it only buys seconds of retry noise at boot.
	addr := strings.TrimSpace(utils.GetEnv("REDIS_ADDR", ""))
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	opt := &redis.Options{
		Addr:     addr,
		Username: utils.GetEnv("REDIS_USERNAME", ""),
		Password: utils.GetEnv("REDIS_PASSWORD", ""),
		DB:       utils.GetIntEnv("REDIS_DB", 0),
	}
	if utils.GetEnv("REDIS_TLS_ENABLED", "") == "true" {
		opt.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	if maxConns := utils.GetIntEnv("REDIS_MAX_CONNS", 0); maxConns > 0 {
		opt.PoolSize = maxConns
	}
	return opt, addr, nil
}

func redisOptionsFromURL(raw string, allowTLS bool) (*redis.Options, string, error) {
	opt, err := redis.ParseURL(raw)
	if err != nil {
		return nil, redactedRedisURL(raw), fmt.Errorf("parse REDIS_URL: %w", err)
	}
	if allowTLS && opt.TLSConfig == nil && strings.HasPrefix(raw, "rediss://") {
		opt.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	// go-redis defaults PoolSize to 10 x GOMAXPROCS, which is fine for one
	// service and fatal for seventeen in one container against a managed Redis
	// that caps total connections. Render Key Value's free plan allows 25; the
	// stack's behaviour there was nine clients connecting and the rest failing
	// with a bare "write: broken pipe" - the server hanging up, not the address
	// being wrong. REDIS_MAX_CONNS sizes one process's share of that budget.
	if maxConns := utils.GetIntEnv("REDIS_MAX_CONNS", 0); maxConns > 0 {
		opt.PoolSize = maxConns
		if opt.MinIdleConns > maxConns {
			opt.MinIdleConns = maxConns
		}
	}
	// Redact credentials before the value can reach a log line.
	return opt, redactedRedisURL(raw), nil
}

// NewOptionalRedisOptions is for services that can run without Redis. It
// returns (nil, nil) when REDIS_URL/REDIS_ADDR is unset instead of falling
// back to localhost, which in a container means "no Redis anywhere".
//
// The client it builds never returns a usable Ping on a dead server - the TLS
// and address are right, the host is not - so callers must treat a ping
// failure as "run uncached", not as fatal.
func NewOptionalRedisOptions() (*redis.Options, error) {
	raw := strings.TrimSpace(utils.GetEnv("REDIS_URL", ""))
	if raw == "" && strings.TrimSpace(utils.GetEnv("REDIS_ADDR", "")) == "" {
		return nil, nil
	}
	opt, _, err := newRedisOptions()
	return opt, err
}

// PingRedis reports whether the target answers PING. Used at boot to decide
// between a shared cache and a process-local one.
func PingRedis(opt *redis.Options, timeout time.Duration) error {
	if opt == nil {
		return errors.New("redis not configured")
	}
	client := redis.NewClient(opt)
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return client.Ping(ctx).Err()
}

// redactedRedisURL strips credentials so a failed connection can be logged.
// It always returns host:port, never the scheme, so a log line reads the same
// way whether the address came from REDIS_URL or REDIS_ADDR.
func redactedRedisURL(raw string) string {
	hostport := raw
	if i := strings.Index(raw, "://"); i >= 0 {
		hostport = raw[i+3:]
	}
	// Keep only what follows the last '@': user:password@ drops out entirely.
	if at := strings.LastIndex(hostport, "@"); at >= 0 {
		hostport = hostport[at+1:]
	}
	return hostport
}

// NewRedisOptions exposes the same REDIS_URL / REDIS_ADDR resolution that
// NewRedisClient uses, for callers that build their own client because they
// must degrade instead of Fatal (worker, recommendations, seed).
func NewRedisOptions() (*redis.Options, error) {
	opt, _, err := newRedisOptions()
	return opt, err
}

// NewRedisClient is for code paths that cannot work without a shared cache.
// It logs the target and returns nil instead of exiting: a missing Redis is a
// degraded deployment, not a reason to kill the process, and the callers that
// matter (JWT and rate limiting) fall back to the in-memory layer in
// cache.NewRedisCacheService when the client is nil.
func NewRedisClient() *redis.Client {
	opt, target, err := newRedisOptions()
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to configure redis; continuing without a shared cache")
		return nil
	}

	// Honour REDIS_MAX_CONNS here too. redisOptionsFromURL already sized the
	// pool, but the lines below used to overwrite it with 20 regardless, so a
	// container running 17 services ignored the cap and the later services in
	// the boot sequence were hung up with "write: broken pipe".
	if maxConns := utils.GetIntEnv("REDIS_MAX_CONNS", 0); maxConns > 0 {
		opt.PoolSize = maxConns
		opt.MinIdleConns = 0
	} else {
		opt.PoolSize = 20
		opt.MinIdleConns = 5
	}
	opt.DialTimeout = 5 * time.Second
	opt.ReadTimeout = 3 * time.Second
	opt.WriteTimeout = 3 * time.Second

	client := redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err = client.Ping(ctx).Result(); err != nil {
		// The target is logged so a bad REDIS_ADDR is visible without a shell.
		logger.Log.Error().Err(err).Str("target", target).Msg("Redis unreachable; running with the in-memory cache only")
		_ = client.Close()
		return nil
	}

	logger.Log.Info().Str("target", target).Msg("Connected to Redis!")
	return client
}
