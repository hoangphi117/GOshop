package cache

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

type memItem struct {
	data      []byte
	expiresAt time.Time
}

type redisCacheService struct {
	ctx               context.Context
	rdb               *redis.Client
	memCache          sync.Map // key string -> memItem
	sf                singleflight.Group
	consecutiveErrors atomic.Int32
	cooldownUntil     atomic.Int64 // Unix timestamp (seconds)
}

func NewRedisCacheService(rdb *redis.Client) RedisCacheService {
	cs := &redisCacheService{
		ctx: context.Background(),
		rdb: rdb,
	}

	// Periodic background cleanup of expired in-memory items (every 1 minute)
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		for range ticker.C {
			now := time.Now()
			cs.memCache.Range(func(k, v any) bool {
				if item, ok := v.(memItem); ok && now.After(item.expiresAt) {
					cs.memCache.Delete(k)
				}
				return true
			})
		}
	}()

	return cs
}

func (cs *redisCacheService) isRedisAvailable() bool {
	if cs.rdb == nil {
		return false
	}
	return time.Now().Unix() >= cs.cooldownUntil.Load()
}

func (cs *redisCacheService) recordRedisError(err error) {
	if err == nil || err == redis.Nil {
		return
	}
	failures := cs.consecutiveErrors.Add(1)
	if failures >= 3 {
		// Enter 15s cooldown
		cs.cooldownUntil.Store(time.Now().Add(15 * time.Second).Unix())
	}
}

func (cs *redisCacheService) recordRedisSuccess() {
	if cs.consecutiveErrors.Load() > 0 {
		cs.consecutiveErrors.Store(0)
	}
	if cs.cooldownUntil.Load() > 0 {
		cs.cooldownUntil.Store(0)
	}
}

func (cs *redisCacheService) setMemory(key string, data []byte, ttl time.Duration) {
	memTTL := ttl
	if memTTL <= 0 || memTTL > 30*time.Second {
		memTTL = 30 * time.Second
	}
	cs.memCache.Store(key, memItem{
		data:      data,
		expiresAt: time.Now().Add(memTTL),
	})
}

func (cs *redisCacheService) getMemory(key string) ([]byte, bool) {
	val, ok := cs.memCache.Load(key)
	if !ok {
		return nil, false
	}
	item, ok := val.(memItem)
	if !ok || time.Now().After(item.expiresAt) {
		cs.memCache.Delete(key)
		return nil, false
	}
	return item.data, true
}

