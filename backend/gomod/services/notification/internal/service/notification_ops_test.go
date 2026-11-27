package service_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	identitycontract "goshop/services/identity/contract"
	"goshop/services/notification/internal/db/sqlc"
	"goshop/services/notification/internal/dto"
	"goshop/services/notification/internal/service"
	"goshop/shared/pkg/utils"
	"goshop/shared/pkg/ws"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockIdentityClient struct {
	mock.Mock
}

func (m *mockIdentityClient) GetUserByID(ctx context.Context, userID int32) (*identitycontract.InternalUserDTO, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*identitycontract.InternalUserDTO), args.Error(1)
}

func (m *mockIdentityClient) GetUserByUUID(ctx context.Context, userUUID uuid.UUID) (*identitycontract.InternalUserDTO, error) {
	args := m.Called(ctx, userUUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*identitycontract.InternalUserDTO), args.Error(1)
}

func (m *mockIdentityClient) GetActiveAdminUserIDs(ctx context.Context) ([]int32, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]int32), args.Error(1)
}

func (m *mockIdentityClient) GetUsersBatch(ctx context.Context, userIDs []int32) (map[int32]identitycontract.InternalUserDTO, error) {
	args := m.Called(ctx, userIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[int32]identitycontract.InternalUserDTO), args.Error(1)
}

func (m *mockIdentityClient) GetAddressByUUID(ctx context.Context, userID int32, addressUUID uuid.UUID) (*identitycontract.AddressDTO, error) {
	args := m.Called(ctx, userID, addressUUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*identitycontract.AddressDTO), args.Error(1)
}

func (m *mockIdentityClient) GetAddressesByUserID(ctx context.Context, userID int32) ([]identitycontract.AddressDTO, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]identitycontract.AddressDTO), args.Error(1)
}

type mockRedisBridge struct {
	mock.Mock
}

func (m *mockRedisBridge) Publish(ctx context.Context, envelope ws.RedisNotificationEnvelope) error {
	args := m.Called(ctx, envelope)
	return args.Error(0)
}

func (m *mockRedisBridge) StartSubscriber(ctx context.Context) {
	m.Called(ctx)
}

func (m *mockRedisBridge) Stop() {
	m.Called()
}

func sampleNotification(id int32, userID int32, nType, title string) sqlc.Notification {
	return sqlc.Notification{
		NotificationID:        id,
		NotificationUuid:      uuid.New(),
		NotificationUserID:    userID,
		NotificationType:      nType,
		NotificationTitle:     title,
		NotificationBody:      "Body of " + title,
		NotificationData:      []byte(`{"key":"value"}`),
		NotificationIsRead:    false,
		NotificationCreatedAt: time.Now().UTC(),
	}
}

func TestNotificationService_CreateNotification(t *testing.T) {
	uUUID := uuid.New()

	t.Run("success with identity client and redis bridge", func(t *testing.T) {
		repo := new(mockNotificationRepo)
		idClient := new(mockIdentityClient)
		bridge := new(mockRedisBridge)
		svc := service.NewNotificationService(repo, idClient, nil, nil, bridge)

		idClient.On("GetUserByUUID", mock.Anything, uUUID).Return(&identitycontract.InternalUserDTO{UserID: 42}, nil)
		repo.On("Create", mock.Anything, mock.MatchedBy(func(p sqlc.CreateNotificationParams) bool {
			return p.NotificationUserID == 42 && p.NotificationType == "order" && p.NotificationTitle == "Hello"
		})).Return(sampleNotification(1, 42, "order", "Hello"), nil)
		bridge.On("Publish", mock.Anything, mock.Anything).Return(nil)

		res, err := svc.CreateNotification(context.Background(), dto.CreateNotificationInput{
			UserUUID: uUUID,
			Type:     "order",
			Title:    "Hello",
			Body:     "World",
			Data:     map[string]any{"key": "value"},
		})
		assert.NoError(t, err)
		assert.Equal(t, "Hello", res.Title)
		idClient.AssertExpectations(t)
		repo.AssertExpectations(t)
		bridge.AssertExpectations(t)
	})

	t.Run("identity user not found", func(t *testing.T) {
		repo := new(mockNotificationRepo)
		idClient := new(mockIdentityClient)
		svc := service.NewNotificationService(repo, idClient, nil, nil, nil)

		idClient.On("GetUserByUUID", mock.Anything, uUUID).Return(nil, errors.New("user not found"))

		_, err := svc.CreateNotification(context.Background(), dto.CreateNotificationInput{
			UserUUID: uUUID,
			Type:     "order",
		})
		assert.Error(t, err)
		var appErr *utils.AppError
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, utils.ErrCodeNotFound, appErr.Code)
	})

	t.Run("marshal error payload", func(t *testing.T) {
		repo := new(mockNotificationRepo)
		svc := service.NewNotificationService(repo, nil, nil, nil, nil)

		_, err := svc.CreateNotification(context.Background(), dto.CreateNotificationInput{
			Data: map[string]any{"bad": make(chan int)},
		})
		assert.Error(t, err)
	})

	t.Run("repo create error", func(t *testing.T) {
		repo := new(mockNotificationRepo)
		svc := service.NewNotificationService(repo, nil, nil, nil, nil)

		repo.On("Create", mock.Anything, mock.Anything).Return(sqlc.Notification{}, errors.New("db error"))

		_, err := svc.CreateNotification(context.Background(), dto.CreateNotificationInput{
			Type:  "promo",
			Title: "Discount",
		})
		assert.Error(t, err)
	})
}

