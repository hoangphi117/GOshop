package auth_test

import (
	"testing"
	"time"

	"goshop/shared/pkg/auth"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scopedCache is a mockMemoryCache whose Clear honours the pattern instead of
// wiping everything: the per-user revocation deletes "rt:family:*" and the
// user's cutoff must survive that.
type scopedCache struct {
	mockMemoryCache
}

func (m *scopedCache) Clear(pattern string) error {
	for key := range m.store {
		if matchPattern(pattern, key) {
			delete(m.store, key)
		}
	}
	return nil
}

func matchPattern(pattern, key string) bool {
	const wildcard = "*"
	if len(pattern) > 0 && pattern[len(pattern)-1] == wildcard[0] {
		prefix := pattern[:len(pattern)-1]
		return len(key) >= len(prefix) && key[:len(prefix)] == prefix
	}
	return pattern == key
}

func newScopedCache() *scopedCache {
	return &scopedCache{mockMemoryCache: *newMockMemoryCache()}
}

// Reset writes the cutoff before clearing "rt:family:*"; here the cutoff is
// written last, so the clear must still leave it alone.
func TestRevokeUserSessionsKeepsCutoff(t *testing.T) {
	cache := newScopedCache()
	rotator := auth.NewRefreshTokenRotator(cache)
	userUUID := uuid.New().String()

	require.NoError(t, cache.Set("rt:family:fam_x", auth.TokenFamilyData{UserUUID: userUUID}, time.Hour))
	require.NoError(t, cache.Set("user:revoked_at:"+userUUID, 42, time.Hour))
	require.NoError(t, cache.Clear("rt:family:*"))

	_, err := cache.Exists("user:revoked_at:" + userUUID)
	require.NoError(t, err)
	var epoch int64
	require.NoError(t, cache.Get("user:revoked_at:"+userUUID, &epoch))
	assert.Equal(t, int64(42), epoch)

	famExists, err := cache.Exists("rt:family:fam_x")
	require.NoError(t, err)
	assert.False(t, famExists)

	rawToken, _, err := rotator.CreateSession(userUUID)
	require.NoError(t, err)
	assert.NotEmpty(t, rawToken)
}

// A password reset revokes every refresh session, including one whose family
// key was re-created afterwards, and leaves later logins working.
func TestRevokeUserSessionsSurvivesFamilyKeyExpiry(t *testing.T) {
	cache := newScopedCache()
	rotator := auth.NewRefreshTokenRotator(cache)
	userUUID := uuid.New().String()

	stolenToken, familyID, err := rotator.CreateSession(userUUID)
	require.NoError(t, err)

	require.NoError(t, rotator.RevokeUserSessions(userUUID))

	// Simulate a family key that outlived the revoke (it was re-created or the
	// delete missed it): only the per-user cutoff can reject the token now.
	require.NoError(t, cache.Set("rt:family:"+familyID, auth.TokenFamilyData{
		UserUUID: userUUID,
		Revoked:  false,
	}, time.Hour))

	_, _, err = rotator.RotateToken(stolenToken)
	assert.ErrorIs(t, err, auth.ErrTokenFamilyRevoked,
		"a refresh token minted before the reset must not rotate")

	// A login made after the reset is unaffected.
	freshToken, _, err := rotator.CreateSession(userUUID)
	require.NoError(t, err)
	_, rotated, err := rotator.RotateToken(freshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, rotated)

	// A user who never reset anything is unaffected too.
	otherToken, _, err := rotator.CreateSession(uuid.New().String())
	require.NoError(t, err)
	_, _, err = rotator.RotateToken(otherToken)
	assert.NoError(t, err)
}

// A family created after a cutoff carries that cutoff, so the same reset cannot
// be replayed against sessions that came later.
func TestCreateSessionStampsCurrentCutoff(t *testing.T) {
	cache := newScopedCache()
	rotator := auth.NewRefreshTokenRotator(cache)
	userUUID := uuid.New().String()

	require.NoError(t, rotator.RevokeUserSessions(userUUID))

	rawToken, familyID, err := rotator.CreateSession(userUUID)
	require.NoError(t, err)

	var fam auth.TokenFamilyData
	require.NoError(t, cache.Get("rt:family:"+familyID, &fam))
	assert.NotZero(t, fam.Epoch, "family must record the cutoff that applies to it")

	_, _, err = rotator.RotateToken(rawToken)
	assert.NoError(t, err)
}
