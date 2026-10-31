package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"

	"goshop/pkg/cache"
	"goshop/pkg/utils"

	"github.com/google/uuid"
)

var (
	ErrInvalidRefreshToken = utils.NewError(utils.ErrCodeUnauthorized, "Invalid or expired refresh token")
	ErrTokenFamilyRevoked  = utils.NewError(utils.ErrCodeUnauthorized, "Session has been revoked")
	ErrReplayDetected      = utils.NewError(utils.ErrCodeUnauthorized, "Security alert: Refresh token reuse detected. Session terminated.")
)

type RefreshTokenData struct {
	UserUUID  string    `json:"user_uuid"`
	FamilyID  string    `json:"family_id"`
	Used      bool      `json:"used"`
	CreatedAt time.Time `json:"created_at"`
}

type TokenFamilyData struct {
	UserUUID string `json:"user_uuid"`
	Revoked  bool   `json:"revoked"`
}

const (
	RefreshTokenDuration = 7 * 24 * time.Hour // 7 days
	GracePeriodDuration  = 2 * time.Minute    // 2 minutes to handle network jitter / parallel requests
)

type RefreshTokenRotator interface {
	GenerateRawToken() (string, error)
	CreateSession(userUUID string) (rawToken string, familyID string, err error)
	RotateToken(oldRawToken string) (userUUID string, newRawToken string, err error)
	RevokeToken(rawToken string) error
	RevokeFamily(familyID string) error
	GetFamilyID(rawToken string) (string, error)
}

type redisRefreshTokenRotator struct {
	cache cache.RedisCacheService
}

func NewRefreshTokenRotator(cache cache.RedisCacheService) RefreshTokenRotator {
	return &redisRefreshTokenRotator{
		cache: cache,
	}
}

func HashToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

func (r *redisRefreshTokenRotator) GenerateRawToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", utils.WrapError(utils.ErrCodeInternal, "Failed to generate crypto random bytes", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (r *redisRefreshTokenRotator) CreateSession(userUUID string) (string, string, error) {
	rawToken, err := r.GenerateRawToken()
	if err != nil {
		return "", "", err
	}

	familyID := "fam_" + uuid.NewString()
	tokenHash := HashToken(rawToken)

	tokenData := RefreshTokenData{
		UserUUID:  userUUID,
		FamilyID:  familyID,
		Used:      false,
		CreatedAt: time.Now(),
	}

	familyData := TokenFamilyData{
		UserUUID: userUUID,
		Revoked:  false,
	}

	if r.cache != nil {
		if err := r.cache.Set("rt:"+tokenHash, tokenData, RefreshTokenDuration); err != nil {
			return "", "", utils.WrapError(utils.ErrCodeInternal, "Failed to persist refresh token", err)
		}
		if err := r.cache.Set("rt:family:"+familyID, familyData, RefreshTokenDuration); err != nil {
			return "", "", utils.WrapError(utils.ErrCodeInternal, "Failed to persist token family", err)
		}
	}

	return rawToken, familyID, nil
}

func (r *redisRefreshTokenRotator) RotateToken(oldRawToken string) (string, string, error) {
	if oldRawToken == "" {
		return "", "", ErrInvalidRefreshToken
	}

	if r.cache == nil {
		return "", "", utils.NewError(utils.ErrCodeInternal, "Cache service unavailable")
	}

	oldHash := HashToken(oldRawToken)
	tokenKey := "rt:" + oldHash

	var rtData RefreshTokenData
	err := r.cache.Get(tokenKey, &rtData)
	if err != nil || rtData.UserUUID == "" {
		return "", "", ErrInvalidRefreshToken
	}

	// Verify token family status
	famKey := "rt:family:" + rtData.FamilyID
	var famData TokenFamilyData
	err = r.cache.Get(famKey, &famData)
	if err != nil || famData.UserUUID == "" {
		return "", "", ErrTokenFamilyRevoked
	}

	if famData.Revoked {
		return "", "", ErrTokenFamilyRevoked
	}

	// REPLAY ATTACK DETECTION
	// If an already used token is submitted again, immediately revoke the entire family!
	if rtData.Used {
		famData.Revoked = true
		_ = r.cache.Set(famKey, famData, 24*time.Hour)
		_ = r.cache.Del(tokenKey)
		return "", "", ErrReplayDetected
	}

	// Normal Rotation (Happy Path):
	// 1. Mark old token as used and set short TTL (grace period for race conditions)
	rtData.Used = true
	_ = r.cache.Set(tokenKey, rtData, GracePeriodDuration)

	// 2. Generate new token in the same family
	newRawToken, err := r.GenerateRawToken()
	if err != nil {
		return "", "", err
	}
	newHash := HashToken(newRawToken)

	newRTData := RefreshTokenData{
		UserUUID:  rtData.UserUUID,
		FamilyID:  rtData.FamilyID,
		Used:      false,
		CreatedAt: time.Now(),
	}

	if err := r.cache.Set("rt:"+newHash, newRTData, RefreshTokenDuration); err != nil {
		return "", "", utils.WrapError(utils.ErrCodeInternal, "Failed to store new refresh token", err)
	}

	return rtData.UserUUID, newRawToken, nil
}

func (r *redisRefreshTokenRotator) RevokeToken(rawToken string) error {
	if rawToken == "" || r.cache == nil {
		return nil
	}

	tokenHash := HashToken(rawToken)
	tokenKey := "rt:" + tokenHash

	var rtData RefreshTokenData
	if err := r.cache.Get(tokenKey, &rtData); err == nil && rtData.FamilyID != "" {
		_ = r.cache.Del("rt:family:" + rtData.FamilyID)
		_ = r.cache.Set("blacklist:family:"+rtData.FamilyID, "revoked", 15*time.Minute)
	}
	return r.cache.Del(tokenKey)
}

func (r *redisRefreshTokenRotator) RevokeFamily(familyID string) error {
	if familyID == "" || r.cache == nil {
		return nil
	}
	_ = r.cache.Del("rt:family:" + familyID)
	return r.cache.Set("blacklist:family:"+familyID, "revoked", 15*time.Minute)
}

func (r *redisRefreshTokenRotator) GetFamilyID(rawToken string) (string, error) {
	if rawToken == "" || r.cache == nil {
		return "", nil
	}
	tokenHash := HashToken(rawToken)
	var rtData RefreshTokenData
	if err := r.cache.Get("rt:"+tokenHash, &rtData); err != nil {
		return "", err
	}
	return rtData.FamilyID, nil
}