func TestNotificationService_BroadcastNotification(t *testing.T) {
	bridge := new(mockRedisBridge)
	svc := service.NewNotificationService(nil, nil, nil, nil, bridge)

	bridge.On("Publish", mock.Anything, mock.MatchedBy(func(env ws.RedisNotificationEnvelope) bool {
		return env.UserID == nil && !env.IsAdmin && env.Message.Type == ws.EventTypeNotification
	})).Return(errors.New("redis pub error")) // tests warning branch

	err := svc.BroadcastNotification(context.Background(), dto.BroadcastNotificationInput{
		Type:  "system_broadcast",
		Title: "Maintenance",
		Body:  "Server down at midnight",
		Data:  nil,
	})
	assert.NoError(t, err)
	bridge.AssertExpectations(t)
}

func TestNotificationService_GetMyNotifications(t *testing.T) {
	uUUID := uuid.New()

	t.Run("success", func(t *testing.T) {
		repo := new(mockNotificationRepo)
		idClient := new(mockIdentityClient)
		svc := service.NewNotificationService(repo, idClient, nil, nil, nil)

		idClient.On("GetUserByUUID", mock.Anything, uUUID).Return(&identitycontract.InternalUserDTO{UserID: 7}, nil)
		items := []sqlc.Notification{
			sampleNotification(1, 7, "order", "Item 1"),
			sampleNotification(2, 7, "order", "Item 2"),
		}
		repo.On("GetByUserID", mock.Anything, int32(7), int32(10), int32(0)).Return(items, nil)
		repo.On("CountByUserID", mock.Anything, int32(7)).Return(int64(2), nil)

		dtos, total, err := svc.GetMyNotifications(context.Background(), uUUID, dto.GetNotificationsQuery{
			Page:  0,
			Limit: 0,
		})
		assert.NoError(t, err)
		assert.Equal(t, int32(2), total)
		assert.Len(t, dtos, 2)
	})

	t.Run("resolve user error", func(t *testing.T) {
		repo := new(mockNotificationRepo)
		idClient := new(mockIdentityClient)
		svc := service.NewNotificationService(repo, idClient, nil, nil, nil)

		idClient.On("GetUserByUUID", mock.Anything, uUUID).Return(nil, errors.New("not found"))

		_, _, err := svc.GetMyNotifications(context.Background(), uUUID, dto.GetNotificationsQuery{})
		assert.Error(t, err)
	})

	t.Run("get by user id repo error", func(t *testing.T) {
		repo := new(mockNotificationRepo)
		idClient := new(mockIdentityClient)
		svc := service.NewNotificationService(repo, idClient, nil, nil, nil)

		idClient.On("GetUserByUUID", mock.Anything, uUUID).Return(&identitycontract.InternalUserDTO{UserID: 7}, nil)
		repo.On("GetByUserID", mock.Anything, int32(7), int32(10), int32(0)).Return([]sqlc.Notification{}, errors.New("db error"))

		_, _, err := svc.GetMyNotifications(context.Background(), uUUID, dto.GetNotificationsQuery{})
		assert.Error(t, err)
	})

	t.Run("count by user id repo error", func(t *testing.T) {
		repo := new(mockNotificationRepo)
		idClient := new(mockIdentityClient)
		svc := service.NewNotificationService(repo, idClient, nil, nil, nil)

		idClient.On("GetUserByUUID", mock.Anything, uUUID).Return(&identitycontract.InternalUserDTO{UserID: 7}, nil)
		repo.On("GetByUserID", mock.Anything, int32(7), int32(10), int32(0)).Return([]sqlc.Notification{}, nil)
		repo.On("CountByUserID", mock.Anything, int32(7)).Return(int64(0), errors.New("db count error"))

		_, _, err := svc.GetMyNotifications(context.Background(), uUUID, dto.GetNotificationsQuery{})
		assert.Error(t, err)
	})
}

