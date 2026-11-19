package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Render Key Value publishes redis:// (internal) and rediss:// (external).
// Before newRedisOptions existed, the scheme stayed inside Addr and every
// connection failed at DNS or dropped at the handshake.
func TestNewRedisOptions_URL(t *testing.T) {
	t.Setenv("REDIS_URL", "rediss://default:s3cret@red-abc123:6379")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("REDIS_TLS_ENABLED", "")

	opt, target, err := newRedisOptions()
	require.NoError(t, err)

	assert.Equal(t, "red-abc123:6379", opt.Addr, "scheme must not survive into Addr")
	assert.Equal(t, "default", opt.Username)
	assert.Equal(t, "s3cret", opt.Password)
	require.NotNil(t, opt.TLSConfig, "rediss:// must enable TLS")
	// The password must never reach a log line.
	assert.NotContains(t, target, "s3cret")
	assert.Contains(t, target, "red-abc123:6379")
}

func TestNewRedisOptions_PlainInternalURL(t *testing.T) {
	t.Setenv("REDIS_URL", "redis://red-abc123:6379")
	t.Setenv("REDIS_ADDR", "")

	opt, target, err := newRedisOptions()
	require.NoError(t, err)

	assert.Equal(t, "red-abc123:6379", opt.Addr)
	assert.Equal(t, "", opt.Username)
	assert.Equal(t, "", opt.Password)
	assert.Nil(t, opt.TLSConfig, "plaintext internal URL must not use TLS")
	assert.Equal(t, "red-abc123:6379", target)
}

func TestNewRedisOptions_SeparateFields(t *testing.T) {
	t.Setenv("REDIS_URL", "")
	t.Setenv("REDIS_ADDR", "redis:6379")
	t.Setenv("REDIS_DB", "3")

	opt, target, err := newRedisOptions()
	require.NoError(t, err)

	assert.Equal(t, "redis:6379", opt.Addr)
	assert.Equal(t, 3, opt.DB)
	assert.Nil(t, opt.TLSConfig)
	assert.Equal(t, "redis:6379", target)
}

// A leading space is what a copy-paste leaves behind, and it turns into a DNS
// failure that reads like a network problem.
func TestNewRedisOptions_WhitespaceTolerated(t *testing.T) {
	t.Setenv("REDIS_URL", "")
	t.Setenv("REDIS_ADDR", "  red-abc123:6379  ")

	opt, _, err := newRedisOptions()
	require.NoError(t, err)
	assert.Equal(t, "red-abc123:6379", opt.Addr)
}

func TestNewRedisOptions_TLSFlagForSeparateFields(t *testing.T) {
	t.Setenv("REDIS_URL", "")
	t.Setenv("REDIS_ADDR", "red-abc123:6379")
	t.Setenv("REDIS_TLS_ENABLED", "true")

	opt, _, err := newRedisOptions()
	require.NoError(t, err)
	require.NotNil(t, opt.TLSConfig)
}

// A malformed URL must fail at startup, not on the first command.
func TestNewRedisOptions_BadURL(t *testing.T) {
	t.Setenv("REDIS_URL", "not a url")

	_, _, err := newRedisOptions()
	require.Error(t, err)
}

// The optional variant is what lets a deployment run without a shared cache:
// unset must mean "no Redis", not "dial localhost and retry five times".
func TestNewOptionalRedisOptions_UnsetMeansNoRedis(t *testing.T) {
	t.Setenv("REDIS_URL", "")
	t.Setenv("REDIS_ADDR", "")

	opt, err := NewOptionalRedisOptions()
	require.NoError(t, err)
	assert.Nil(t, opt, "an unset address must yield no client, not a localhost client")
}

func TestNewOptionalRedisOptions_ConfiguredReturnsOptions(t *testing.T) {
	t.Setenv("REDIS_URL", "")
	t.Setenv("REDIS_ADDR", "red-abc123:6379")

	opt, err := NewOptionalRedisOptions()
	require.NoError(t, err)
	require.NotNil(t, opt)
	assert.Equal(t, "red-abc123:6379", opt.Addr)
}

func TestPingRedis_NilIsError(t *testing.T) {
	require.Error(t, PingRedis(nil, 50*time.Millisecond))
}

// A managed Redis caps total connections, not connections per client, so the
// stack's seventeen processes have to divide the budget rather than each take
// go-redis' default 10 x GOMAXPROCS.
func TestNewOptionalRedisOptions_HonoursPoolCap(t *testing.T) {
	t.Setenv("REDIS_URL", "redis://red-abc123:6379")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("REDIS_MAX_CONNS", "2")

	opt, err := NewOptionalRedisOptions()
	require.NoError(t, err)
	require.NotNil(t, opt)
	assert.Equal(t, 2, opt.PoolSize)
	assert.LessOrEqual(t, opt.MinIdleConns, 2, "idle floor must not exceed the cap")
}

func TestNewOptionalRedisOptions_UnsetCapKeepsDefault(t *testing.T) {
	t.Setenv("REDIS_URL", "redis://red-abc123:6379")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("REDIS_MAX_CONNS", "")

	opt, err := NewOptionalRedisOptions()
	require.NoError(t, err)
	require.NotNil(t, opt)
	assert.Equal(t, 0, opt.PoolSize, "0 means go-redis picks its own default")
}
