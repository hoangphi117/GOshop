package v1service_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/testutils"
	"goshop/internal/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationService_Send(t *testing.T) {
	ctx := context.Background()

	t.Run("Case 1: Happy Path -> Persists Notification", func(t *testing.T) {
		var capturedParams sqlc.CreateNotificationParams
		mockNotifRepo := &testutils.MockNotificationRepository{
			CreateFunc: func(ctx context.Context, params sqlc.CreateNotificationParams) (sqlc.Notification, error) {
				capturedParams = params
				return sqlc.Notification{
					NotificationID:     1,
					NotificationUuid:   uuid.New(),
					NotificationUserID: params.NotificationUserID,
					NotificationType:   params.NotificationType,
					NotificationTitle:  params.NotificationTitle,
					NotificationBody:   params.NotificationBody,
					NotificationData:   params.NotificationData,
					NotificationCreatedAt: time.Now(),
				}, nil
			},
		}

		svc := v1service.NewNotificationService(mockNotifRepo, nil, nil)
		err := svc.Send(ctx, v1dto.SendNotificationInput{
			UserID: 10,
			Type:   "order_created",
			Title:  "Order Placed",
			Body:   "Your order was placed successfully",
			Data:   map[string]any{"order_id": 123},
		})

		require.NoError(t, err)
		assert.Equal(t, int32(10), capturedParams.NotificationUserID)
		assert.Equal(t, "order_created", capturedParams.NotificationType)
		assert.Contains(t, string(capturedParams.NotificationData), "123")
	})

	t.Run("Case 2: Repository Error -> Returns Error", func(t *testing.T) {
		mockNotifRepo := &testutils.MockNotificationRepository{
			CreateFunc: func(ctx context.Context, params sqlc.CreateNotificationParams) (sqlc.Notification, error) {
				return sqlc.Notification{}, errors.New("db insert failure")
			},
		}

		svc := v1service.NewNotificationService(mockNotifRepo, nil, nil)
		err := svc.Send(ctx, v1dto.SendNotificationInput{
			UserID: 10,
			Type:   "order_created",
		})

		require.Error(t, err)
	})
}

func TestNotificationService_CreateNotification(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	notifUUID := uuid.New()

	t.Run("Case 1: Recipient User Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewNotificationService(nil, mockUserRepo, nil)
		_, err := svc.CreateNotification(ctx, v1dto.CreateNotificationInput{
			UserUUID: userUUID,
			Type:     "promo",
			Title:    "Special Deal",
			Body:     "Check out our new products",
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Notification Repo Error -> Returns 500", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 5, UserUuid: userUUID}, nil
			},
		}
		mockNotifRepo := &testutils.MockNotificationRepository{
			CreateFunc: func(ctx context.Context, params sqlc.CreateNotificationParams) (sqlc.Notification, error) {
				return sqlc.Notification{}, errors.New("insert error")
			},
		}

		svc := v1service.NewNotificationService(mockNotifRepo, mockUserRepo, nil)
		_, err := svc.CreateNotification(ctx, v1dto.CreateNotificationInput{
			UserUUID: userUUID,
			Type:     "promo",
			Title:    "Special Deal",
			Body:     "Check out our new products",
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})

	t.Run("Case 3: Happy Path -> Creates and Returns DTO", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 5, UserUuid: userUUID}, nil
			},
		}
		mockNotifRepo := &testutils.MockNotificationRepository{
			CreateFunc: func(ctx context.Context, params sqlc.CreateNotificationParams) (sqlc.Notification, error) {
				return sqlc.Notification{
					NotificationID:        1,
					NotificationUuid:      notifUUID,
					NotificationUserID:    params.NotificationUserID,
					NotificationType:      params.NotificationType,
					NotificationTitle:     params.NotificationTitle,
					NotificationBody:      params.NotificationBody,
					NotificationData:      params.NotificationData,
					NotificationIsRead:    false,
					NotificationCreatedAt: time.Now(),
				}, nil
			},
		}

		svc := v1service.NewNotificationService(mockNotifRepo, mockUserRepo, nil)
		dto, err := svc.CreateNotification(ctx, v1dto.CreateNotificationInput{
			UserUUID: userUUID,
			Type:     "promo",
			Title:    "Special Deal",
			Body:     "Check out our new products",
			Data:     map[string]any{"discount": 20},
		})

		require.NoError(t, err)
		assert.Equal(t, notifUUID, dto.UUID)
		assert.Equal(t, "promo", dto.Type)
		assert.Equal(t, "Special Deal", dto.Title)
		assert.False(t, dto.IsRead)
	})
}

