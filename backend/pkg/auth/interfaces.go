package auth

import (
	"github.com/golang-jwt/jwt/v5"
)

// UserTokenPayload represents the domain model required to issue and rotate tokens.
type UserTokenPayload struct {
	UserUUID string `json:"user_uuid"`
	Email    string `json:"email"`
	Role     int32  `json:"role"`
}

type AccessTokenService interface {
	GenerateAccessToken(user UserTokenPayload) (string, error)
	GenerateAccessTokenWithFamily(user UserTokenPayload, familyID string) (string, error)
	ParseToken(tokenString string) (*jwt.Token, jwt.MapClaims, error)
	DecryptAccessTokenPayload(tokenString string) (*EncryptedPayload, error)
}

type TokenService interface {
	AccessTokenService

	// Refresh Token Rotation (RTR) operations
	GenerateRefreshToken(user UserTokenPayload) (RefreshToken, error)
	StoreRefreshToken(token RefreshToken) error
	ValidateRefreshToken(token string) (RefreshToken, error)
	RevokeRefreshToken(token string) error

	RotateRefreshToken(oldToken string) (userUUID string, newRefreshToken string, err error)
	RevokeTokenFamily(familyID string) error
	GetTokenFamilyID(rawToken string) (string, error)
}

