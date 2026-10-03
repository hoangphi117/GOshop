package v1service_test

import (
	"context"
	"testing"
	"time"

	"goshop/internal/db/sqlc"
	v1service "goshop/internal/service/v1"
	"goshop/internal/testutils"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionService_ListUserSessions(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	userID := int32(10)
	currentFamilyID := "fam_current"

	testUser := sqlc.User{
		UserID:   userID,
		UserUuid: userUUID,
	}

	ip1 := "192.168.1.5"
	ua1 := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0"
	dev1 := "Chrome on Windows"

	ip2 := "10.0.0.1"
	ua2 := "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0) Safari/604.1"
	dev2 := "Safari on iOS"

	sessions := []sqlc.UserSession{
		{
			SessionUuid:  uuid.New(),
			UserID:       userID,
			FamilyID:     currentFamilyID,
			IpAddress:    &ip1,
			UserAgent:    &ua1,
			DeviceName:   &dev1,
			LastActiveAt: time.Now(),
			CreatedAt:    time.Now().Add(-time.Hour),
		},
		{
			SessionUuid:  uuid.New(),
			UserID:       userID,
			FamilyID:     "fam_other",
			IpAddress:    &ip2,
			UserAgent:    &ua2,
			DeviceName:   &dev2,
			LastActiveAt: time.Now().Add(-30 * time.Minute),
			CreatedAt:    time.Now().Add(-2 * time.Hour),
		},
	}

	mockUserRepo := &testutils.MockUserRepository{
		GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
			return testUser, nil
		},
	}

	mockSessionRepo := &testutils.MockSessionRepository{
		ListByUserIDFunc: func(ctx context.Context, uid int32) ([]sqlc.UserSession, error) {
			return sessions, nil
		},
	}

	mockTokenSvc := &testutils.MockTokenService{}

	svc := v1service.NewSessionService(mockSessionRepo, mockUserRepo, mockTokenSvc)

	t.Run("Happy Path - Lists sessions correctly", func(t *testing.T) {
		res, err := svc.ListUserSessions(ctx, userUUID, "")
		require.NoError(t, err)
		assert.Len(t, res, 2)
		assert.Equal(t, "Chrome on Windows", res[0].DeviceName)
		assert.Equal(t, "192.168.1.5", res[0].IPAddress)
	})
}

func TestSessionService_RevokeSession(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	userID := int32(10)
	sessionUUID := uuid.New()
	familyID := "fam_to_revoke"

	testUser := sqlc.User{
		UserID:   userID,
		UserUuid: userUUID,
	}

	testSession := sqlc.UserSession{
		SessionUuid: sessionUUID,
		UserID:      userID,
		FamilyID:    familyID,
	}

	mockUserRepo := &testutils.MockUserRepository{
		GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
			return testUser, nil
		},
	}

	t.Run("Happy Path - Revokes session and family", func(t *testing.T) {
		var deletedUUID uuid.UUID
		var deletedUserID int32

		mockSessionRepo := &testutils.MockSessionRepository{
			GetByUUIDFunc: func(ctx context.Context, sUUID uuid.UUID) (sqlc.UserSession, error) {
				return testSession, nil
			},
			DeleteByUUIDFunc: func(ctx context.Context, sUUID uuid.UUID, uID int32) error {
				deletedUUID = sUUID
				deletedUserID = uID
				return nil
			},
		}

		svc := v1service.NewSessionService(mockSessionRepo, mockUserRepo, nil)
		err := svc.RevokeSession(ctx, userUUID, sessionUUID)
		require.NoError(t, err)
		assert.Equal(t, sessionUUID, deletedUUID)
		assert.Equal(t, userID, deletedUserID)
	})

	t.Run("Forbidden when session belongs to another user", func(t *testing.T) {
		otherSession := sqlc.UserSession{
			SessionUuid: sessionUUID,
			UserID:      int32(999), // different user
			FamilyID:    familyID,
		}

		mockSessionRepo := &testutils.MockSessionRepository{
			GetByUUIDFunc: func(ctx context.Context, sUUID uuid.UUID) (sqlc.UserSession, error) {
				return otherSession, nil
			},
		}

		svc := v1service.NewSessionService(mockSessionRepo, mockUserRepo, nil)
		err := svc.RevokeSession(ctx, userUUID, sessionUUID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Unauthorized")
	})
}