func TestNotificationService_BroadcastNotification(t *testing.T) {
	ctx := context.Background()

	t.Run("Case 1: Repo Error -> Returns 500", func(t *testing.T) {
		mockNotifRepo := &testutils.MockNotificationRepository{
			CreateForAllUsersFunc: func(ctx context.Context, nType, title, body string, data []byte) error {
				return errors.New("broadcast failed")
			},
		}

		svc := v1service.NewNotificationService(mockNotifRepo, nil, nil)
		err := svc.BroadcastNotification(ctx, v1dto.BroadcastNotificationInput{
			Type:  "system_maintenance",
			Title: "Maintenance",
			Body:  "Server maintenance in 1 hour",
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})

	t.Run("Case 2: Happy Path -> Broadcasts Successfully", func(t *testing.T) {
		broadcastCalled := false
		mockNotifRepo := &testutils.MockNotificationRepository{
			CreateForAllUsersFunc: func(ctx context.Context, nType, title, body string, data []byte) error {
				broadcastCalled = true
				return nil
			},
		}

		svc := v1service.NewNotificationService(mockNotifRepo, nil, nil)
		err := svc.BroadcastNotification(ctx, v1dto.BroadcastNotificationInput{
			Type:  "system_maintenance",
			Title: "Maintenance",
			Body:  "Server maintenance in 1 hour",
		})

		require.NoError(t, err)
		assert.True(t, broadcastCalled)
	})
}

func TestNotificationService_GetMyNotifications(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	notifUUID := uuid.New()

	t.Run("Case 1: User Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewNotificationService(nil, mockUserRepo, nil)
		_, _, err := svc.GetMyNotifications(ctx, userUUID, v1dto.GetNotificationsQuery{})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Repo GetByUserID Error -> Returns 500", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockNotifRepo := &testutils.MockNotificationRepository{
			GetByUserIDFunc: func(ctx context.Context, userID, limit, offset int32) ([]sqlc.Notification, error) {
				return nil, errors.New("db error")
			},
		}

		svc := v1service.NewNotificationService(mockNotifRepo, mockUserRepo, nil)
		_, _, err := svc.GetMyNotifications(ctx, userUUID, v1dto.GetNotificationsQuery{})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})

	t.Run("Case 3: Repo CountByUserID Error -> Returns 500", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockNotifRepo := &testutils.MockNotificationRepository{
			GetByUserIDFunc: func(ctx context.Context, userID, limit, offset int32) ([]sqlc.Notification, error) {
				return []sqlc.Notification{}, nil
			},
			CountByUserIDFunc: func(ctx context.Context, userID int32) (int64, error) {
				return 0, errors.New("count failed")
			},
		}

		svc := v1service.NewNotificationService(mockNotifRepo, mockUserRepo, nil)
		_, _, err := svc.GetMyNotifications(ctx, userUUID, v1dto.GetNotificationsQuery{})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})

	t.Run("Case 4: Happy Path -> Returns Notifications & Total", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockNotifRepo := &testutils.MockNotificationRepository{
			GetByUserIDFunc: func(ctx context.Context, userID, limit, offset int32) ([]sqlc.Notification, error) {
				return []sqlc.Notification{
					{
						NotificationID:        1,
						NotificationUuid:      notifUUID,
						NotificationUserID:    1,
						NotificationType:      "order",
						NotificationTitle:     "Shipped",
						NotificationBody:      "Your package is on the way",
						NotificationCreatedAt: time.Now(),
					},
				}, nil
			},
			CountByUserIDFunc: func(ctx context.Context, userID int32) (int64, error) {
				return 1, nil
			},
		}

		svc := v1service.NewNotificationService(mockNotifRepo, mockUserRepo, nil)
		dtos, total, err := svc.GetMyNotifications(ctx, userUUID, v1dto.GetNotificationsQuery{Page: 1, Limit: 10})

		require.NoError(t, err)
		assert.Equal(t, int32(1), total)
		assert.Len(t, dtos, 1)
		assert.Equal(t, notifUUID, dtos[0].UUID)
		assert.Equal(t, "Shipped", dtos[0].Title)
	})
}

