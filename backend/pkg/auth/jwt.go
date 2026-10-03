package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"goshop/internal/db/sqlc"
	"goshop/internal/utils"
	"goshop/pkg/cache"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/google/uuid"
)

type JWTService struct {
	cache   cache.RedisCacheService
	rotator RefreshTokenRotator
}

type EncryptedPayload struct {
	UserUUID string `json:"user_uuid"`
	Email    string `json:"email"`
	Role     int32  `json:"role"`
	FamilyID string `json:"family_id,omitempty"`
}

type RefreshToken struct {
	Token     string    `json:"token"`
	UserUUID  string    `json:"user_uuid"`
	ExpiresAt time.Time `json:"expires_at"`
	Revoked   bool      `json:"revoked"`
}

var (
	jwtSecret     = []byte(utils.GetEnv("JWT_SECRET", "I-am-the-bone-of-my-sword-unknown-to-death-nor-known-to-life"))
	jwtEncryptKey = []byte(utils.GetEnv("JWT_ENCRYPT_KEY", "12345678901234561234567890123456")) // 32 bytes key for AES-256 encryption
)

const (
	AccessTokenTTL  = 15 * time.Minute
	RefreshTokenTTL = 7 * 24 * time.Hour
)

func NewJWTService(cache cache.RedisCacheService) TokenService {
	return &JWTService{
		cache:   cache,
		rotator: NewRefreshTokenRotator(cache),
	}
}

/***************** Access Token (JWT) *****************/

func (js *JWTService) GenerateAccessToken(user sqlc.User) (string, error) {
	return js.GenerateAccessTokenWithFamily(user, "")
}

func (js *JWTService) GenerateAccessTokenWithFamily(user sqlc.User, familyID string) (string, error) {
	payload := &EncryptedPayload{
		UserUUID: user.UserUuid.String(),
		Email:    user.UserEmail,
		Role:     user.UserLevel,
		FamilyID: familyID,
	}

	rawData, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	encryptedData, err := utils.EncryptAES(rawData, jwtEncryptKey)
	if err != nil {
		return "", err
	}

	claims := jwt.MapClaims{
		"data": encryptedData,
		"jti":  uuid.NewString(),
		"fid":  familyID,
		"exp":  time.Now().Add(AccessTokenTTL).Unix(),
		"iat":  time.Now().Unix(),
		"iss":  "goshop",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(jwtSecret)
}

func (js *JWTService) ParseToken(tokenString string) (*jwt.Token, jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		return jwtSecret, nil
	})

	if err != nil || !token.Valid {
		return nil, nil, utils.NewError(utils.ErrCodeUnauthorized, "Invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, nil, utils.NewError(utils.ErrCodeUnauthorized, "Invalid token claims")
	}

	return token, claims, nil
}

func (js *JWTService) DecryptAccessTokenPayload(tokenString string) (*EncryptedPayload, error) {
	_, claims, err := js.ParseToken(tokenString)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Cannot parse token", err)
	}

	encryptedData, ok := claims["data"].(string)
	if !ok {
		return nil, utils.NewError(utils.ErrCodeUnauthorized, "Encoded data not found")
	}

	rawData, err := utils.DecryptAES(encryptedData, jwtEncryptKey)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Cannot decrypt token payload", err)
	}

	var payload EncryptedPayload
	if err := json.Unmarshal(rawData, &payload); err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Invalid data format", err)
	}

	return &payload, nil
}

/***************** Refresh Token & Rotation *****************/

func (js *JWTService) GenerateRefreshToken(user sqlc.User) (RefreshToken, error) {
	if js.rotator != nil {
		rawToken, _, err := js.rotator.CreateSession(user.UserUuid.String())
		if err != nil {
			return RefreshToken{}, err
		}
		return RefreshToken{
			Token:     rawToken,
			UserUUID:  user.UserUuid.String(),
			ExpiresAt: time.Now().Add(RefreshTokenTTL),
			Revoked:   false,
		}, nil
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return RefreshToken{}, utils.WrapError(utils.ErrCodeInternal, "Failed to generate refresh token", err)
	}

	token := base64.StdEncoding.EncodeToString(tokenBytes)

	return RefreshToken{
		Token:     token,
		UserUUID:  user.UserUuid.String(),
		ExpiresAt: time.Now().Add(RefreshTokenTTL),
		Revoked:   false,
	}, nil
}

func (js *JWTService) StoreRefreshToken(token RefreshToken) error {
	// If rotator is used, tokens are persisted in CreateSession / RotateToken
	return nil
}

func (js *JWTService) ValidateRefreshToken(token string) (RefreshToken, error) {
	// Kept for backward compatibility
	if js.rotator != nil {
		oldHash := HashToken(token)
		var rtData RefreshTokenData
		if err := js.cache.Get("rt:"+oldHash, &rtData); err == nil && rtData.UserUUID != "" && !rtData.Used {
			return RefreshToken{
				Token:     token,
				UserUUID:  rtData.UserUUID,
				ExpiresAt: time.Now().Add(RefreshTokenTTL),
				Revoked:   false,
			}, nil
		}
	}
	return RefreshToken{}, utils.NewError(utils.ErrCodeUnauthorized, "Refresh token is invalid or revoked")
}

func (js *JWTService) RevokeRefreshToken(token string) error {
	if js.rotator != nil {
		return js.rotator.RevokeToken(token)
	}
	return nil
}

func (js *JWTService) RotateRefreshToken(oldToken string) (string, string, error) {
	if js.rotator != nil {
		return js.rotator.RotateToken(oldToken)
	}
	return "", "", utils.NewError(utils.ErrCodeInternal, "Token rotator not initialized")
}

func (js *JWTService) RevokeTokenFamily(familyID string) error {
	if js.rotator != nil {
		return js.rotator.RevokeFamily(familyID)
	}
	return nil
}

func (js *JWTService) GetTokenFamilyID(rawToken string) (string, error) {
	if js.rotator != nil {
		return js.rotator.GetFamilyID(rawToken)
	}
	return "", nil
}
