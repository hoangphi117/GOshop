package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"goshop/shared/pkg/llm"

	"github.com/redis/go-redis/v9"
)

// Redis-backed tests follow the repo convention (see ws_publisher_test.go):
// hit localhost:6379 and skip when the dev container is not running.
func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 15})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis not available: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

func TestRedisSessionStoreRoundTrip(t *testing.T) {
	rdb := newTestRedis(t)
	store := NewRedisSessionStore(rdb, time.Hour, 4)
	ctx := context.Background()
	sid := "test-" + strings.Repeat("a", 8)

	_ = rdb.Del(ctx, sessionKey(sid))

	got, err := store.Load(ctx, sid)
	if err != nil {
		t.Fatalf("Load empty: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Load empty = %d messages, want 0", len(got))
	}

	for i, content := range []string{"one", "two", "three"} {
		role := llm.RoleUser
		if i%2 == 1 {
			role = llm.RoleAssistant
		}
		if err := store.Append(ctx, sid, llm.Message{Role: role, Content: content}); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}

	got, err = store.Load(ctx, sid)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 3 || got[0].Content != "one" || got[2].Content != "three" {
		t.Fatalf("Load = %+v", got)
	}

	ttl, err := rdb.TTL(ctx, sessionKey(sid)).Result()
	if err != nil {
		t.Fatalf("TTL: %v", err)
	}
	if ttl <= 0 || ttl > time.Hour {
		t.Fatalf("TTL = %v, want (0, 1h]", ttl)
	}

	_ = rdb.Del(ctx, sessionKey(sid))
}

func TestRedisSessionStoreCapsHistory(t *testing.T) {
	rdb := newTestRedis(t)
	store := NewRedisSessionStore(rdb, time.Hour, 4)
	ctx := context.Background()
	sid := "test-" + strings.Repeat("b", 8)
	_ = rdb.Del(ctx, sessionKey(sid))

	for _, c := range []string{"1", "2", "3", "4", "5", "6"} {
		if err := store.Append(ctx, sid, llm.Message{Role: llm.RoleUser, Content: c}); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	got, _ := store.Load(ctx, sid)
	if len(got) != 4 {
		t.Fatalf("len = %d, want cap 4", len(got))
	}
	if got[0].Content != "3" || got[3].Content != "6" {
		t.Fatalf("kept wrong slice: %+v", got)
	}

	_ = rdb.Del(ctx, sessionKey(sid))
}

// Empty session ids must be a no-op so anonymous callers never touch Redis.
func TestRedisSessionStoreIgnoresEmptyID(t *testing.T) {
	store := NewRedisSessionStore(nil, time.Hour, 4)
	if err := store.Append(context.Background(), "", llm.Message{Content: "x"}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	got, err := store.Load(context.Background(), "")
	if err != nil || len(got) != 0 {
		t.Fatalf("Load = %v, %v", got, err)
	}
}