func TestNotificationService_GetUnreadNotificationCount(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()

	t.Run("Case 1: User Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewNotificationService(nil, mockUserRepo, nil)
		_, err := svc.GetUnreadNotificationCount(ctx, userUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Repo Error -> Returns 500", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockNotifRepo := &testutils.MockNotificationRepository{
			CountUnreadByUserIDFunc: func(ctx context.Context, userID int32) (int64, error) {
				return 0, errors.New("db error")
			},
		}

		svc := v1service.NewNotificationService(mockNotifRepo, mockUserRepo, nil)
		_, err := svc.GetUnreadNotificationCount(ctx, userUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})

	t.Run("Case 3: Happy Path -> Returns Unread Count", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockNotifRepo := &testutils.MockNotificationRepository{
			CountUnreadByUserIDFunc: func(ctx context.Context, userID int32) (int64, error) {
				return 3, nil
			},
		}

		svc := v1service.NewNotificationService(mockNotifRepo, mockUserRepo, nil)
		dto, err := svc.GetUnreadNotificationCount(ctx, userUUID)

		require.NoError(t, err)
		assert.Equal(t, int64(3), dto.UnreadCount)
	})
}

func TestNotificationService_MarkNotificationAsRead(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	notifUUID := uuid.New()

	t.Run("Case 1: User Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewNotificationService(nil, mockUserRepo, nil)
		_, err := svc.MarkNotificationAsRead(ctx, userUUID, notifUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Notification Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockNotifRepo := &testutils.MockNotificationRepository{
			MarkAsReadFunc: func(ctx context.Context, notifUUID uuid.UUID, userID int32) (sqlc.Notification, error) {
				return sqlc.Notification{}, sql.ErrNoRows
			},
		}

		svc := v1service.NewNotificationService(mockNotifRepo, mockUserRepo, nil)
		_, err := svc.MarkNotificationAsRead(ctx, userUUID, notifUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 3: Happy Path -> Marks As Read", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockNotifRepo := &testutils.MockNotificationRepository{
			MarkAsReadFunc: func(ctx context.Context, notifUUID uuid.UUID, userID int32) (sqlc.Notification, error) {
				return sqlc.Notification{
					NotificationID:     1,
					NotificationUuid:   notifUUID,
					NotificationUserID: 1,
					NotificationType:   "order",
					NotificationTitle:  "Delivered",
					NotificationIsRead: true,
					NotificationCreatedAt: time.Now(),
				}, nil
			},
		}

		svc := v1service.NewNotificationService(mockNotifRepo, mockUserRepo, nil)
		dto, err := svc.MarkNotificationAsRead(ctx, userUUID, notifUUID)

		require.NoError(t, err)
		assert.Equal(t, notifUUID, dto.UUID)
		assert.True(t, dto.IsRead)
	})
}

func TestNotificationService_MarkAllNotificationsAsRead(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()

	t.Run("Case 1: User Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewNotificationService(nil, mockUserRepo, nil)
		err := svc.MarkAllNotificationsAsRead(ctx, userUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Happy Path -> Marks All Read", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		markedAll := false
		mockNotifRepo := &testutils.MockNotificationRepository{
			MarkAllAsReadFunc: func(ctx context.Context, userID int32) error {
				markedAll = true
				return nil
			},
		}

		svc := v1service.NewNotificationService(mockNotifRepo, mockUserRepo, nil)
		err := svc.MarkAllNotificationsAsRead(ctx, userUUID)

		require.NoError(t, err)
		assert.True(t, markedAll)
	})
}

