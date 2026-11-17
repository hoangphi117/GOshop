package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"goshop/shared/pkg/llm"

	"github.com/redis/go-redis/v9"
)

// redisSessionStore keeps the last N turns of a conversation under a single
// key with a sliding TTL. No table, no migration: a lost session just means the
// customer repeats themselves.
type redisSessionStore struct {
	rdb    *redis.Client
	ttl    time.Duration
	maxMsg int
}

// NewRedisSessionStore builds the store. maxMessages is the cap on replayed
// messages (keep it even: one user + one assistant per turn).
func NewRedisSessionStore(rdb *redis.Client, ttl time.Duration, maxMessages int) SessionStore {
	if ttl <= 0 {
		ttl = time.Hour
	}
	if maxMessages <= 0 {
		maxMessages = 12
	}
	return &redisSessionStore{rdb: rdb, ttl: ttl, maxMsg: maxMessages}
}

func sessionKey(sessionID string) string { return "assistant:sess:" + sessionID }

// Load returns the stored turns. A missing key is not an error.
func (s *redisSessionStore) Load(ctx context.Context, sessionID string) ([]llm.Message, error) {
	if sessionID == "" || s.rdb == nil {
		return nil, nil
	}
	raw, err := s.rdb.Get(ctx, sessionKey(sessionID)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("session load: %w", err)
	}
	var msgs []llm.Message
	if err := json.Unmarshal(raw, &msgs); err != nil {
		// Corrupt payload: drop it rather than failing every future turn.
		_ = s.rdb.Del(ctx, sessionKey(sessionID)).Err()
		return nil, nil
	}
	return msgs, nil
}

// Append adds messages and refreshes the TTL. Read-modify-write is fine here:
// a lost append on a concurrent double-submit costs one repeated sentence,
// where a Lua script costs a new file to maintain.
func (s *redisSessionStore) Append(ctx context.Context, sessionID string, msgs ...llm.Message) error {
	if sessionID == "" || s.rdb == nil || len(msgs) == 0 {
		return nil
	}
	existing, err := s.Load(ctx, sessionID)
	if err != nil {
		return err
	}
	merged := append(existing, msgs...)
	if len(merged) > s.maxMsg {
		merged = merged[len(merged)-s.maxMsg:]
	}
	payload, err := json.Marshal(merged)
	if err != nil {
		return fmt.Errorf("session encode: %w", err)
	}
	return s.rdb.Set(ctx, sessionKey(sessionID), payload, s.ttl).Err()
}
