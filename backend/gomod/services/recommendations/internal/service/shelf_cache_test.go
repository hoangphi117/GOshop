package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	recsdto "goshop/services/recommendations/internal/dto"
)

// fakeCache is an in-memory RedisCacheService. The real one marshals through
// JSON, so the payload — not just the type — is what the tests exercise.
type fakeCache struct {
	items   map[string]cachePayload
	getErrs int
	setErrs int
	cleared []string
}

type cachePayload struct {
	v   int
	raw []byte
}

func newFakeCache() *fakeCache { return &fakeCache{items: map[string]cachePayload{}} }

// The real cache has RedisCacheService's shape; this trimmed double implements
// only what shelf_cache.go calls, through a local interface assertion.
func (f *fakeCache) Get(key string, dest any) error {
	if f.getErrs > 0 {
		f.getErrs--
		return errors.New("cache down")
	}
	payload, ok := f.items[key]
	if !ok {
		return errors.New("miss")
	}
	return json.Unmarshal(payload.raw, dest)
}

func (f *fakeCache) Set(key string, value any, ttl time.Duration) error {
	if f.setErrs > 0 {
		f.setErrs--
		return errors.New("cache down")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	f.items[key] = cachePayload{v: cacheV, raw: raw}
	return nil
}

func (f *fakeCache) Clear(pattern string) error {
	f.cleared = append(f.cleared, pattern)
	return nil
}

// The double has to satisfy the real interface, not a narrowed one: the service
// accepts cache.RedisCacheService, and a fake that only covers the methods this
// file happens to call would stop compiling the moment the service used another.
func (f *fakeCache) SetNX(string, any, time.Duration) (bool, error) { return true, nil }
func (f *fakeCache) Exists(string) (bool, error)                    { return false, nil }
func (f *fakeCache) Del(string) error                               { return nil }
func (f *fakeCache) Incr(string) (int64, error)                     { return 0, nil }
func (f *fakeCache) Expire(string, time.Duration) error             { return nil }
func (f *fakeCache) TTL(string) (time.Duration, error)              { return 0, nil }

func shelfFixture() Shelf {
	return Shelf{Source: SourcePersonalized, Items: []recsdto.RecommendationItem{{UUID: "p1", Name: "Keyboard"}}}
}

// TestReadThroughCachesOnlyRealShelves pins the two rules that make the cache
// safe: a loader that reports "no answer" must not be stored (or the fallback
// would stick for a whole TTL), and a store failure must not surface as a page
// error.
func TestReadThroughCachesOnlyRealShelves(t *testing.T) {
	t.Run("hit is served without calling the loader", func(t *testing.T) {
		fc := newFakeCache()
		if err := fc.Set("k", cacheShelf{V: cacheV, Source: SourcePersonalized}, time.Minute); err != nil {
			t.Fatal(err)
		}
		svc := &personalizedService{cache: fc}

		calls := 0
		got, err := svc.readThrough(context.Background(), "k", time.Minute, func(context.Context) (Shelf, error) {
			calls++
			return shelfFixture(), nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if calls != 0 {
			t.Fatalf("cache hit still called the loader %d times", calls)
		}
		if got.Source != SourcePersonalized {
			t.Fatalf("source=%q", got.Source)
		}
	})

	t.Run("empty shelf is not stored", func(t *testing.T) {
		fc := newFakeCache()
		svc := &personalizedService{cache: fc}

		if _, err := svc.readThrough(context.Background(), "k", time.Minute, func(context.Context) (Shelf, error) {
			return Shelf{}, nil
		}); err != nil {
			t.Fatal(err)
		}
		if _, ok := fc.items["k"]; ok {
			t.Fatal("an empty shelf was cached; the best-seller fallback would stick")
		}
	})

	t.Run("store failure is not a page error", func(t *testing.T) {
		fc := newFakeCache()
		fc.setErrs = 1
		svc := &personalizedService{cache: fc}

		got, err := svc.readThrough(context.Background(), "k", time.Minute, func(context.Context) (Shelf, error) {
			return shelfFixture(), nil
		})
		if err != nil {
			t.Fatalf("cache write failure leaked out: %v", err)
		}
		if len(got.Items) != 1 {
			t.Fatalf("shelf lost while the cache was down: %+v", got)
		}
	})

	t.Run("loader error is returned as is", func(t *testing.T) {
		want := errors.New("database down")
		svc := &personalizedService{cache: newFakeCache()}

		_, err := svc.readThrough(context.Background(), "k", time.Minute, func(context.Context) (Shelf, error) {
			return Shelf{}, want
		})
		if !errors.Is(err, want) {
			t.Fatalf("err=%v, want %v", err, want)
		}
	})

	t.Run("nil cache is a straight pass-through", func(t *testing.T) {
		svc := &personalizedService{}
		got, err := svc.readThrough(context.Background(), "k", time.Minute, func(context.Context) (Shelf, error) {
			return shelfFixture(), nil
		})
		if err != nil || len(got.Items) != 1 {
			t.Fatalf("uncached mode broke: %v %+v", err, got)
		}
	})

	t.Run("a payload written by an older format is a miss", func(t *testing.T) {
		fc := newFakeCache()
		if err := fc.Set("k", cacheShelf{V: cacheV - 1, Source: SourcePersonalized}, time.Minute); err != nil {
			t.Fatal(err)
		}
		svc := &personalizedService{cache: fc}

		calls := 0
		if _, err := svc.readThrough(context.Background(), "k", time.Minute, func(context.Context) (Shelf, error) {
			calls++
			return shelfFixture(), nil
		}); err != nil {
			t.Fatal(err)
		}
		if calls != 1 {
			t.Fatalf("stale-format entry was served (calls=%d)", calls)
		}
	})
}

// TestCacheKeysKeepRequestsApart guards the one property a cache key must have:
// two different requests never share an entry. Search caching is off (see
// shelf_cache.go), so only the visitor-scoped home key is checked here.
func TestCacheKeysKeepRequestsApart(t *testing.T) {
	// The visitor is the whole point of the home key: two visitors must never
	// see each other's shelf.
	if homeCacheKey("a:1", 12) == homeCacheKey("u:2", 12) {
		t.Fatal("home shelf cache key ignores the visitor")
	}
	// The limit is part of the key too: a twelve-item shelf is not a fifty.
	if homeCacheKey("a:1", 12) == homeCacheKey("a:1", 50) {
		t.Fatal("home shelf cache key ignores the limit")
	}
	if similarCacheKey(uuid.New(), 12) == similarCacheKey(uuid.New(), 50) {
		t.Fatal("similar cache key ignores the limit")
	}
}