func TestNotificationService_GetUnreadNotificationCount(t *testing.T) {
	uUUID := uuid.New()

	t.Run("success", func(t *testing.T) {
		repo := new(mockNotificationRepo)
		idClient := new(mockIdentityClient)
		svc := service.NewNotificationService(repo, idClient, nil, nil, nil)

		idClient.On("GetUserByUUID", mock.Anything, uUUID).Return(&identitycontract.InternalUserDTO{UserID: 15}, nil)
		repo.On("CountUnreadByUserID", mock.Anything, int32(15)).Return(int64(3), nil)

		res, err := svc.GetUnreadNotificationCount(context.Background(), uUUID)
		assert.NoError(t, err)
		assert.Equal(t, int64(3), res.UnreadCount)
	})

	t.Run("count repo error", func(t *testing.T) {
		repo := new(mockNotificationRepo)
		idClient := new(mockIdentityClient)
		svc := service.NewNotificationService(repo, idClient, nil, nil, nil)

		idClient.On("GetUserByUUID", mock.Anything, uUUID).Return(&identitycontract.InternalUserDTO{UserID: 15}, nil)
		repo.On("CountUnreadByUserID", mock.Anything, int32(15)).Return(int64(0), errors.New("fail"))

		_, err := svc.GetUnreadNotificationCount(context.Background(), uUUID)
		assert.Error(t, err)
	})
}

func TestNotificationService_MarkNotificationAsRead(t *testing.T) {
	uUUID := uuid.New()
	nUUID := uuid.New()

	t.Run("success", func(t *testing.T) {
		repo := new(mockNotificationRepo)
		idClient := new(mockIdentityClient)
		svc := service.NewNotificationService(repo, idClient, nil, nil, nil)

		idClient.On("GetUserByUUID", mock.Anything, uUUID).Return(&identitycontract.InternalUserDTO{UserID: 20}, nil)
		notif := sampleNotification(5, 20, "order", "Read me")
		notif.NotificationIsRead = true
		repo.On("MarkAsRead", mock.Anything, nUUID, int32(20)).Return(notif, nil)

		res, err := svc.MarkNotificationAsRead(context.Background(), uUUID, nUUID)
		assert.NoError(t, err)
		assert.True(t, res.IsRead)
	})

	t.Run("not found pgx", func(t *testing.T) {
		repo := new(mockNotificationRepo)
		idClient := new(mockIdentityClient)
		svc := service.NewNotificationService(repo, idClient, nil, nil, nil)

		idClient.On("GetUserByUUID", mock.Anything, uUUID).Return(&identitycontract.InternalUserDTO{UserID: 20}, nil)
		repo.On("MarkAsRead", mock.Anything, nUUID, int32(20)).Return(sqlc.Notification{}, pgx.ErrNoRows)

		_, err := svc.MarkNotificationAsRead(context.Background(), uUUID, nUUID)
		assert.Error(t, err)
		var appErr *utils.AppError
		assert.True(t, errors.As(err, &appErr))
		assert.Equal(t, utils.ErrCodeNotFound, appErr.Code)
	})

	t.Run("not found sql", func(t *testing.T) {
		repo := new(mockNotificationRepo)
		idClient := new(mockIdentityClient)
		svc := service.NewNotificationService(repo, idClient, nil, nil, nil)

		idClient.On("GetUserByUUID", mock.Anything, uUUID).Return(&identitycontract.InternalUserDTO{UserID: 20}, nil)
		repo.On("MarkAsRead", mock.Anything, nUUID, int32(20)).Return(sqlc.Notification{}, sql.ErrNoRows)

		_, err := svc.MarkNotificationAsRead(context.Background(), uUUID, nUUID)
		assert.Error(t, err)
	})

	t.Run("repo error", func(t *testing.T) {
		repo := new(mockNotificationRepo)
		idClient := new(mockIdentityClient)
		svc := service.NewNotificationService(repo, idClient, nil, nil, nil)

		idClient.On("GetUserByUUID", mock.Anything, uUUID).Return(&identitycontract.InternalUserDTO{UserID: 20}, nil)
		repo.On("MarkAsRead", mock.Anything, nUUID, int32(20)).Return(sqlc.Notification{}, errors.New("crash"))

		_, err := svc.MarkNotificationAsRead(context.Background(), uUUID, nUUID)
		assert.Error(t, err)
	})
}

