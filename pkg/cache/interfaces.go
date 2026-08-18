package cache

import "time"

type RedisCacheService interface {
	Set(key string, value any, ttl time.Duration) error
	SetNX(key string, value any, ttl time.Duration) (bool, error)
	Get(key string, dest any) error
	Clear(pattern string) error
	Exists(key string) (bool, error)
}
