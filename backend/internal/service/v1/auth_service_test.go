package v1service_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/testutils"
	"goshop/internal/utils"
	"goshop/pkg/auth"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestAuthService_Register(t *testing.T) {
	ctx := context.Background()

	t.Run("Case 1: Duplicate Email 23505 - Returns 409 Conflict", func(t *testing.T) {
		mockRepo := &testutils.MockUserRepository{
			CreateFunc: func(ctx context.Context, input sqlc.CreateUserParams) (sqlc.User, error) {
				return sqlc.User{}, &pgconn.PgError{Code: "23505", Message: "duplicate key"}
			},
		}

		svc := v1service.NewAuthService(mockRepo, nil, nil, nil, nil, nil, nil, nil, "http://localhost:3000")

		err := svc.Register(ctx, v1dto.RegisterInput{
			Email:    "test@example.com",
			Password: "password123",
			Name:     "Test User",
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeConflict)
		assert.Contains(t, err.Error(), "email already exists")
	})

	t.Run("Case 2: Repo Error - Returns 500", func(t *testing.T) {
		mockRepo := &testutils.MockUserRepository{
			CreateFunc: func(ctx context.Context, input sqlc.CreateUserParams) (sqlc.User, error) {
				return sqlc.User{}, errors.New("db failure")
			},
		}

		svc := v1service.NewAuthService(mockRepo, nil, nil, nil, nil, nil, nil, nil, "http://localhost:3000")

		err := svc.Register(ctx, v1dto.RegisterInput{
			Email:    "test@example.com",
			Password: "password123",
			Name:     "Test User",
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})

	t.Run("Case 3: Happy Path - Registers User with Hashed Password and Normalized Email", func(t *testing.T) {
		var capturedUser sqlc.CreateUserParams
		mockRepo := &testutils.MockUserRepository{
			CreateFunc: func(ctx context.Context, input sqlc.CreateUserParams) (sqlc.User, error) {
				capturedUser = input
				return sqlc.User{
					UserID:       1,
					UserUuid:     uuid.New(),
					UserFullname: input.UserFullname,
					UserEmail:    input.UserEmail,
					UserPassword: input.UserPassword,
				}, nil
			},
		}

		svc := v1service.NewAuthService(mockRepo, nil, nil, nil, nil, nil, nil, nil, "http://localhost:3000")

		age := int32(25)
		err := svc.Register(ctx, v1dto.RegisterInput{
			Email:    "  TEST@example.com  ",
			Password: "mysecretpassword",
			Name:     "Test User",
			Age:      &age,
		})
		require.NoError(t, err)
		assert.Equal(t, "test@example.com", capturedUser.UserEmail)
		require.NotNil(t, capturedUser.UserPassword)
		assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(*capturedUser.UserPassword), []byte("mysecretpassword")))
		assert.Equal(t, int32(1), capturedUser.UserStatus)
		assert.Equal(t, int32(3), capturedUser.UserLevel)
	})
}

func TestAuthService_Login(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	plainPass := "securepassword123"
	hashedPass, _ := bcrypt.GenerateFromPassword([]byte(plainPass), bcrypt.DefaultCost)
	hashedPassStr := string(hashedPass)
	testUser := sqlc.User{
		UserID:       1,
		UserUuid:     userUUID,
		UserEmail:    "user@example.com",
		UserPassword: &hashedPassStr,
	}

	t.Run("Case 1: IP Rate Limit Exceeded - Returns 429 Too Many Requests", func(t *testing.T) {
		mockCache := &testutils.MockRedisCacheService{
			GetFunc: func(key string, dest any) error {
				if ptr, ok := dest.(*int); ok {
					*ptr = 5
				}
				return nil
			},
		}

		svc := v1service.NewAuthService(nil, nil, nil, mockCache, nil, nil, nil, nil, "http://localhost:3000")

		_, _, _, err := svc.Login(ctx, "1.2.3.4", "Mozilla/5.0", "user@example.com", plainPass)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeTooManyRequests)
		assert.Contains(t, err.Error(), "Too many login attempts")
	})

	t.Run("Case 2: User Not Found - Records Failed Attempt and Returns 401", func(t *testing.T) {
		var recordedKey string
		var recordedAttempts int
		mockCache := &testutils.MockRedisCacheService{
			GetFunc: func(key string, dest any) error {
				return nil
			},
			SetFunc: func(key string, value any, ttl time.Duration) error {
				recordedKey = key
				if val, ok := value.(int); ok {
					recordedAttempts = val
				}
				return nil
			},
		}
		mockRepo := &testutils.MockUserRepository{
			GetByEmailFunc: func(ctx context.Context, email string) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewAuthService(mockRepo, nil, nil, mockCache, nil, nil, nil, nil, "http://localhost:3000")

		_, _, _, err := svc.Login(ctx, "192.168.1.1", "Mozilla/5.0", "nonexistent@example.com", plainPass)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeUnauthorized)
		assert.Contains(t, err.Error(), "Invalid email or password")
		assert.Equal(t, "auth:ratelimit:login:192.168.1.1", recordedKey)
		assert.Equal(t, 1, recordedAttempts)
	})

	t.Run("Case 3: Password Mismatch - Records Failed Attempt and Returns 401", func(t *testing.T) {
		var recordedKey string
		mockCache := &testutils.MockRedisCacheService{
			GetFunc: func(key string, dest any) error {
				return nil
			},
			SetFunc: func(key string, value any, ttl time.Duration) error {
				recordedKey = key
				return nil
			},
		}
		mockRepo := &testutils.MockUserRepository{
			GetByEmailFunc: func(ctx context.Context, email string) (sqlc.User, error) {
				return testUser, nil
			},
		}

		svc := v1service.NewAuthService(mockRepo, nil, nil, mockCache, nil, nil, nil, nil, "http://localhost:3000")

		_, _, _, err := svc.Login(ctx, "192.168.1.2", "Mozilla/5.0", "user@example.com", "wrongpass")
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeUnauthorized)
		assert.Contains(t, err.Error(), "Invalid email or password")
		assert.Equal(t, "auth:ratelimit:login:192.168.1.2", recordedKey)
	})

	t.Run("Case 4: Happy Path - Resets Rate Limit and Returns Tokens", func(t *testing.T) {
		var deletedKey string
		mockCache := &testutils.MockRedisCacheService{
			GetFunc: func(key string, dest any) error {
				return nil
			},
			DelFunc: func(key string) error {
				deletedKey = key
				return nil
			},
		}
		mockRepo := &testutils.MockUserRepository{
			GetByEmailFunc: func(ctx context.Context, email string) (sqlc.User, error) {
				return testUser, nil
			},
		}
		mockTokenSvc := &testutils.MockTokenService{
			GenerateAccessTokenWithFamilyFunc: func(u auth.UserTokenPayload, familyID string) (string, error) {
				assert.Equal(t, userUUID.String(), u.UserUUID)
				return "generated-access-token", nil
			},
			GenerateRefreshTokenFunc: func(u auth.UserTokenPayload) (auth.RefreshToken, error) {
				return auth.RefreshToken{Token: "generated-refresh-token", UserUUID: u.UserUUID}, nil
			},
			StoreRefreshTokenFunc: func(token auth.RefreshToken) error {
				assert.Equal(t, "generated-refresh-token", token.Token)
				return nil
			},
			GetTokenFamilyIDFunc: func(token string) (string, error) {
				return "family-123", nil
			},
		}

		svc := v1service.NewAuthService(mockRepo, nil, mockTokenSvc, mockCache, nil, nil, nil, nil, "http://localhost:3000")

		accessToken, refreshToken, expiresIn, err := svc.Login(ctx, "192.168.1.3", "Mozilla/5.0", "  USER@EXAMPLE.COM  ", plainPass)
		require.NoError(t, err)
		assert.Equal(t, "generated-access-token", accessToken)
		assert.Equal(t, "generated-refresh-token", refreshToken)
		assert.Greater(t, expiresIn, 0)
		assert.Equal(t, "auth:ratelimit:login:192.168.1.3", deletedKey)
	})
}

