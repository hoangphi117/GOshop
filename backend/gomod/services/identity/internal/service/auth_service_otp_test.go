package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goshop/contracts/events"
	"goshop/services/identity/internal/db/sqlc"
	dto "goshop/services/identity/internal/dto"
	"goshop/shared/pkg/utils"
)

func TestSendVerifyResendRegisterOTP_FullLifecycle(t *testing.T) {
	c := newMemCache()
	broker := &mockIdentityBroker{}
	uUUID := uuid.New()

	repo := &mockIdentityUserRepo{
		getByEmailFn: func(_ context.Context, email string) (sqlc.User, error) {
			if email == "existing@example.com" {
				return sqlc.User{UserID: 1, UserEmail: email}, nil
			}
			return sqlc.User{}, pgx.ErrNoRows
		},
		createFn: func(_ context.Context, p sqlc.CreateUserParams) (sqlc.User, error) {
			return sqlc.User{
				UserID:       99,
				UserUuid:     uUUID,
				UserFullname: p.UserFullname,
				UserEmail:    p.UserEmail,
				UserLevel:    p.UserLevel,
				UserStatus:   p.UserStatus,
			}, nil
		},
	}

	svc := NewAuthService(repo, &mockTokenService{}, c, nil, broker, nil, nil, "")

	t.Run("rejects existing email with ErrCodeConflict", func(t *testing.T) {
		err := svc.SendRegisterOTP(context.Background(), dto.SendRegisterOTPInput{
			Name: "Dup", Email: "existing@example.com", Password: "Password123!",
		})
		assertIdentityErrCode(t, err, utils.ErrCodeConflict)
	})

	t.Run("sends OTP, enforces cooldown, increments wrong attempts, resends, and verifies", func(t *testing.T) {
		require.NoError(t, svc.SendRegisterOTP(context.Background(), dto.SendRegisterOTPInput{
			Name: "New User", Email: "newuser@example.com", Password: "Password123!",
		}))
		assert.Contains(t, broker.published, "auth_email_queue")

		// Cooldown blocks immediate resend
		errCD := svc.SendRegisterOTP(context.Background(), dto.SendRegisterOTPInput{
			Name: "New User", Email: "newuser@example.com", Password: "Password123!",
		})
		assertIdentityErrCode(t, errCD, utils.ErrCodeTooManyRequests)

		// Wrong OTP increments attempts
		_, _, _, errWrong := svc.VerifyRegisterOTP(context.Background(), dto.VerifyRegisterOTPInput{
			Email: "newuser@example.com", OTP: "000000",
		})
		assertIdentityErrCode(t, errWrong, utils.ErrCodeBadRequest)

		var pending pendingRegistrationData
		require.NoError(t, c.Get("auth:register:newuser@example.com", &pending))
		assert.Equal(t, 1, pending.Attempts)

		// Clear cooldown so ResendRegisterOTP succeeds and resets attempts
		require.NoError(t, c.Del("auth:otp_resend:newuser@example.com"))
		require.NoError(t, svc.ResendRegisterOTP(context.Background(), dto.ResendRegisterOTPInput{
			Email: "newuser@example.com",
		}))

		require.NoError(t, c.Get("auth:register:newuser@example.com", &pending))
		assert.Equal(t, 0, pending.Attempts)
		assert.Len(t, pending.OTP, 6)

		// Valid OTP creates user, cleans Redis, publishes UserCreated, and returns tokens
		access, refresh, _, err := svc.VerifyRegisterOTP(context.Background(), dto.VerifyRegisterOTPInput{
			Email: "newuser@example.com", OTP: pending.OTP,
		})
		require.NoError(t, err)
		assert.Equal(t, "access-token-1", access)
		assert.Equal(t, "refresh-token-1", refresh)
		assert.Contains(t, broker.published, events.QueueUserCreated)

		exists, _ := c.Exists("auth:register:newuser@example.com")
		assert.False(t, exists)
	})

	t.Run("locks out after maxOTPAttempts (5) and deletes session", func(t *testing.T) {
		require.NoError(t, c.Set("auth:register:locked@example.com", pendingRegistrationData{
			Name: "Locked", Email: "locked@example.com", OTP: "123456", Attempts: maxOTPAttempts,
		}, time.Minute))

		_, _, _, err := svc.VerifyRegisterOTP(context.Background(), dto.VerifyRegisterOTPInput{
			Email: "locked@example.com", OTP: "123456",
		})
		assertIdentityErrCode(t, err, utils.ErrCodeBadRequest)

		exists, _ := c.Exists("auth:register:locked@example.com")
		assert.False(t, exists)
	})
}
