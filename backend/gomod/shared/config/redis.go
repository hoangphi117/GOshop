package config

import (
	"context"
	"crypto/tls"
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
		opt, err := redis.ParseURL(raw)
		if err != nil {
			return nil, raw, fmt.Errorf("parse REDIS_URL: %w", err)
		}
		if opt.TLSConfig == nil && strings.HasPrefix(raw, "rediss://") {
			opt.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		}
		// Redact credentials before the value can reach a log line.
		return opt, redactedRedisURL(raw), nil
	}

	addr := strings.TrimSpace(utils.GetEnv("REDIS_ADDR", "localhost:6379"))
	opt := &redis.Options{
		Addr:     addr,
		Username: utils.GetEnv("REDIS_USERNAME", ""),
		Password: utils.GetEnv("REDIS_PASSWORD", ""),
		DB:       utils.GetIntEnv("REDIS_DB", 0),
	}
	if utils.GetEnv("REDIS_TLS_ENABLED", "") == "true" {
		opt.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return opt, addr, nil
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

func NewRedisClient() *redis.Client {
	opt, target, err := newRedisOptions()
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to configure redis")
	}

	opt.PoolSize = 20
	opt.MinIdleConns = 5
	opt.DialTimeout = 5 * time.Second
	opt.ReadTimeout = 3 * time.Second
	opt.WriteTimeout = 3 * time.Second

	client := redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = client.Ping(ctx).Result()
	if err != nil {
		// The target is logged so a bad REDIS_ADDR is visible without a shell.
		logger.Log.Fatal().Err(err).Str("target", target).Msg("Failed to connect to redis")
	}

	logger.Log.Info().Str("target", target).Msg("Connected to Redis!")

	return client
}