func TestAuthService_Logout(t *testing.T) {
	ctx := context.Background()

	t.Run("Case 1: Invalid Refresh Token - Returns 401 Unauthorized", func(t *testing.T) {
		mockTokenSvc := &testutils.MockTokenService{
			ValidateRefreshTokenFunc: func(token string) (auth.RefreshToken, error) {
				return auth.RefreshToken{}, errors.New("invalid signature")
			},
		}

		svc := v1service.NewAuthService(nil, nil, mockTokenSvc, nil, nil, nil, nil, nil, "http://localhost:3000")

		err := svc.Logout(ctx, "", "invalid-refresh-token")
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeUnauthorized)
	})

	t.Run("Case 2: Happy Path - Blacklists Access Token & Revokes Refresh Token", func(t *testing.T) {
		var blacklistedKey string
		mockTokenSvc := &testutils.MockTokenService{
			ParseTokenFunc: func(tokenString string) (*jwt.Token, jwt.MapClaims, error) {
				return &jwt.Token{Valid: true}, jwt.MapClaims{"jti": "jwt-id-123", "exp": float64(time.Now().Add(time.Hour).Unix())}, nil
			},
			ValidateRefreshTokenFunc: func(token string) (auth.RefreshToken, error) {
				return auth.RefreshToken{Token: token}, nil
			},
			RevokeRefreshTokenFunc: func(token string) error {
				assert.Equal(t, "valid-refresh-token", token)
				return nil
			},
		}
		mockCache := &testutils.MockRedisCacheService{
			SetFunc: func(key string, value any, ttl time.Duration) error {
				blacklistedKey = key
				return nil
			},
		}

		svc := v1service.NewAuthService(nil, nil, mockTokenSvc, mockCache, nil, nil, nil, nil, "http://localhost:3000")

		err := svc.Logout(ctx, "valid-access-token", "valid-refresh-token")
		require.NoError(t, err)
		assert.Equal(t, "blacklist:jwt-id-123", blacklistedKey)
	})
}

