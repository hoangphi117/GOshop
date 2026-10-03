package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisCacheService struct {
	ctx context.Context
	rdb *redis.Client
}

func NewRedisCacheService(rdb *redis.Client) RedisCacheService {
	return &redisCacheService{
		ctx: context.Background(),
		rdb: rdb,
	}
}

func (cs *redisCacheService) Set(key string, value any, ttl time.Duration) error {
	if cs.rdb == nil {
		return nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return cs.rdb.Set(cs.ctx, key, data, ttl).Err()
}

func (cs *redisCacheService) SetNX(key string, value any, ttl time.Duration) (bool, error) {
	if cs.rdb == nil {
		return true, nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		return false, err
	}

	return cs.rdb.SetNX(cs.ctx, key, data, ttl).Result()
}

func (cs *redisCacheService) Get(key string, dest any) error {
	if cs.rdb == nil {
		return redis.Nil
	}

	result, err := cs.rdb.Get(cs.ctx, key).Result()

	if err == redis.Nil {
		return err
	}

	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(result), dest)
}

func (cs *redisCacheService) Clear(pattern string) error {
	if cs.rdb == nil {
		return nil
	}

	cursor := uint64(0)
	for {
		keys, nextCursor, err := cs.rdb.Scan(cs.ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}

		if len(keys) > 0 {
			cs.rdb.Del(cs.ctx, keys...).Err()
		}

		cursor = nextCursor

		if cursor == 0 {
			break
		}
	}

	return nil
}

func (cs *redisCacheService) Exists(key string) (bool, error) {
	if cs.rdb == nil {
		return false, nil
	}

	count, err := cs.rdb.Exists(cs.ctx, key).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (cs *redisCacheService) Del(key string) error {
	if cs.rdb == nil {
		return nil
	}

	return cs.rdb.Del(cs.ctx, key).Err()
}

func (cs *redisCacheService) Incr(key string) (int64, error) {
	if cs.rdb == nil {
		return 1, nil
	}

	return cs.rdb.Incr(cs.ctx, key).Result()
}

func (cs *redisCacheService) Expire(key string, ttl time.Duration) error {
	if cs.rdb == nil {
		return nil
	}

	return cs.rdb.Expire(cs.ctx, key, ttl).Err()
}

func (cs *redisCacheService) TTL(key string) (time.Duration, error) {
	if cs.rdb == nil {
		return 0, nil
	}

	return cs.rdb.TTL(cs.ctx, key).Result()
}
