package v1service_test

import (
	"context"
	"errors"
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

		svc := v1service.NewAuthService(mockRepo, nil, nil, nil, nil, nil, nil, nil, nil, "http://localhost:3000")

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

		svc := v1service.NewAuthService(mockRepo, nil, nil, nil, nil, nil, nil, nil, nil, "http://localhost:3000")

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

		svc := v1service.NewAuthService(mockRepo, nil, nil, nil, nil, nil, nil, nil, nil, "http://localhost:3000")

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

		svc := v1service.NewAuthService(nil, nil, nil, nil, mockCache, nil, nil, nil, nil, "http://localhost:3000")

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

		svc := v1service.NewAuthService(mockRepo, nil, nil, nil, mockCache, nil, nil, nil, nil, "http://localhost:3000")

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

		svc := v1service.NewAuthService(mockRepo, nil, nil, nil, mockCache, nil, nil, nil, nil, "http://localhost:3000")

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
			GenerateAccessTokenWithFamilyFunc: func(u sqlc.User, familyID string) (string, error) {
				assert.Equal(t, userUUID, u.UserUuid)
				return "generated-access-token", nil
			},
			GenerateRefreshTokenFunc: func(u sqlc.User) (auth.RefreshToken, error) {
				return auth.RefreshToken{Token: "generated-refresh-token", UserUUID: u.UserUuid.String()}, nil
			},
			StoreRefreshTokenFunc: func(token auth.RefreshToken) error {
				assert.Equal(t, "generated-refresh-token", token.Token)
				return nil
			},
			GetTokenFamilyIDFunc: func(token string) (string, error) {
				return "family-123", nil
			},
		}

		svc := v1service.NewAuthService(mockRepo, nil, nil, mockTokenSvc, mockCache, nil, nil, nil, nil, "http://localhost:3000")

		accessToken, refreshToken, expiresIn, err := svc.Login(ctx, "192.168.1.3", "Mozilla/5.0", "  USER@EXAMPLE.COM  ", plainPass)
		require.NoError(t, err)
		assert.Equal(t, "generated-access-token", accessToken)
		assert.Equal(t, "generated-refresh-token", refreshToken)
		assert.Greater(t, expiresIn, 0)
		assert.Equal(t, "auth:ratelimit:login:192.168.1.3", deletedKey)
	})

	t.Run("Case 5: Suspicious Login from New Device - Dispatches Security Notification", func(t *testing.T) {
		mockRepo := &testutils.MockUserRepository{
			GetByEmailFunc: func(ctx context.Context, email string) (sqlc.User, error) {
				return testUser, nil
			},
		}
		oldDevice := "Windows PC"
		oldIP := "10.0.0.1"
		mockSessionRepo := &testutils.MockSessionRepository{
			ListByUserIDFunc: func(ctx context.Context, userID int32) ([]sqlc.UserSession, error) {
				return []sqlc.UserSession{
					{
						SessionID:  1,
						UserID:     userID,
						FamilyID:   "old-family",
						DeviceName: &oldDevice,
						IpAddress:  &oldIP,
					},
				}, nil
			},
			CreateFunc: func(ctx context.Context, params sqlc.CreateSessionParams) (sqlc.UserSession, error) {
				return sqlc.UserSession{SessionID: 2, UserID: params.UserID}, nil
			},
		}
		var createdNotif sqlc.CreateNotificationParams
		mockNotifRepo := &testutils.MockNotificationRepository{
			CreateFunc: func(ctx context.Context, params sqlc.CreateNotificationParams) (sqlc.Notification, error) {
				createdNotif = params
				return sqlc.Notification{
					NotificationID:    1,
					NotificationUuid:  uuid.New(),
					NotificationTitle: params.NotificationTitle,
					NotificationBody:  params.NotificationBody,
				}, nil
			},
		}
		mockTokenSvc := &testutils.MockTokenService{
			GenerateAccessTokenWithFamilyFunc: func(u sqlc.User, familyID string) (string, error) {
				return "generated-access-token", nil
			},
			GenerateRefreshTokenFunc: func(u sqlc.User) (auth.RefreshToken, error) {
				return auth.RefreshToken{Token: "generated-refresh-token", UserUUID: u.UserUuid.String()}, nil
			},
			StoreRefreshTokenFunc: func(token auth.RefreshToken) error {
				return nil
			},
			GetTokenFamilyIDFunc: func(token string) (string, error) {
				return "new-family", nil
			},
		}

		svc := v1service.NewAuthService(mockRepo, mockSessionRepo, mockNotifRepo, mockTokenSvc, nil, nil, nil, nil, nil, "http://localhost:3000")

		// Login from an iPhone (different device & different IP 192.168.1.99)
		_, _, _, err := svc.Login(ctx, "192.168.1.99", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X)", "user@example.com", plainPass)
		require.NoError(t, err)
		assert.Equal(t, int32(1), createdNotif.NotificationUserID)
		assert.Equal(t, "security_alert", createdNotif.NotificationType)
		assert.Contains(t, createdNotif.NotificationTitle, "New login")
		assert.Contains(t, createdNotif.NotificationBody, "192.168.1.99")
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

		svc := v1service.NewAuthService(nil, nil, nil, mockTokenSvc, nil, nil, nil, nil, nil, "http://localhost:3000")

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

		svc := v1service.NewAuthService(nil, nil, nil, mockTokenSvc, mockCache, nil, nil, nil, nil, "http://localhost:3000")

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

		svc := v1service.NewAuthService(nil, nil, nil, mockTokenSvc, nil, nil, nil, nil, nil, "http://localhost:3000")

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

		svc := v1service.NewAuthService(nil, nil, nil, mockTokenSvc, nil, nil, nil, nil, nil, "http://localhost:3000")

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

		svc := v1service.NewAuthService(mockRepo, nil, nil, mockTokenSvc, nil, nil, nil, nil, nil, "http://localhost:3000")

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
			GenerateAccessTokenWithFamilyFunc: func(u sqlc.User, familyID string) (string, error) {
				return "new-access-token", nil
			},
		}
		mockRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return testUser, nil
			},
		}

		svc := v1service.NewAuthService(mockRepo, nil, nil, mockTokenSvc, nil, nil, nil, nil, nil, "http://localhost:3000")

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

		svc := v1service.NewAuthService(nil, nil, nil, nil, mockCache, nil, nil, nil, nil, "http://localhost:3000")

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

		svc := v1service.NewAuthService(mockRepo, nil, nil, nil, mockCache, nil, nil, nil, nil, "http://localhost:3000")

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

		svc := v1service.NewAuthService(mockRepo, nil, nil, nil, mockCache, nil, mockRabbitMQ, nil, nil, "http://localhost:3000")

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

		svc := v1service.NewAuthService(nil, nil, nil, nil, mockCache, nil, nil, nil, nil, "http://localhost:3000")

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

		svc := v1service.NewAuthService(mockRepo, nil, nil, nil, mockCache, nil, nil, nil, nil, "http://localhost:3000")

		err := svc.ResetPassword(ctx, "valid-reset-token", "brandnewsecret123")
		require.NoError(t, err)
		assert.Equal(t, userUUID, capturedParams.UserUuid)
		require.NotNil(t, capturedParams.NewPassword)
		assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(*capturedParams.NewPassword), []byte("brandnewsecret123")))
		assert.Equal(t, "reset:valid-reset-token", deletedTokenKey)
	})
}