func matchPattern(pattern, key string) bool {
	if pattern == "*" || pattern == "" {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(key, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == key
}

func (cs *redisCacheService) Set(key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	// Always write to L1 in-memory cache
	cs.setMemory(key, data, ttl)

	if !cs.isRedisAvailable() {
		return nil
	}

	ctx, cancel := context.WithTimeout(cs.ctx, 1*time.Second)
	defer cancel()

	err = cs.rdb.Set(ctx, key, data, ttl).Err()
	if err != nil {
		cs.recordRedisError(err)
		return err
	}
	cs.recordRedisSuccess()
	return nil
}

func (cs *redisCacheService) SetNX(key string, value any, ttl time.Duration) (bool, error) {
	if !cs.isRedisAvailable() {
		// When Redis is down, check L1 memory for non-existence
		if _, exists := cs.getMemory(key); exists {
			return false, nil
		}
		data, err := json.Marshal(value)
		if err != nil {
			return false, err
		}
		cs.setMemory(key, data, ttl)
		return true, nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		return false, err
	}

	ctx, cancel := context.WithTimeout(cs.ctx, 1*time.Second)
	defer cancel()

	ok, err := cs.rdb.SetNX(ctx, key, data, ttl).Result()
	if err != nil {
		cs.recordRedisError(err)
		return false, err
	}
	cs.recordRedisSuccess()
	if ok {
		cs.setMemory(key, data, ttl)
	}
	return ok, nil
}

func (cs *redisCacheService) Get(key string, dest any) error {
	// 1. Try L1 In-Memory Cache first (sub-millisecond)
	if data, ok := cs.getMemory(key); ok {
		return json.Unmarshal(data, dest)
	}

	// 2. If Redis is down / in cooldown, fail-fast immediately without network wait
	if !cs.isRedisAvailable() {
		return redis.Nil
	}

	// 3. Use Singleflight to deduplicate concurrent Redis calls for the same key
	res, err, _ := cs.sf.Do(key, func() (any, error) {
		// Re-check memory in case another goroutine just populated it
		if data, ok := cs.getMemory(key); ok {
			return data, nil
		}

		if !cs.isRedisAvailable() {
			return nil, redis.Nil
		}

		ctx, cancel := context.WithTimeout(cs.ctx, 1*time.Second)
		defer cancel()

		result, err := cs.rdb.Get(ctx, key).Bytes()
		if err != nil {
			if err != redis.Nil {
				cs.recordRedisError(err)
			}
			return nil, err
		}

		cs.recordRedisSuccess()
		cs.setMemory(key, result, 30*time.Second)
		return result, nil
	})

	if err != nil {
		return err
	}

	return json.Unmarshal(res.([]byte), dest)
}

func (cs *redisCacheService) Clear(pattern string) error {
	// Invalidate matching keys in L1 Memory
	cs.memCache.Range(func(k, v any) bool {
		if keyStr, ok := k.(string); ok && matchPattern(pattern, keyStr) {
			cs.memCache.Delete(keyStr)
		}
		return true
	})

	if !cs.isRedisAvailable() {
		return nil
	}

	cursor := uint64(0)
	for {
		ctx, cancel := context.WithTimeout(cs.ctx, 2*time.Second)
		keys, nextCursor, err := cs.rdb.Scan(ctx, cursor, pattern, 100).Result()
		cancel()
		if err != nil {
			cs.recordRedisError(err)
			return err
		}

		if len(keys) > 0 {
			delCtx, delCancel := context.WithTimeout(cs.ctx, 2*time.Second)
			if err := cs.rdb.Del(delCtx, keys...).Err(); err != nil {
				cs.recordRedisError(err)
			}
			delCancel()
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	cs.recordRedisSuccess()
	return nil
}

func (cs *redisCacheService) Exists(key string) (bool, error) {
	if _, ok := cs.getMemory(key); ok {
		return true, nil
	}

	if !cs.isRedisAvailable() {
		return false, nil
	}

	ctx, cancel := context.WithTimeout(cs.ctx, 1*time.Second)
	defer cancel()

	count, err := cs.rdb.Exists(ctx, key).Result()
	if err != nil {
		cs.recordRedisError(err)
		return false, err
	}
	cs.recordRedisSuccess()
	return count > 0, nil
}

func (cs *redisCacheService) Del(key string) error {
	cs.memCache.Delete(key)

	if !cs.isRedisAvailable() {
		return nil
	}

	ctx, cancel := context.WithTimeout(cs.ctx, 1*time.Second)
	defer cancel()

	err := cs.rdb.Del(ctx, key).Err()
	if err != nil {
		cs.recordRedisError(err)
		return err
	}
	cs.recordRedisSuccess()
	return nil
}

func (cs *redisCacheService) Incr(key string) (int64, error) {
	if !cs.isRedisAvailable() {
		return 1, nil
	}

	ctx, cancel := context.WithTimeout(cs.ctx, 1*time.Second)
	defer cancel()

	val, err := cs.rdb.Incr(ctx, key).Result()
	if err != nil {
		cs.recordRedisError(err)
		return 1, err
	}
	cs.recordRedisSuccess()
	return val, nil
}

func (cs *redisCacheService) Expire(key string, ttl time.Duration) error {
	if !cs.isRedisAvailable() {
		return nil
	}

	ctx, cancel := context.WithTimeout(cs.ctx, 1*time.Second)
	defer cancel()

	err := cs.rdb.Expire(ctx, key, ttl).Err()
	if err != nil {
		cs.recordRedisError(err)
		return err
	}
	cs.recordRedisSuccess()
	return nil
}

func (cs *redisCacheService) TTL(key string) (time.Duration, error) {
	if !cs.isRedisAvailable() {
		return 0, nil
	}

	ctx, cancel := context.WithTimeout(cs.ctx, 1*time.Second)
	defer cancel()

	dur, err := cs.rdb.TTL(ctx, key).Result()
	if err != nil {
		cs.recordRedisError(err)
		return 0, err
	}
	cs.recordRedisSuccess()
	return dur, nil
}
