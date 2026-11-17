package service

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisLimiter is the per-caller daily quota.
//
// INCR + EXPIRE is two round trips, so a crash between them would leave a
// counter with no TTL and a caller blocked forever. A single Lua script is the
// boring fix: one atomic call, no partial state, and it is 6 lines.
type redisLimiter struct {
	rdb   *redis.Client
	limit int64
	now   func() time.Time // injectable for tests
}

// incrWithTTL increments key and sets the TTL only when the key is new.
// redis.call returns the post-INCR value, so the caller keeps counting.
var incrWithTTL = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 then
  redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return n
`)

// NewRedisLimiter returns a daily limiter. limit <= 0 disables the check
// (returns the noop limiter) so a misconfigured env cannot block every chat.
func NewRedisLimiter(rdb *redis.Client, limit int64) Limiter {
	if rdb == nil || limit <= 0 {
		return noopLimiter{}
	}
	return &redisLimiter{rdb: rdb, limit: limit, now: time.Now}
}

func quotaKey(callerKey string, now time.Time) string {
	return fmt.Sprintf("assistant:quota:%s:%s", callerKey, now.UTC().Format("20060102"))
}

// Allow counts this call and reports whether the caller stays under the limit.
// Redis failures fail open: a broken cache must not stop paying customers.
func (l *redisLimiter) Allow(ctx context.Context, callerKey string) (bool, error) {
	if callerKey == "" {
		return true, nil
	}
	key := quotaKey(callerKey, l.now())
	n, err := incrWithTTL.Run(ctx, l.rdb, []string{key}, int64(24*time.Hour/time.Second)).Int64()
	if err != nil {
		return true, fmt.Errorf("quota: %w", err)
	}
	return n <= l.limit, nil
}