func TestAuthService_RefreshToken(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	testUser := sqlc.User{
		UserID:   1,
		UserUuid: userUUID,
	}

	t.Run("Case 1: Invalid Refresh Token - Returns 401 Unauthorized", func(t *testing.T) {
		mockTokenSvc := &testutils.MockTokenService{
			RotateRefreshTokenFunc: func(token string) (string, string, error) {
				return "", "", auth.ErrInvalidRefreshToken
			},
		}

		svc := v1service.NewAuthService(nil, nil, mockTokenSvc, nil, nil, nil, nil, nil, "http://localhost:3000")

		_, _, _, err := svc.RefreshToken(ctx, "expired-token")
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeUnauthorized)
	})

	t.Run("Case 2: Replay Attack Detected - Returns 401 Unauthorized", func(t *testing.T) {
		mockTokenSvc := &testutils.MockTokenService{
			RotateRefreshTokenFunc: func(token string) (string, string, error) {
				return "", "", auth.ErrReplayDetected
			},
		}

		svc := v1service.NewAuthService(nil, nil, mockTokenSvc, nil, nil, nil, nil, nil, "http://localhost:3000")

		_, _, _, err := svc.RefreshToken(ctx, "replayed-token")
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeUnauthorized)
		assert.Contains(t, err.Error(), "reuse detected")
	})

	t.Run("Case 3: User Not Found - Returns 404", func(t *testing.T) {
		mockTokenSvc := &testutils.MockTokenService{
			RotateRefreshTokenFunc: func(token string) (string, string, error) {
				return userUUID.String(), "new-refresh-token", nil
			},
		}
		mockRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewAuthService(mockRepo, nil, mockTokenSvc, nil, nil, nil, nil, nil, "http://localhost:3000")

		_, _, _, err := svc.RefreshToken(ctx, "valid-refresh-token")
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 4: Happy Path - Rotates Refresh Token and Generates New Access Token", func(t *testing.T) {
		var passedOldToken string

		mockTokenSvc := &testutils.MockTokenService{
			RotateRefreshTokenFunc: func(token string) (string, string, error) {
				passedOldToken = token
				return userUUID.String(), "new-rotated-refresh-token", nil
			},
			GenerateAccessTokenWithFamilyFunc: func(u auth.UserTokenPayload, familyID string) (string, error) {
				return "new-access-token", nil
			},
		}
		mockRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return testUser, nil
			},
		}

		svc := v1service.NewAuthService(mockRepo, nil, mockTokenSvc, nil, nil, nil, nil, nil, "http://localhost:3000")

		access, refresh, exp, err := svc.RefreshToken(ctx, "old-refresh-token")
		require.NoError(t, err)
		assert.Equal(t, "new-access-token", access)
		assert.Equal(t, "new-rotated-refresh-token", refresh)
		assert.Greater(t, exp, 0)
		assert.Equal(t, "old-refresh-token", passedOldToken)
	})
}

