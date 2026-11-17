package app

import (
	"context"

	assistantservice "goshop/services/assistant/internal/service"

	"github.com/redis/go-redis/v9"
)

// guestDailyQuota is a quarter of the signed-in allowance: a guest is cheap to
// abuse (new session id = new bucket), and 10 questions is still generous.
const guestDailyQuotaDivisor = 4

// callerLimiter routes to the user or guest bucket by key prefix. Both keys are
// minted by the handler ("user:" vs "sess:"), so no extra parameter is needed.
type callerLimiter struct {
	member assistantservice.Limiter
	guest  assistantservice.Limiter
}

func newCallerLimiter(rdb *redis.Client, dailyPerUser int64) assistantservice.Limiter {
	guest := dailyPerUser / guestDailyQuotaDivisor
	if guest < 1 {
		guest = 1
	}
	return callerLimiter{
		member: assistantservice.NewRedisLimiter(rdb, dailyPerUser),
		guest:  assistantservice.NewRedisLimiter(rdb, guest),
	}
}

func (l callerLimiter) Allow(ctx context.Context, key string) (bool, error) {
	if len(key) >= 5 && key[:5] == "user:" {
		return l.member.Allow(ctx, key)
	}
	return l.guest.Allow(ctx, key)
}
