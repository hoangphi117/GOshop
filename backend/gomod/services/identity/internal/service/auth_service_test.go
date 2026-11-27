package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"goshop/contracts/events"
	"goshop/services/identity/internal/db/sqlc"
	dto "goshop/services/identity/internal/dto"
	"goshop/shared/pkg/auth"
	"goshop/shared/pkg/utils"
)

func assertIdentityErrCode(t *testing.T, err error, want utils.ErrorCode) {
	t.Helper()
	var appErr *utils.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, want, appErr.Code)
}

func hashedPass(t *testing.T, plain string) *string {
	t.Helper()
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.MinCost)
	require.NoError(t, err)
	s := string(b)
	return &s
}

func TestLogin_ValidCredentials_ReturnsTokenPair(t *testing.T) {
	uUUID := uuid.New()
	c := newMemCache()
	// Pre-populate a failed attempt to verify Login resets the rate-limit key.
	require.NoError(t, c.Set("auth:ratelimit:login:10.0.0.1", 2, time.Minute))

	repo := &mockIdentityUserRepo{
		getByEmailFn: func(_ context.Context, email string) (sqlc.User, error) {
			assert.Equal(t, "alice@example.com", email)
			return sqlc.User{
				UserID:       1,
				UserUuid:     uUUID,
				UserEmail:    email,
				UserFullname: "Alice",
				UserPassword: hashedPass(t, "Secret123!"),
				UserStatus:   UserStatusActive,
				UserLevel:    UserLevelUser,
			}, nil
		},
	}

	svc := NewAuthService(repo, &mockTokenService{}, c, nil, nil, nil, nil, "http://localhost:3000")
	access, refresh, ttl, err := svc.Login(context.Background(), "10.0.0.1", "ua", "  ALICE@example.com ", "Secret123!")

	require.NoError(t, err)
	assert.Equal(t, "access-token-1", access)
	assert.Equal(t, "refresh-token-1", refresh)
	assert.Greater(t, ttl, 0)

	exists, _ := c.Exists("auth:ratelimit:login:10.0.0.1")
	assert.False(t, exists)
}

func TestLogin_InvalidPasswordAndNonexistentUser_ReturnSameUnauthorized(t *testing.T) {
	c := newMemCache()
	repo := &mockIdentityUserRepo{
		getByEmailFn: func(_ context.Context, email string) (sqlc.User, error) {
			if email == "alice@example.com" {
				return sqlc.User{UserUuid: uuid.New(), UserEmail: email, UserPassword: hashedPass(t, "Secret123!")}, nil
			}
			return sqlc.User{}, pgx.ErrNoRows
		},
	}
	svc := NewAuthService(repo, &mockTokenService{}, c, nil, nil, nil, nil, "")

	_, _, _, errWrongPass := svc.Login(context.Background(), "10.0.0.2", "ua", "alice@example.com", "Wrong!")
	_, _, _, errNoUser := svc.Login(context.Background(), "10.0.0.3", "ua", "nobody@example.com", "Wrong!")

	assertIdentityErrCode(t, errWrongPass, utils.ErrCodeUnauthorized)
	assertIdentityErrCode(t, errNoUser, utils.ErrCodeUnauthorized)
	assert.Equal(t, errWrongPass.Error(), errNoUser.Error())
}

func TestLogin_ExceedRateLimit_ReturnsRateLimited(t *testing.T) {
	c := newMemCache()
	require.NoError(t, c.Set("auth:ratelimit:login:10.0.0.9", maxLoginAttempts, time.Minute))

	repoCalled := false
	repo := &mockIdentityUserRepo{
		getByEmailFn: func(context.Context, string) (sqlc.User, error) {
			repoCalled = true
			return sqlc.User{}, nil
		},
	}

	svc := NewAuthService(repo, &mockTokenService{}, c, nil, nil, nil, nil, "")
	_, _, _, err := svc.Login(context.Background(), "10.0.0.9", "ua", "a@b.com", "pass")

	assertIdentityErrCode(t, err, utils.ErrCodeTooManyRequests)
	assert.False(t, repoCalled)
}

func TestRefreshToken_ValidAndInvalid(t *testing.T) {
	uUUID := uuid.New()
	repo := &mockIdentityUserRepo{
		getByUUIDFn: func(_ context.Context, id uuid.UUID) (sqlc.User, error) {
			assert.Equal(t, uUUID, id)
			return sqlc.User{UserID: 5, UserUuid: uUUID, UserEmail: "a@b.com", UserLevel: UserLevelUser}, nil
		},
	}
	tokens := &mockTokenService{
		rotateRefreshFn: func(old string) (string, string, error) {
			if old == "valid-rt" {
				return uUUID.String(), "rotated-rt", nil
			}
			return "", "", utils.NewError(utils.ErrCodeUnauthorized, "revoked")
		},
	}

	svc := NewAuthService(repo, tokens, newMemCache(), nil, nil, nil, nil, "")

	access, refresh, _, err := svc.RefreshToken(context.Background(), "valid-rt")
	require.NoError(t, err)
	assert.Equal(t, "access-token-1", access)
	assert.Equal(t, "rotated-rt", refresh)

	_, _, _, err = svc.RefreshToken(context.Background(), "revoked-rt")
	assertIdentityErrCode(t, err, utils.ErrCodeUnauthorized)
}

