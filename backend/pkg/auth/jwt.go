package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"goshop/internal/utils"
	"goshop/pkg/cache"
	"goshop/pkg/logger"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JWTService struct {
	cache      cache.RedisCacheService
	rotator    RefreshTokenRotator
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	keyID      string
}

type EncryptedPayload struct {
	UserID   int32  `json:"user_id,omitempty"`
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
	jwtEncryptKey = []byte(utils.GetEnv("JWT_ENCRYPT_KEY", "12345678901234561234567890123456")) // 32 bytes key for AES-256 encryption
)

const (
	AccessTokenTTL  = 15 * time.Minute
	RefreshTokenTTL = 7 * 24 * time.Hour
)

func loadKeyData(val string) []byte {
	trimmed := strings.TrimSpace(val)
	if trimmed == "" {
		return nil
	}
	if content, err := os.ReadFile(trimmed); err == nil {
		return content
	}
	normalized := strings.ReplaceAll(trimmed, `\n`, "\n")
	return []byte(normalized)
}

func loadOrGenerateRSAKeys() (*rsa.PrivateKey, *rsa.PublicKey, string) {
	keyID := utils.GetEnv("JWT_KEY_ID", "goshop-key-1")

	var privKey *rsa.PrivateKey
	var pubKey *rsa.PublicKey

	privEnv := utils.GetEnv("JWT_PRIVATE_KEY", "")
	if privEnv != "" {
		if data := loadKeyData(privEnv); len(data) > 0 {
			if k, err := jwt.ParseRSAPrivateKeyFromPEM(data); err == nil {
				privKey = k
				pubKey = &k.PublicKey
			} else {
				logger.Log.Error().Err(err).Msg("Failed to parse JWT_PRIVATE_KEY")
			}
		}
	}

	pubEnv := utils.GetEnv("JWT_PUBLIC_KEY", "")
	if pubEnv != "" {
		if data := loadKeyData(pubEnv); len(data) > 0 {
			if k, err := jwt.ParseRSAPublicKeyFromPEM(data); err == nil {
				pubKey = k
			} else {
				logger.Log.Error().Err(err).Msg("Failed to parse JWT_PUBLIC_KEY")
			}
		}
	}

	// Try default key file locations if not explicitly configured via env
	if privKey == nil {
		for _, path := range []string{"certs/jwt_private.pem", "/app/certs/jwt_private.pem", "../certs/jwt_private.pem"} {
			if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
				if k, err := jwt.ParseRSAPrivateKeyFromPEM(data); err == nil {
					privKey = k
					if pubKey == nil {
						pubKey = &k.PublicKey
					}
					break
				}
			}
		}
	}
	if pubKey == nil {
		for _, path := range []string{"certs/jwt_public.pem", "/app/certs/jwt_public.pem", "../certs/jwt_public.pem"} {
			if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
				if k, err := jwt.ParseRSAPublicKeyFromPEM(data); err == nil {
					pubKey = k
					break
				}
			}
		}
	}

	// Ephemeral generation if neither private key nor public key is configured
	if privKey == nil && pubKey == nil {
		if utils.GetEnv("APP_ENV", "development") == "production" {
			logger.Log.Fatal().Msg("FATAL: JWT_PRIVATE_KEY or JWT_PUBLIC_KEY must be configured in production environment")
		}
		generated, err := rsa.GenerateKey(rand.Reader, 2048)
		if err == nil {
			privKey = generated
			pubKey = &generated.PublicKey
			logger.Log.Info().Str("kid", keyID).Msg("🔑 Initialized ephemeral RSA-2048 keypair for RS256 JWT")
		} else {
			logger.Log.Error().Err(err).Msg("Failed to generate ephemeral RSA keypair")
		}
	}

	return privKey, pubKey, keyID
}

func NewJWTService(cache cache.RedisCacheService) TokenService {
	privKey, pubKey, keyID := loadOrGenerateRSAKeys()
	return &JWTService{
		cache:      cache,
		rotator:    NewRefreshTokenRotator(cache),
		privateKey: privKey,
		publicKey:  pubKey,
		keyID:      keyID,
	}
}

func NewJWTServiceWithKeys(cache cache.RedisCacheService, privKey *rsa.PrivateKey, pubKey *rsa.PublicKey, keyID string) TokenService {
	return &JWTService{
		cache:      cache,
		rotator:    NewRefreshTokenRotator(cache),
		privateKey: privKey,
		publicKey:  pubKey,
		keyID:      keyID,
	}
}