func TestNotificationService_DeleteNotification(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	notifUUID := uuid.New()

	t.Run("Case 1: User Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewNotificationService(nil, mockUserRepo, nil)
		err := svc.DeleteNotification(ctx, userUUID, notifUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Notification Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockNotifRepo := &testutils.MockNotificationRepository{
			DeleteFunc: func(ctx context.Context, notifUUID uuid.UUID, userID int32) error {
				return pgx.ErrNoRows
			},
		}

		svc := v1service.NewNotificationService(mockNotifRepo, mockUserRepo, nil)
		err := svc.DeleteNotification(ctx, userUUID, notifUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 3: Happy Path -> Deletes Notification", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		deleted := false
		mockNotifRepo := &testutils.MockNotificationRepository{
			DeleteFunc: func(ctx context.Context, notifUUID uuid.UUID, userID int32) error {
				deleted = true
				return nil
			},
		}

		svc := v1service.NewNotificationService(mockNotifRepo, mockUserRepo, nil)
		err := svc.DeleteNotification(ctx, userUUID, notifUUID)

		require.NoError(t, err)
		assert.True(t, deleted)
	})
}

func TestNotificationService_NotifyAdmins(t *testing.T) {
	ctx := context.Background()

	t.Run("Case 1: Happy Path -> Persists For Admins & Dispatches via WebSocket", func(t *testing.T) {
		persisted := false
		mockNotifRepo := &testutils.MockNotificationRepository{
			CreateForAdminsFunc: func(ctx context.Context, nType, title, body string, data []byte) error {
				persisted = true
				assert.Equal(t, "order", nType)
				assert.Equal(t, "New Order", title)
				return nil
			},
		}
		mockUserRepo := &testutils.MockUserRepository{
			GetActiveAdminUserIDsFunc: func(ctx context.Context) ([]int32, error) {
				return []int32{1, 2}, nil
			},
		}

		svc := v1service.NewNotificationService(mockNotifRepo, mockUserRepo, nil)
		err := svc.NotifyAdmins(ctx, v1dto.AdminNotifyInput{
			Type:  "order",
			Title: "New Order",
			Body:  "Order #123 has been placed",
			Data:  map[string]any{"order_id": 123},
		})

		require.NoError(t, err)
		assert.True(t, persisted)
	})

	t.Run("Case 2: Repo Error -> Returns Internal Error", func(t *testing.T) {
		mockNotifRepo := &testutils.MockNotificationRepository{
			CreateForAdminsFunc: func(ctx context.Context, nType, title, body string, data []byte) error {
				return errors.New("db connection failure")
			},
		}

		svc := v1service.NewNotificationService(mockNotifRepo, nil, nil)
		err := svc.NotifyAdmins(ctx, v1dto.AdminNotifyInput{
			Type:  "system",
			Title: "Alert",
			Body:  "Server load high",
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})
}

func TestNotificationService_GetAdminNotifications(t *testing.T) {
	ctx := context.Background()
	adminUUID := uuid.New()

	t.Run("Case 1: Admin User Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewNotificationService(nil, mockUserRepo, nil)
		_, _, err := svc.GetAdminNotifications(ctx, adminUUID, v1dto.AdminGetNotificationsQuery{})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Happy Path -> Retrieves Filtered Admin Notifications & Total", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 10, UserUuid: adminUUID}, nil
			},
		}
		notifUUID := uuid.New()
		mockNotifRepo := &testutils.MockNotificationRepository{
			GetAdminNotificationsFunc: func(ctx context.Context, userID int32, nType *string, isRead *bool, limit, offset int32) ([]sqlc.Notification, error) {
				assert.Equal(t, int32(10), userID)
				assert.Equal(t, int32(10), limit)
				assert.Equal(t, int32(0), offset)
				return []sqlc.Notification{
					{
						NotificationID:        1,
						NotificationUuid:      notifUUID,
						NotificationUserID:    10,
						NotificationType:      "order",
						NotificationTitle:     "New Order Received",
						NotificationBody:      "Order #555",
						NotificationCreatedAt: time.Now(),
					},
				}, nil
			},
			CountAdminNotificationsFunc: func(ctx context.Context, userID int32, nType *string, isRead *bool) (int64, error) {
				return 1, nil
			},
		}

		svc := v1service.NewNotificationService(mockNotifRepo, mockUserRepo, nil)
		dtos, total, err := svc.GetAdminNotifications(ctx, adminUUID, v1dto.AdminGetNotificationsQuery{
			Page:  1,
			Limit: 10,
		})

		require.NoError(t, err)
		assert.Equal(t, int32(1), total)
		require.Len(t, dtos, 1)
		assert.Equal(t, notifUUID, dtos[0].UUID)
		assert.Equal(t, "New Order Received", dtos[0].Title)
	})
}

