package service_test

import (
	"context"
	"testing"
	"time"

	"goshop/services/notification/internal/db/sqlc"
	"goshop/services/notification/internal/dto"
	"goshop/services/notification/internal/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockNotificationRepo struct {
	mock.Mock
}

func (m *mockNotificationRepo) Create(ctx context.Context, params sqlc.CreateNotificationParams) (sqlc.Notification, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(sqlc.Notification), args.Error(1)
}

func (m *mockNotificationRepo) GetByUserID(ctx context.Context, userID int32, limit, offset int32) ([]sqlc.Notification, error) {
	args := m.Called(ctx, userID, limit, offset)
	return args.Get(0).([]sqlc.Notification), args.Error(1)
}

func (m *mockNotificationRepo) CountByUserID(ctx context.Context, userID int32) (int64, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockNotificationRepo) CountUnreadByUserID(ctx context.Context, userID int32) (int64, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockNotificationRepo) MarkAsRead(ctx context.Context, notifUUID uuid.UUID, userID int32) (sqlc.Notification, error) {
	args := m.Called(ctx, notifUUID, userID)
	return args.Get(0).(sqlc.Notification), args.Error(1)
}

func (m *mockNotificationRepo) MarkAllAsRead(ctx context.Context, userID int32) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *mockNotificationRepo) Delete(ctx context.Context, notifUUID uuid.UUID, userID int32) error {
	args := m.Called(ctx, notifUUID, userID)
	return args.Error(0)
}

func (m *mockNotificationRepo) GetAdminNotifications(ctx context.Context, userID int32, nType *string, isRead *bool, limit, offset int32) ([]sqlc.Notification, error) {
	args := m.Called(ctx, userID, nType, isRead, limit, offset)
	return args.Get(0).([]sqlc.Notification), args.Error(1)
}

func (m *mockNotificationRepo) CountAdminNotifications(ctx context.Context, userID int32, nType *string, isRead *bool) (int64, error) {
	args := m.Called(ctx, userID, nType, isRead)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockNotificationRepo) CountAdminUnreadNotifications(ctx context.Context, userID int32) (int64, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockNotificationRepo) MarkAllAdminAsRead(ctx context.Context, userID int32) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *mockNotificationRepo) DeleteOldReadNotifications(ctx context.Context, cutoff time.Time) (int64, error) {
	args := m.Called(ctx, cutoff)
	return args.Get(0).(int64), args.Error(1)
}

func TestNotificationService_Send(t *testing.T) {
	repo := new(mockNotificationRepo)
	svc := service.NewNotificationService(repo, nil, nil, nil, nil)

	input := dto.SendNotificationInput{
		UserID: 10,
		Type:   "order",
		Title:  "Order Placed",
		Body:   "Order #1234 has been created",
		Data:   map[string]any{"order_id": "1234"},
	}

	expectedNotif := sqlc.Notification{
		NotificationID:        1,
		NotificationUuid:      uuid.New(),
		NotificationUserID:    10,
		NotificationType:      "order",
		NotificationTitle:     input.Title,
		NotificationBody:      input.Body,
		NotificationIsRead:    false,
		NotificationCreatedAt: time.Now(),
	}

	repo.On("Create", mock.Anything, mock.MatchedBy(func(p sqlc.CreateNotificationParams) bool {
		return p.NotificationUserID == 10 && p.NotificationType == "order"
	})).Return(expectedNotif, nil)

	err := svc.Send(context.Background(), input)
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestNotificationService_PurgeStaleNotifications(t *testing.T) {
	repo := new(mockNotificationRepo)
	svc := service.NewNotificationService(repo, nil, nil, nil, nil)

	repo.On("DeleteOldReadNotifications", mock.Anything, mock.Anything).Return(int64(5), nil)

	count, err := svc.PurgeStaleNotifications(context.Background(), 30)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), count)
	repo.AssertExpectations(t)
}
