package service

import (
	"context"
	"testing"
	"time"
)

func TestRedisLimiterCountsAndBlocks(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()
	caller := "user:test-limiter-" + time.Now().Format("150405.000000")

	l := NewRedisLimiter(rdb, 3)
	key := quotaKey(caller, time.Now())
	t.Cleanup(func() { _ = rdb.Del(context.Background(), key) })

	for i := 1; i <= 3; i++ {
		ok, err := l.Allow(ctx, caller)
		if err != nil {
			t.Fatalf("Allow #%d: %v", i, err)
		}
		if !ok {
			t.Fatalf("Allow #%d = false, want true (limit is 3)", i)
		}
	}

	ok, err := l.Allow(ctx, caller)
	if err != nil {
		t.Fatalf("Allow #4: %v", err)
	}
	if ok {
		t.Fatal("Allow #4 = true, want false")
	}

	// The counter must carry a TTL from the first INCR, otherwise the caller
	// stays blocked forever.
	ttl, err := rdb.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("TTL: %v", err)
	}
	if ttl <= 0 || ttl > 24*time.Hour {
		t.Fatalf("TTL = %v, want (0, 24h]", ttl)
	}
}

// Two different callers must not share a bucket.
func TestRedisLimiterIsolatesCallers(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()
	stamp := time.Now().Format("150405.000000")
	a, b := "sess:limiter-a-"+stamp, "sess:limiter-b-"+stamp

	l := NewRedisLimiter(rdb, 1)
	t.Cleanup(func() {
		_ = rdb.Del(context.Background(), quotaKey(a, time.Now()), quotaKey(b, time.Now()))
	})

	if ok, _ := l.Allow(ctx, a); !ok {
		t.Fatal("first call for a = false")
	}
	if ok, _ := l.Allow(ctx, b); !ok {
		t.Fatal("first call for b = false, buckets leaked between callers")
	}
	if ok, _ := l.Allow(ctx, a); ok {
		t.Fatal("second call for a = true, want false")
	}
}

// limit <= 0 and a nil client must both degrade to "always allow".
func TestRedisLimiterDisabledIsNoop(t *testing.T) {
	for _, l := range []Limiter{NewRedisLimiter(nil, 5), NewRedisLimiter(nil, 0)} {
		for i := 0; i < 100; i++ {
			if ok, err := l.Allow(context.Background(), "user:1"); !ok || err != nil {
				t.Fatalf("disabled limiter = (%v, %v)", ok, err)
			}
		}
	}
}

// A caller key of "" (unauthenticated without a session) must not create keys.
func TestRedisLimiterEmptyCallerAllowed(t *testing.T) {
	l := NewRedisLimiter(nil, 1)
	if ok, err := l.Allow(context.Background(), ""); !ok || err != nil {
		t.Fatalf("empty caller = (%v, %v)", ok, err)
	}
}

func TestQuotaKeyIsPerDay(t *testing.T) {
	d1 := time.Date(2026, 11, 10, 23, 59, 0, 0, time.UTC)
	d2 := time.Date(2026, 11, 11, 0, 1, 0, 0, time.UTC)
	if quotaKey("user:1", d1) == quotaKey("user:1", d2) {
		t.Fatal("quota key did not roll over at midnight UTC")
	}
	// A non-UTC instant must map to the same UTC day as its UTC twin.
	local := d1.In(time.FixedZone("ICT", 7*3600))
	if quotaKey("user:1", local) != quotaKey("user:1", d1) {
		t.Fatalf("quota key depends on the caller's timezone: %s vs %s",
			quotaKey("user:1", local), quotaKey("user:1", d1))
	}
}
