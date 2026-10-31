package cache

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryCache_FallbackWhenRedisNil(t *testing.T) {
	// When rdb is nil (Redis completely unavailable)
	cacheSvc := NewRedisCacheService(nil)

	key := "test:item:1"
	type TestItem struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	item := TestItem{ID: 101, Name: "Sneakers"}

	// 1. Set should write to L1 in-memory cache without error
	err := cacheSvc.Set(key, item, 2*time.Second)
	require.NoError(t, err)

	// 2. Exists should return true from memory
	exists, err := cacheSvc.Exists(key)
	require.NoError(t, err)
	assert.True(t, exists)

	// 3. Get should retrieve from L1 in-memory cache
	var retrieved TestItem
	err = cacheSvc.Get(key, &retrieved)
	require.NoError(t, err)
	assert.Equal(t, item, retrieved)

	// 4. Del should remove from memory
	err = cacheSvc.Del(key)
	require.NoError(t, err)

	exists, err = cacheSvc.Exists(key)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestMemoryCache_ClearPattern(t *testing.T) {
	cacheSvc := NewRedisCacheService(nil)

	_ = cacheSvc.Set("product:1", "p1", time.Minute)
	_ = cacheSvc.Set("product:2", "p2", time.Minute)
	_ = cacheSvc.Set("category:1", "c1", time.Minute)

	err := cacheSvc.Clear("product:*")
	require.NoError(t, err)

	p1Exists, _ := cacheSvc.Exists("product:1")
	p2Exists, _ := cacheSvc.Exists("product:2")
	c1Exists, _ := cacheSvc.Exists("category:1")

	assert.False(t, p1Exists)
	assert.False(t, p2Exists)
	assert.True(t, c1Exists)
}

func TestMemoryCache_ConcurrentReads(t *testing.T) {
	cacheSvc := NewRedisCacheService(nil)
	key := "product:hot"
	_ = cacheSvc.Set(key, "data", time.Minute)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var dest string
			err := cacheSvc.Get(key, &dest)
			assert.NoError(t, err)
			assert.Equal(t, "data", dest)
		}()
	}
	wg.Wait()
}

func TestMatchPattern(t *testing.T) {
	assert.True(t, matchPattern("*", "anything"))
	assert.True(t, matchPattern("", "anything"))
	assert.True(t, matchPattern("product:*", "product:123"))
	assert.False(t, matchPattern("product:*", "category:123"))
	assert.True(t, matchPattern("exact", "exact"))
	assert.False(t, matchPattern("exact", "other"))
}
