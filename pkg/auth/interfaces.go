package auth

import (
	"goshop/internal/db/sqlc"

	"github.com/golang-jwt/jwt/v5"
)

type TokenService interface {
	GenerateAccessToken(user sqlc.User) (string, error)
	GenerateRefreshToken(userID string) (string, error)
	ParseToken(tokenString string) (*jwt.Token, jwt.MapClaims, error)
	DecryptAccessTokenPayload(tokenString string) (*EncryptedPayload, error)
}