func TestNotificationService_MarkAllNotificationsAsRead(t *testing.T) {
	uUUID := uuid.New()

	t.Run("success", func(t *testing.T) {
		repo := new(mockNotificationRepo)
		idClient := new(mockIdentityClient)
		svc := service.NewNotificationService(repo, idClient, nil, nil, nil)

		idClient.On("GetUserByUUID", mock.Anything, uUUID).Return(&identitycontract.InternalUserDTO{UserID: 12}, nil)
		repo.On("MarkAllAsRead", mock.Anything, int32(12)).Return(nil)

		err := svc.MarkAllNotificationsAsRead(context.Background(), uUUID)
		assert.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		repo := new(mockNotificationRepo)
		idClient := new(mockIdentityClient)
		svc := service.NewNotificationService(repo, idClient, nil, nil, nil)

		idClient.On("GetUserByUUID", mock.Anything, uUUID).Return(&identitycontract.InternalUserDTO{UserID: 12}, nil)
		repo.On("MarkAllAsRead", mock.Anything, int32(12)).Return(errors.New("db error"))

		err := svc.MarkAllNotificationsAsRead(context.Background(), uUUID)
		assert.Error(t, err)
	})
}

func TestNotificationService_DeleteNotification(t *testing.T) {
	uUUID := uuid.New()
	nUUID := uuid.New()

	t.Run("success", func(t *testing.T) {
		repo := new(mockNotificationRepo)
		idClient := new(mockIdentityClient)
		svc := service.NewNotificationService(repo, idClient, nil, nil, nil)

		idClient.On("GetUserByUUID", mock.Anything, uUUID).Return(&identitycontract.InternalUserDTO{UserID: 8}, nil)
		repo.On("Delete", mock.Anything, nUUID, int32(8)).Return(nil)

		err := svc.DeleteNotification(context.Background(), uUUID, nUUID)
		assert.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		repo := new(mockNotificationRepo)
		idClient := new(mockIdentityClient)
		svc := service.NewNotificationService(repo, idClient, nil, nil, nil)

		idClient.On("GetUserByUUID", mock.Anything, uUUID).Return(&identitycontract.InternalUserDTO{UserID: 8}, nil)
		repo.On("Delete", mock.Anything, nUUID, int32(8)).Return(errors.New("delete error"))

		err := svc.DeleteNotification(context.Background(), uUUID, nUUID)
		assert.Error(t, err)
	})
}