func TestAuthService_RequestForgotPassword(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	testUser := sqlc.User{
		UserID:       1,
		UserUuid:     userUUID,
		UserFullname: "John Doe",
		UserEmail:    "john@example.com",
	}

	t.Run("Case 1: Rate Limit Exceeded - Returns 429 Too Many Requests", func(t *testing.T) {
		mockCache := &testutils.MockRedisCacheService{
			ExistsFunc: func(key string) (bool, error) {
				return true, nil
			},
		}

		svc := v1service.NewAuthService(nil, nil, nil, mockCache, nil, nil, nil, nil, "http://localhost:3000")

		err := svc.RequestForgotPassword(ctx, "john@example.com")
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeTooManyRequests)
	})

	t.Run("Case 2: Email Not Found - Returns 404", func(t *testing.T) {
		mockCache := &testutils.MockRedisCacheService{
			ExistsFunc: func(key string) (bool, error) {
				return false, nil
			},
		}
		mockRepo := &testutils.MockUserRepository{
			GetByEmailFunc: func(ctx context.Context, email string) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewAuthService(mockRepo, nil, nil, mockCache, nil, nil, nil, nil, "http://localhost:3000")

		err := svc.RequestForgotPassword(ctx, "nonexistent@example.com")
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 3: Happy Path - Sets Reset Token in Cache and Publishes to RabbitMQ", func(t *testing.T) {
		mockCache := &testutils.MockRedisCacheService{
			ExistsFunc: func(key string) (bool, error) {
				return false, nil
			},
			SetFunc: func(key string, value any, ttl time.Duration) error {
				return nil
			},
		}
		mockRepo := &testutils.MockUserRepository{
			GetByEmailFunc: func(ctx context.Context, email string) (sqlc.User, error) {
				return testUser, nil
			},
		}
		var publishedQueue string
		mockRabbitMQ := &testutils.MockRabbitMQService{
			PublishFunc: func(ctx context.Context, queue string, message any) error {
				publishedQueue = queue
				return nil
			},
		}

		svc := v1service.NewAuthService(mockRepo, nil, nil, mockCache, nil, mockRabbitMQ, nil, nil, "http://localhost:3000")

		err := svc.RequestForgotPassword(ctx, "john@example.com")
		require.NoError(t, err)
		assert.Equal(t, "auth_email_queue", publishedQueue)
	})
}

