package auth_test

import (
	"encoding/json"
	"testing"
	"time"

	"goshop/shared/pkg/auth"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockMemoryCache struct {
	store map[string]string
}

func newMockMemoryCache() *mockMemoryCache {
	return &mockMemoryCache{
		store: make(map[string]string),
	}
}

func (m *mockMemoryCache) Set(key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	m.store[key] = string(data)
	return nil
}

func (m *mockMemoryCache) SetNX(key string, value any, ttl time.Duration) (bool, error) {
	if _, ok := m.store[key]; ok {
		return false, nil
	}
	return true, m.Set(key, value, ttl)
}

func (m *mockMemoryCache) Get(key string, dest any) error {
	val, ok := m.store[key]
	if !ok {
		return redis.Nil
	}
	return json.Unmarshal([]byte(val), dest)
}

func (m *mockMemoryCache) Clear(pattern string) error {
	m.store = make(map[string]string)
	return nil
}

func (m *mockMemoryCache) Exists(key string) (bool, error) {
	_, ok := m.store[key]
	return ok, nil
}

func (m *mockMemoryCache) Del(key string) error {
	delete(m.store, key)
	return nil
}

func (m *mockMemoryCache) Incr(key string) (int64, error) {
	return 1, nil
}

func (m *mockMemoryCache) Expire(key string, ttl time.Duration) error {
	return nil
}

func (m *mockMemoryCache) TTL(key string) (time.Duration, error) {
	return time.Hour, nil
}

func TestRefreshTokenRotator(t *testing.T) {
	cache := newMockMemoryCache()
	rotator := auth.NewRefreshTokenRotator(cache)
	userUUID := uuid.New().String()

	t.Run("CreateSession - generates token and family", func(t *testing.T) {
		rawToken, familyID, err := rotator.CreateSession(userUUID)
		require.NoError(t, err)
		assert.NotEmpty(t, rawToken)
		assert.NotEmpty(t, familyID)

		// Check stored in cache
		tokenHash := auth.HashToken(rawToken)
		var rtData auth.RefreshTokenData
		err = cache.Get("rt:"+tokenHash, &rtData)
		require.NoError(t, err)
		assert.Equal(t, userUUID, rtData.UserUUID)
		assert.Equal(t, familyID, rtData.FamilyID)
		assert.False(t, rtData.Used)
	})

	t.Run("RotateToken - Happy Path", func(t *testing.T) {
		rawToken1, _, err := rotator.CreateSession(userUUID)
		require.NoError(t, err)

		returnedUUID, rawToken2, err := rotator.RotateToken(rawToken1)
		require.NoError(t, err)
		assert.Equal(t, userUUID, returnedUUID)
		assert.NotEmpty(t, rawToken2)
		assert.NotEqual(t, rawToken1, rawToken2)

		// Verify rawToken1 is marked as Used
		var rtData1 auth.RefreshTokenData
		err = cache.Get("rt:"+auth.HashToken(rawToken1), &rtData1)
		require.NoError(t, err)
		assert.True(t, rtData1.Used)

		// Verify rawToken2 is not Used
		var rtData2 auth.RefreshTokenData
		err = cache.Get("rt:"+auth.HashToken(rawToken2), &rtData2)
		require.NoError(t, err)
		assert.False(t, rtData2.Used)
		assert.Equal(t, rtData1.FamilyID, rtData2.FamilyID)
	})

	t.Run("RotateToken - Replay Attack Detection", func(t *testing.T) {
		rawToken1, _, err := rotator.CreateSession(userUUID)
		require.NoError(t, err)

		// First rotation (Valid)
		_, rawToken2, err := rotator.RotateToken(rawToken1)
		require.NoError(t, err)
		assert.NotEmpty(t, rawToken2)

		// Attacker attempts to reuse rawToken1 (Replay!)
		_, _, err = rotator.RotateToken(rawToken1)
		require.Error(t, err)
		assert.Equal(t, auth.ErrReplayDetected, err)

		// Verify entire family is now revoked, so even rawToken2 cannot be rotated
		_, _, err = rotator.RotateToken(rawToken2)
		require.Error(t, err)
		assert.Equal(t, auth.ErrTokenFamilyRevoked, err)
	})

	t.Run("RevokeToken - cleans up session and family", func(t *testing.T) {
		rawToken, familyID, err := rotator.CreateSession(userUUID)
		require.NoError(t, err)

		err = rotator.RevokeToken(rawToken)
		require.NoError(t, err)

		// Attempting to rotate should now fail
		_, _, err = rotator.RotateToken(rawToken)
		require.Error(t, err)
		assert.Equal(t, auth.ErrInvalidRefreshToken, err)

		// Verify family key deleted
		var famData auth.TokenFamilyData
		err = cache.Get("rt:family:"+familyID, &famData)
		assert.Error(t, err)
	})
}