func TestLogout_RevokesRefreshToken(t *testing.T) {
	tokens := &mockTokenService{
		parseTokenFn: func(string) (*jwt.Token, jwt.MapClaims, error) {
			return nil, jwt.MapClaims{
				"jti": "jti-123",
				"exp": float64(time.Now().Add(10 * time.Minute).Unix()),
			}, nil
		},
	}
	c := newMemCache()
	svc := NewAuthService(&mockIdentityUserRepo{}, tokens, c, nil, nil, nil, nil, "")

	require.NoError(t, svc.Logout(context.Background(), "access-jwt", "rt-1"))
	assert.Equal(t, []string{"rt-1"}, tokens.revokedTokens)
	exists, _ := c.Exists("blacklist:jti-123")
	assert.True(t, exists)
}

func TestLogout_InvalidRefreshToken_ReturnsUnauthorized(t *testing.T) {
	tokens := &mockTokenService{
		validateRefreshFn: func(string) (auth.RefreshToken, error) {
			return auth.RefreshToken{}, errors.New("invalid")
		},
	}
	svc := NewAuthService(&mockIdentityUserRepo{}, tokens, newMemCache(), nil, nil, nil, nil, "")
	err := svc.Logout(context.Background(), "", "bad-rt")
	assertIdentityErrCode(t, err, utils.ErrCodeUnauthorized)
}

func TestRegister_SuccessAndDuplicateEmail(t *testing.T) {
	broker := &mockIdentityBroker{}
	repo := &mockIdentityUserRepo{
		createFn: func(_ context.Context, input sqlc.CreateUserParams) (sqlc.User, error) {
			if input.UserEmail == "dup@example.com" {
				return sqlc.User{}, &pgconn.PgError{Code: "23505"}
			}
			return sqlc.User{UserID: 10, UserUuid: uuid.New(), UserEmail: input.UserEmail, UserFullname: input.UserFullname}, nil
		},
	}
	svc := NewAuthService(repo, &mockTokenService{}, newMemCache(), nil, broker, nil, nil, "")

	err := svc.Register(context.Background(), dto.RegisterInput{
		Name:     "Bob",
		Email:    "Bob@Example.com",
		Password: "Password123!",
	})
	require.NoError(t, err)
	assert.Contains(t, broker.published, events.QueueUserCreated)

	errDup := svc.Register(context.Background(), dto.RegisterInput{
		Name:     "Dup",
		Email:    "dup@example.com",
		Password: "Password123!",
	})
	assertIdentityErrCode(t, errDup, utils.ErrCodeConflict)
}

func TestForgotAndResetPassword_Flow(t *testing.T) {
	uUUID := uuid.New()
	c := newMemCache()
	broker := &mockIdentityBroker{}
	tokens := &mockTokenService{}
	var updatedPassword string

	repo := &mockIdentityUserRepo{
		getByEmailFn: func(_ context.Context, email string) (sqlc.User, error) {
			return sqlc.User{UserUuid: uUUID, UserEmail: email, UserFullname: "Alice"}, nil
		},
		updatePasswordFn: func(_ context.Context, input sqlc.UpdatePasswordParams) (sqlc.User, error) {
			require.NotNil(t, input.NewPassword)
			updatedPassword = *input.NewPassword
			return sqlc.User{UserUuid: input.UserUuid}, nil
		},
	}

	svc := NewAuthService(repo, tokens, c, nil, broker, nil, nil, "https://shop.test")

	// RequestForgotPassword queues mail and sets rate limit
	require.NoError(t, svc.RequestForgotPassword(context.Background(), "alice@example.com"))
	assert.Contains(t, broker.published, "auth_email_queue")

	// Second request within 5m hits rate limit
	errRate := svc.RequestForgotPassword(context.Background(), "alice@example.com")
	assertIdentityErrCode(t, errRate, utils.ErrCodeTooManyRequests)

	// ResetPassword with expired/missing token
	errMissing := svc.ResetPassword(context.Background(), "bad-token", "NewPass123!")
	assertIdentityErrCode(t, errMissing, utils.ErrCodeNotFound)

	// ResetPassword with valid token updates password and revokes user sessions
	require.NoError(t, c.Set("reset:good-tok", uUUID.String(), time.Minute))
	require.NoError(t, svc.ResetPassword(context.Background(), "good-tok", "NewPass123!"))
	assert.NotEmpty(t, updatedPassword)
	assert.Contains(t, tokens.revokedUsers, uUUID.String())
}
