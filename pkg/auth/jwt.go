package auth

import (
	"encoding/json"
	"goshop/internal/db/sqlc"
	"goshop/internal/utils"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/google/uuid"
)

type JWTService struct{}

type EncryptedPayload struct {
	UserUUID string `json:"user_uuid"`
	Email    string `json:"email"`
	Role     int32  `json:"role"`
}

var (
	jwtSecret     = []byte(utils.GetEnv("JWT_SECRET", "I-am-the-bone-of-my-sword-unknown-to-death-nor-known-to-life"))
	jwtEncryptKey = []byte(utils.GetEnv("JWT_ENCRYPT_KEY", "12345678901234561234567890123456")) // 32 bytes key for AES-256 encryption
)

const (
	AccessTokenTTL = 15 * time.Minute
)

func NewJWTService() TokenService {
	return &JWTService{}
}

func (js *JWTService) GenerateAccessToken(user sqlc.User) (string, error) {
	payload := &EncryptedPayload{
		UserUUID: user.UserUuid.String(),
		Email:    user.UserEmail,
		Role:     user.UserLevel,
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
		"exp":  time.Now().Add(AccessTokenTTL).Unix(),
		"iat":  time.Now().Unix(),
		"iss":  "goshop",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(jwtSecret)
}

func (js *JWTService) GenerateRefreshToken(userID string) (string, error) {
	// Implement JWT refresh token generation logic here
	return "", nil
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
