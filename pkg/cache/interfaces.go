package cache

import "time"

type RedisCacheService interface {
	Set(key string, value any, ttl time.Duration) error
	Get(key string, dest any) error
	Clear(pattern string) error
	Exists(key string) (bool, error)
}