func TestAuthService_ResetPassword(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()

	t.Run("Case 1: Expired or Invalid Token - Returns 404", func(t *testing.T) {
		mockCache := &testutils.MockRedisCacheService{
			GetFunc: func(key string, dest any) error {
				return errors.New("redis nil")
			},
		}

		svc := v1service.NewAuthService(nil, nil, nil, mockCache, nil, nil, nil, nil, "http://localhost:3000")

		err := svc.ResetPassword(ctx, "invalid-token", "newpassword123")
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Happy Path - Updates Password and Deletes Token from Cache", func(t *testing.T) {
		var capturedParams sqlc.UpdatePasswordParams
		var deletedTokenKey string

		mockCache := &testutils.MockRedisCacheService{
			GetFunc: func(key string, dest any) error {
				if d, ok := dest.(*string); ok {
					*d = userUUID.String()
				}
				return nil
			},
			DelFunc: func(key string) error {
				deletedTokenKey = key
				return nil
			},
		}
		mockRepo := &testutils.MockUserRepository{
			UpdatePasswordFunc: func(ctx context.Context, input sqlc.UpdatePasswordParams) (sqlc.User, error) {
				capturedParams = input
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}

		svc := v1service.NewAuthService(mockRepo, nil, nil, mockCache, nil, nil, nil, nil, "http://localhost:3000")

		err := svc.ResetPassword(ctx, "valid-reset-token", "brandnewsecret123")
		require.NoError(t, err)
		assert.Equal(t, userUUID, capturedParams.UserUuid)
		require.NotNil(t, capturedParams.NewPassword)
		assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(*capturedParams.NewPassword), []byte("brandnewsecret123")))
		assert.Equal(t, "reset:valid-reset-token", deletedTokenKey)
	})
}

func TestAuthService_SendRegisterOTP(t *testing.T) {
	ctx := context.Background()

	t.Run("Case 1: Email already exists in database - Returns 409 Conflict", func(t *testing.T) {
		mockRepo := &testutils.MockUserRepository{
			GetByEmailFunc: func(ctx context.Context, email string) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserEmail: email}, nil
			},
		}
		svc := v1service.NewAuthService(mockRepo, nil, nil, nil, nil, nil, nil, nil, "http://localhost:3000")

		err := svc.SendRegisterOTP(ctx, v1dto.SendRegisterOTPInput{
			Name:     "Existing User",
			Email:    "exist@example.com",
			Password: "Password123!",
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeConflict)
	})

	t.Run("Case 2: Resend cooldown active - Returns 429 Too Many Requests", func(t *testing.T) {
		mockRepo := &testutils.MockUserRepository{
			GetByEmailFunc: func(ctx context.Context, email string) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}
		mockCache := &testutils.MockRedisCacheService{
			GetFunc: func(key string, dest any) error {
				if strings.HasPrefix(key, "auth:otp_resend:") {
					if d, ok := dest.(*string); ok {
						*d = "1"
					}
					return nil
				}
				return redis.Nil
			},
		}
		svc := v1service.NewAuthService(mockRepo, nil, nil, mockCache, nil, nil, nil, nil, "http://localhost:3000")

		err := svc.SendRegisterOTP(ctx, v1dto.SendRegisterOTPInput{
			Name:     "New User",
			Email:    "new@example.com",
			Password: "Password123!",
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeTooManyRequests)
	})

	t.Run("Case 3: Happy Path - Saves pending data in Redis and publishes email", func(t *testing.T) {
		mockRepo := &testutils.MockUserRepository{
			GetByEmailFunc: func(ctx context.Context, email string) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}
		savedKeys := make(map[string]any)
		mockCache := &testutils.MockRedisCacheService{
			GetFunc: func(key string, dest any) error {
				return redis.Nil
			},
			SetFunc: func(key string, value any, ttl time.Duration) error {
				savedKeys[key] = value
				return nil
			},
		}
		publishedQueue := ""
		mockRabbitMQ := &testutils.MockRabbitMQService{
			PublishFunc: func(ctx context.Context, queue string, message any) error {
				publishedQueue = queue
				return nil
			},
		}

		svc := v1service.NewAuthService(mockRepo, nil, nil, mockCache, nil, mockRabbitMQ, nil, nil, "http://localhost:3000")

		err := svc.SendRegisterOTP(ctx, v1dto.SendRegisterOTPInput{
			Name:     "Alice Doe",
			Email:    "alice@example.com",
			Password: "Password123!",
		})
		require.NoError(t, err)
		assert.Equal(t, "auth_email_queue", publishedQueue)
		assert.Contains(t, savedKeys, "auth:register:alice@example.com")
		assert.Contains(t, savedKeys, "auth:otp_resend:alice@example.com")
	})
}

