package auth

import (
	"goshop/internal/db/sqlc"

	"github.com/golang-jwt/jwt/v5"
)

type AccessTokenService interface {
	GenerateAccessToken(user sqlc.User) (string, error)
	GenerateAccessTokenWithFamily(user sqlc.User, familyID string) (string, error)
	ParseToken(tokenString string) (*jwt.Token, jwt.MapClaims, error)
	DecryptAccessTokenPayload(tokenString string) (*EncryptedPayload, error)
}

type TokenService interface {
	AccessTokenService

	// Refresh Token Rotation (RTR) operations
	GenerateRefreshToken(user sqlc.User) (RefreshToken, error)
	StoreRefreshToken(token RefreshToken) error
	ValidateRefreshToken(token string) (RefreshToken, error)
	RevokeRefreshToken(token string) error

	RotateRefreshToken(oldToken string) (userUUID string, newRefreshToken string, err error)
	RevokeTokenFamily(familyID string) error
	GetTokenFamilyID(rawToken string) (string, error)
}

