package consumer_test

import (
	"context"
	"encoding/json"
	"testing"

	"goshop/pkg/logger"
	"goshop/pkg/rabbitmq"
	"goshop/services/notification/internal/consumer"
	"goshop/services/notification/internal/dto"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockNotificationService struct {
	mock.Mock
}

func (m *mockNotificationService) Send(ctx context.Context, input dto.SendNotificationInput) error {
	args := m.Called(ctx, input)
	return args.Error(0)
}

func (m *mockNotificationService) CreateNotification(ctx context.Context, input dto.CreateNotificationInput) (dto.NotificationDTO, error) {
	args := m.Called(ctx, input)
	return args.Get(0).(dto.NotificationDTO), args.Error(1)
}

func (m *mockNotificationService) BroadcastNotification(ctx context.Context, input dto.BroadcastNotificationInput) error {
	args := m.Called(ctx, input)
	return args.Error(0)
}

func (m *mockNotificationService) GetMyNotifications(ctx context.Context, userUUID uuid.UUID, query dto.GetNotificationsQuery) ([]dto.NotificationDTO, int32, error) {
	return nil, 0, nil
}

func (m *mockNotificationService) GetUnreadNotificationCount(ctx context.Context, userUUID uuid.UUID) (dto.UnreadCountDTO, error) {
	return dto.UnreadCountDTO{}, nil
}

func (m *mockNotificationService) MarkNotificationAsRead(ctx context.Context, userUUID uuid.UUID, notificationUUID uuid.UUID) (dto.NotificationDTO, error) {
	return dto.NotificationDTO{}, nil
}

func (m *mockNotificationService) MarkAllNotificationsAsRead(ctx context.Context, userUUID uuid.UUID) error {
	return nil
}

func (m *mockNotificationService) DeleteNotification(ctx context.Context, userUUID uuid.UUID, notificationUUID uuid.UUID) error {
	return nil
}

func (m *mockNotificationService) NotifyAdmins(ctx context.Context, input dto.AdminNotifyInput) error {
	args := m.Called(ctx, input)
	return args.Error(0)
}

func (m *mockNotificationService) GetAdminNotifications(ctx context.Context, userUUID uuid.UUID, query dto.AdminGetNotificationsQuery) ([]dto.NotificationDTO, int32, error) {
	return nil, 0, nil
}

func (m *mockNotificationService) GetAdminUnreadNotificationCount(ctx context.Context, userUUID uuid.UUID) (dto.UnreadCountDTO, error) {
	return dto.UnreadCountDTO{}, nil
}

func (m *mockNotificationService) MarkAllAdminNotificationsAsRead(ctx context.Context, userUUID uuid.UUID) error {
	return nil
}

func (m *mockNotificationService) PurgeStaleNotifications(ctx context.Context, retentionDays int) (int64, error) {
	return 0, nil
}

func TestTopicConsumer_OrderCreated(t *testing.T) {
	svc := new(mockNotificationService)
	log := logger.NewLogger(logger.LoggerConfig{Level: "debug", IsDev: "development"})

	c := consumer.NewTopicConsumer(svc, nil, nil, log)

	userID := int32(42)
	payload := map[string]any{
		"order_number": "ORD-12345",
		"user_id":      userID,
		"total_amount": 99.99,
	}
	body, _ := json.Marshal(payload)

	headers := amqp.Table{
		"x-routing-key": rabbitmq.RoutingKeyOrderCreated,
		"trace_id":      "trace-test-123",
	}

	svc.On("Send", mock.Anything, mock.MatchedBy(func(input dto.SendNotificationInput) bool {
		return input.UserID == 42 && input.Type == "order"
	})).Return(nil)

	svc.On("NotifyAdmins", mock.Anything, mock.MatchedBy(func(input dto.AdminNotifyInput) bool {
		return input.Type == "order"
	})).Return(nil)

	err := c.HandleEvent(headers, body)
	assert.NoError(t, err)
	svc.AssertExpectations(t)
}

func TestTopicConsumer_OrderStatusChanged_NumericStatus(t *testing.T) {
	svc := new(mockNotificationService)
	log := logger.NewLogger(logger.LoggerConfig{Level: "debug", IsDev: "development"})

	c := consumer.NewTopicConsumer(svc, nil, nil, log)

	userID := int32(42)
	payload := map[string]any{
		"order_number": "ORD-99999",
		"user_id":      userID,
		"new_status":   2, // numeric status (Confirmed)
	}
	body, _ := json.Marshal(payload)

	headers := amqp.Table{
		"x-routing-key": rabbitmq.RoutingKeyOrderStatusChanged,
		"x-message-id":  "msg-trace-456", // fallback when trace_id header is missing
	}

	svc.On("Send", mock.Anything, mock.MatchedBy(func(input dto.SendNotificationInput) bool {
		return input.UserID == 42 &&
			input.Type == "order_status" &&
			input.Data["status"] == "Confirmed"
	})).Return(nil)

	err := c.HandleEvent(headers, body)
	assert.NoError(t, err)
	svc.AssertExpectations(t)
}

func TestTopicConsumer_OrderStatusChanged_Shipped(t *testing.T) {
	svc := new(mockNotificationService)
	log := logger.NewLogger(logger.LoggerConfig{Level: "debug", IsDev: "development"})

	c := consumer.NewTopicConsumer(svc, nil, nil, log)

	userID := int32(42)
	payload := map[string]any{
		"order_number": "ORD-12345",
		"user_id":      userID,
		"new_status":   3, // numeric status (Shipped)
	}
	body, _ := json.Marshal(payload)

	headers := amqp.Table{
		"x-routing-key": rabbitmq.RoutingKeyOrderStatusChanged,
		"trace_id":      "trace-shipped-789",
	}

	svc.On("Send", mock.Anything, mock.MatchedBy(func(input dto.SendNotificationInput) bool {
		return input.UserID == 42 &&
			input.Type == "order_status" &&
			input.Title == "Order Shipped" &&
			input.Data["status"] == "Shipped"
	})).Return(nil)

	err := c.HandleEvent(headers, body)
	assert.NoError(t, err)
	svc.AssertExpectations(t)
}