func TestAuthService_VerifyRegisterOTP(t *testing.T) {
	ctx := context.Background()

	t.Run("Case 1: Expired or missing session - Returns 400 Bad Request", func(t *testing.T) {
		mockCache := &testutils.MockRedisCacheService{
			GetFunc: func(key string, dest any) error {
				return redis.Nil
			},
		}
		svc := v1service.NewAuthService(nil, nil, nil, mockCache, nil, nil, nil, nil, "http://localhost:3000")

		_, _, _, err := svc.VerifyRegisterOTP(ctx, v1dto.VerifyRegisterOTPInput{
			Email: "expired@example.com",
			OTP:   "123456",
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "Verification code expired")
	})

	t.Run("Case 2: Invalid OTP - Increments attempts and returns 400 Bad Request", func(t *testing.T) {
		mockCache := &testutils.MockRedisCacheService{
			GetFunc: func(key string, dest any) error {
				dataBytes, _ := json.Marshal(map[string]any{
					"name":            "Bob",
					"email":           "bob@example.com",
					"hashed_password": "hash",
					"otp":             "654321",
					"attempts":        0,
				})
				_ = json.Unmarshal(dataBytes, dest)
				return nil
			},
			SetFunc: func(key string, value any, ttl time.Duration) error {
				return nil
			},
		}
		svc := v1service.NewAuthService(nil, nil, nil, mockCache, nil, nil, nil, nil, "http://localhost:3000")

		_, _, _, err := svc.VerifyRegisterOTP(ctx, v1dto.VerifyRegisterOTPInput{
			Email: "bob@example.com",
			OTP:   "000000",
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "Invalid verification code")
	})

	t.Run("Case 3: Happy Path - Creates user, generates tokens, and cleans up cache", func(t *testing.T) {
		deletedKeys := make([]string, 0)
		mockCache := &testutils.MockRedisCacheService{
			GetFunc: func(key string, dest any) error {
				dataBytes, _ := json.Marshal(map[string]any{
					"name":            "Charlie",
					"email":           "charlie@example.com",
					"hashed_password": "hashed_secret",
					"otp":             "123456",
					"attempts":        0,
				})
				_ = json.Unmarshal(dataBytes, dest)
				return nil
			},
			DelFunc: func(key string) error {
				deletedKeys = append(deletedKeys, key)
				return nil
			},
		}

		userUUID := uuid.New()
		mockRepo := &testutils.MockUserRepository{
			CreateFunc: func(ctx context.Context, params sqlc.CreateUserParams) (sqlc.User, error) {
				return sqlc.User{
					UserID:       10,
					UserUuid:     userUUID,
					UserFullname: params.UserFullname,
					UserEmail:    params.UserEmail,
					UserStatus:   1,
				}, nil
			},
		}

		mockTokenSvc := &testutils.MockTokenService{
			GenerateRefreshTokenFunc: func(user auth.UserTokenPayload) (auth.RefreshToken, error) {
				return auth.RefreshToken{Token: "test-refresh-token"}, nil
			},
			StoreRefreshTokenFunc: func(refreshToken auth.RefreshToken) error {
				return nil
			},
			GetTokenFamilyIDFunc: func(token string) (string, error) {
				return "family-1", nil
			},
			GenerateAccessTokenWithFamilyFunc: func(user auth.UserTokenPayload, familyID string) (string, error) {
				return "test-access-token", nil
			},
		}

		svc := v1service.NewAuthService(mockRepo, nil, mockTokenSvc, mockCache, nil, nil, nil, nil, "http://localhost:3000")

		accessToken, refreshToken, expiresIn, err := svc.VerifyRegisterOTP(ctx, v1dto.VerifyRegisterOTPInput{
			Email: "charlie@example.com",
			OTP:   "123456",
		})
		require.NoError(t, err)
		assert.Equal(t, "test-access-token", accessToken)
		assert.Equal(t, "test-refresh-token", refreshToken)
		assert.Equal(t, int(auth.AccessTokenTTL.Seconds()), expiresIn)
		assert.Contains(t, deletedKeys, "auth:register:charlie@example.com")
		assert.Contains(t, deletedKeys, "auth:otp_resend:charlie@example.com")
	})
}

func TestAuthService_ResendRegisterOTP(t *testing.T) {
	ctx := context.Background()

	t.Run("Case 1: No pending registration session - Returns 400 Bad Request", func(t *testing.T) {
		mockCache := &testutils.MockRedisCacheService{
			GetFunc: func(key string, dest any) error {
				return redis.Nil
			},
		}
		svc := v1service.NewAuthService(nil, nil, nil, mockCache, nil, nil, nil, nil, "http://localhost:3000")

		err := svc.ResendRegisterOTP(ctx, v1dto.ResendRegisterOTPInput{
			Email: "none@example.com",
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
	})

	t.Run("Case 2: Cooldown active - Returns 429 Too Many Requests", func(t *testing.T) {
		mockCache := &testutils.MockRedisCacheService{
			GetFunc: func(key string, dest any) error {
				if strings.HasPrefix(key, "auth:register:") {
					dataBytes, _ := json.Marshal(map[string]any{
						"name":            "Dan",
						"email":           "dan@example.com",
						"hashed_password": "hash",
						"otp":             "111111",
					})
					_ = json.Unmarshal(dataBytes, dest)
					return nil
				}
				if strings.HasPrefix(key, "auth:otp_resend:") {
					if d, ok := dest.(*string); ok {
						*d = "1"
					}
					return nil
				}
				return redis.Nil
			},
		}
		svc := v1service.NewAuthService(nil, nil, nil, mockCache, nil, nil, nil, nil, "http://localhost:3000")

		err := svc.ResendRegisterOTP(ctx, v1dto.ResendRegisterOTPInput{
			Email: "dan@example.com",
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeTooManyRequests)
	})

	t.Run("Case 3: Happy Path - Updates OTP in Redis and sends email", func(t *testing.T) {
		mockCache := &testutils.MockRedisCacheService{
			GetFunc: func(key string, dest any) error {
				if strings.HasPrefix(key, "auth:register:") {
					dataBytes, _ := json.Marshal(map[string]any{
						"name":            "Dan",
						"email":           "dan@example.com",
						"hashed_password": "hash",
						"otp":             "111111",
					})
					_ = json.Unmarshal(dataBytes, dest)
					return nil
				}
				return redis.Nil
			},
			SetFunc: func(key string, value any, ttl time.Duration) error {
				return nil
			},
		}

		publishedQueue := ""
		mockRabbitMQ := &testutils.MockRabbitMQService{
			PublishFunc: func(ctx context.Context, queue string, message any) error {
				publishedQueue = queue
				return nil
			},
		}

		svc := v1service.NewAuthService(nil, nil, nil, mockCache, nil, mockRabbitMQ, nil, nil, "http://localhost:3000")

		err := svc.ResendRegisterOTP(ctx, v1dto.ResendRegisterOTPInput{
			Email: "dan@example.com",
		})
		require.NoError(t, err)
		assert.Equal(t, "auth_email_queue", publishedQueue)
	})
}

