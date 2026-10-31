package cache

import "time"

type RedisCacheService interface {
	Set(key string, value any, ttl time.Duration) error
	SetNX(key string, value any, ttl time.Duration) (bool, error)
	Get(key string, dest any) error
	Clear(pattern string) error
	Exists(key string) (bool, error)
	Del(key string) error
	Incr(key string) (int64, error)
	Expire(key string, ttl time.Duration) error
	TTL(key string) (time.Duration, error)
}