/***************** Access Token (JWT) *****************/

func (js *JWTService) GenerateAccessToken(user UserTokenPayload) (string, error) {
	return js.GenerateAccessTokenWithFamily(user, "")
}

func (js *JWTService) GenerateAccessTokenWithFamily(user UserTokenPayload, familyID string) (string, error) {
	claims := jwt.MapClaims{
		"sub":       user.UserUUID,
		"user_uuid": user.UserUUID,
		"user_id":   user.UserID,
		"email":     user.Email,
		"role":      user.Role,
		"jti":       uuid.NewString(),
		"fid":       familyID,
		"exp":       time.Now().Add(AccessTokenTTL).Unix(),
		"iat":       time.Now().Unix(),
		"iss":       "goshop",
	}

	if len(jwtEncryptKey) == 32 {
		payload := &EncryptedPayload{
			UserID:   user.UserID,
			UserUUID: user.UserUUID,
			Email:    user.Email,
			Role:     user.Role,
			FamilyID: familyID,
		}
		if rawData, err := json.Marshal(payload); err == nil {
			if encryptedData, err := utils.EncryptAES(rawData, jwtEncryptKey); err == nil {
				claims["data"] = encryptedData
			}
		}
	}

	// Strictly use RS256 asymmetric signing
	if js.privateKey == nil {
		return "", utils.NewError(utils.ErrCodeInternal, "Private key not configured for RS256 signing")
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = js.keyID
	return token.SignedString(js.privateKey)
}

func (js *JWTService) ParseToken(tokenString string) (*jwt.Token, jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		// Strictly enforce RS256 verification
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, utils.NewError(utils.ErrCodeUnauthorized, fmt.Sprintf("Unexpected signing method: %v, only RS256 is supported", t.Header["alg"]))
		}
		if js.publicKey == nil {
			return nil, utils.NewError(utils.ErrCodeUnauthorized, "Public key not configured for RS256 verification")
		}
		return js.publicKey, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}))

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

	// 1. Direct claims extraction (offline verification across microservices)
	userUUID, _ := claims["user_uuid"].(string)
	if userUUID == "" {
		if sub, ok := claims["sub"].(string); ok {
			userUUID = sub
		}
	}

	if userUUID != "" {
		payload := &EncryptedPayload{
			UserUUID: userUUID,
		}
		if email, ok := claims["email"].(string); ok {
			payload.Email = email
		}
		if fid, ok := claims["fid"].(string); ok {
			payload.FamilyID = fid
		}
		if roleVal, ok := claims["role"]; ok {
			switch v := roleVal.(type) {
			case float64:
				payload.Role = int32(v)
			case int32:
				payload.Role = v
			case int:
				payload.Role = int32(v)
			}
		}
		if idVal, ok := claims["user_id"]; ok {
			switch v := idVal.(type) {
			case float64:
				payload.UserID = int32(v)
			case int32:
				payload.UserID = v
			case int:
				payload.UserID = int32(v)
			}
		}
		return payload, nil
	}

	// 2. Legacy fallback to AES decrypted data
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

// GetJWKS publishes the public key set in standard RFC 7517 format for offline microservice verification.
func (js *JWTService) GetJWKS() JWKS {
	if js.publicKey == nil {
		return JWKS{Keys: []JWK{}}
	}

	n := base64.RawURLEncoding.EncodeToString(js.publicKey.N.Bytes())
	eBytes := big.NewInt(int64(js.publicKey.E)).Bytes()
	e := base64.RawURLEncoding.EncodeToString(eBytes)

	return JWKS{
		Keys: []JWK{
			{
				Kty: "RSA",
				Use: "sig",
				Alg: "RS256",
				Kid: js.keyID,
				N:   n,
				E:   e,
			},
		},
	}
}

/***************** Refresh Token & Rotation *****************/

func (js *JWTService) GenerateRefreshToken(user UserTokenPayload) (RefreshToken, error) {
	if js.rotator != nil {
		rawToken, _, err := js.rotator.CreateSession(user.UserUUID)
		if err != nil {
			return RefreshToken{}, err
		}
		return RefreshToken{
			Token:     rawToken,
			UserUUID:  user.UserUUID,
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
		UserUUID:  user.UserUUID,
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
