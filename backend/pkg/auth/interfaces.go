package auth

import (
	"github.com/golang-jwt/jwt/v5"
)

// UserTokenPayload represents the domain model required to issue and rotate tokens.
type UserTokenPayload struct {
	UserID   int32  `json:"user_id,omitempty"`
	UserUUID string `json:"user_uuid"`
	Email    string `json:"email"`
	Role     int32  `json:"role"`
}

// JWK represents a JSON Web Key (RFC 7517).
type JWK struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// JWKS represents a JSON Web Key Set (RFC 7517).
type JWKS struct {
	Keys []JWK `json:"keys"`
}

type AccessTokenService interface {
	GenerateAccessToken(user UserTokenPayload) (string, error)
	GenerateAccessTokenWithFamily(user UserTokenPayload, familyID string) (string, error)
	ParseToken(tokenString string) (*jwt.Token, jwt.MapClaims, error)
	DecryptAccessTokenPayload(tokenString string) (*EncryptedPayload, error)
	GetJWKS() JWKS
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