func TestNotificationService_NotifyAdmins(t *testing.T) {
	t.Run("success without prefix and with broadcast", func(t *testing.T) {
		repo := new(mockNotificationRepo)
		idClient := new(mockIdentityClient)
		bridge := new(mockRedisBridge)
		svc := service.NewNotificationService(repo, idClient, nil, nil, bridge)

		idClient.On("GetActiveAdminUserIDs", mock.Anything).Return([]int32{1, 2}, nil)
		repo.On("Create", mock.Anything, mock.MatchedBy(func(p sqlc.CreateNotificationParams) bool {
			return p.NotificationType == "admin_order_alert"
		})).Return(sqlc.Notification{}, nil).Twice()
		bridge.On("Publish", mock.Anything, mock.Anything).Return(nil)

		err := svc.NotifyAdmins(context.Background(), dto.AdminNotifyInput{
			Type:  "order_alert",
			Title: "New high value order",
			Body:  "Check order #999",
			Data:  map[string]any{"order_id": 999},
		})
		assert.NoError(t, err)
		repo.AssertExpectations(t)
	})

	t.Run("identity admin IDs fetch error", func(t *testing.T) {
		repo := new(mockNotificationRepo)
		idClient := new(mockIdentityClient)
		svc := service.NewNotificationService(repo, idClient, nil, nil, nil)

		idClient.On("GetActiveAdminUserIDs", mock.Anything).Return(nil, errors.New("grpc err"))

		err := svc.NotifyAdmins(context.Background(), dto.AdminNotifyInput{
			Type:  "admin_system",
			Title: "Test",
		})
		assert.Error(t, err)
	})

	t.Run("repo create error", func(t *testing.T) {
		repo := new(mockNotificationRepo)
		idClient := new(mockIdentityClient)
		svc := service.NewNotificationService(repo, idClient, nil, nil, nil)

		idClient.On("GetActiveAdminUserIDs", mock.Anything).Return([]int32{1}, nil)
		repo.On("Create", mock.Anything, mock.Anything).Return(sqlc.Notification{}, errors.New("db error"))

		err := svc.NotifyAdmins(context.Background(), dto.AdminNotifyInput{
			Type: "admin_stock_alert",
		})
		assert.Error(t, err)
	})
}

func TestNotificationService_AdminNotificationQueries(t *testing.T) {
	adminUUID := uuid.New()
	adminID := int32(100)

	t.Run("GetAdminNotifications success", func(t *testing.T) {
		repo := new(mockNotificationRepo)
		idClient := new(mockIdentityClient)
		svc := service.NewNotificationService(repo, idClient, nil, nil, nil)

		idClient.On("GetUserByUUID", mock.Anything, adminUUID).Return(&identitycontract.InternalUserDTO{UserID: adminID}, nil)
		nType := "admin_order"
		isRead := false
		items := []sqlc.Notification{
			sampleNotification(1, adminID, "admin_order", "Order placed"),
		}
		repo.On("GetAdminNotifications", mock.Anything, adminID, &nType, &isRead, int32(10), int32(0)).Return(items, nil)
		repo.On("CountAdminNotifications", mock.Anything, adminID, &nType, &isRead).Return(int64(1), nil)

		dtos, total, err := svc.GetAdminNotifications(context.Background(), adminUUID, dto.AdminGetNotificationsQuery{
			Type:   &nType,
			IsRead: &isRead,
			Page:   1,
			Limit:  10,
		})
		assert.NoError(t, err)
		assert.Equal(t, int32(1), total)
		assert.Len(t, dtos, 1)
	})

	t.Run("GetAdminUnreadNotificationCount success", func(t *testing.T) {
		repo := new(mockNotificationRepo)
		idClient := new(mockIdentityClient)
		svc := service.NewNotificationService(repo, idClient, nil, nil, nil)

		idClient.On("GetUserByUUID", mock.Anything, adminUUID).Return(&identitycontract.InternalUserDTO{UserID: adminID}, nil)
		repo.On("CountAdminUnreadNotifications", mock.Anything, adminID).Return(int64(4), nil)

		res, err := svc.GetAdminUnreadNotificationCount(context.Background(), adminUUID)
		assert.NoError(t, err)
		assert.Equal(t, int64(4), res.UnreadCount)
	})

	t.Run("MarkAllAdminNotificationsAsRead success", func(t *testing.T) {
		repo := new(mockNotificationRepo)
		idClient := new(mockIdentityClient)
		svc := service.NewNotificationService(repo, idClient, nil, nil, nil)

		idClient.On("GetUserByUUID", mock.Anything, adminUUID).Return(&identitycontract.InternalUserDTO{UserID: adminID}, nil)
		repo.On("MarkAllAdminAsRead", mock.Anything, adminID).Return(nil)

		err := svc.MarkAllAdminNotificationsAsRead(context.Background(), adminUUID)
		assert.NoError(t, err)
	})
}
