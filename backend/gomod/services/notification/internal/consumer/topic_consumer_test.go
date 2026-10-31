package consumer_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"goshop/contracts/events"
	"goshop/services/notification/internal/consumer"
	"goshop/services/notification/internal/dto"
	"goshop/shared/pkg/logger"
	"goshop/shared/pkg/messaging"
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

	c := consumer.NewTopicConsumerWithLedger(svc, nil, nil, log, newFakeLedger())

	userID := int32(42)
	payload := map[string]any{
		"order_number": "ORD-12345",
		"user_id":      userID,
		"total_amount": 99.99,
	}
	body, _ := json.Marshal(payload)

	headers := messaging.Header{
		"x-routing-key": events.RoutingKeyOrderCreated,
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

	c := consumer.NewTopicConsumerWithLedger(svc, nil, nil, log, newFakeLedger())

	userID := int32(42)
	payload := map[string]any{
		"order_number": "ORD-99999",
		"user_id":      userID,
		"new_status":   2, // numeric status (Confirmed)
	}
	body, _ := json.Marshal(payload)

	headers := messaging.Header{
		"x-routing-key": events.RoutingKeyOrderStatusChanged,
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

	c := consumer.NewTopicConsumerWithLedger(svc, nil, nil, log, newFakeLedger())

	userID := int32(42)
	payload := map[string]any{
		"order_number": "ORD-12345",
		"user_id":      userID,
		"new_status":   3, // numeric status (Shipped)
	}
	body, _ := json.Marshal(payload)

	headers := messaging.Header{
		"x-routing-key": events.RoutingKeyOrderStatusChanged,
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

// fakeLedger is an in-memory EventLedgerRepository for redelivery tests.
type fakeLedger struct {
	marked    map[string]bool
	failCheck error
	failMark  error
}

func newFakeLedger() *fakeLedger {
	return &fakeLedger{marked: map[string]bool{}}
}

func (f *fakeLedger) IsEventProcessed(_ context.Context, eventID, _ string) (bool, error) {
	if f.failCheck != nil {
		return false, f.failCheck
	}
	return f.marked[eventID], nil
}

func (f *fakeLedger) MarkEventProcessed(_ context.Context, eventID, _ string) error {
	if f.failMark != nil {
		return f.failMark
	}
	f.marked[eventID] = true
	return nil
}

func orderCreatedBody(t *testing.T, eventID string) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"event_id":     eventID,
		"order_number": "ORD-777",
		"user_id":      42,
		"total_amount": 10.0,
	})
	assert.NoError(t, err)
	return body
}

func orderCreatedHeaders() messaging.Header {
	return messaging.Header{"x-routing-key": events.RoutingKeyOrderCreated, "trace_id": "trace-redelivery"}
}

// A redelivered event must produce exactly one logical notification.
func TestTopicConsumer_Redelivery_SideEffectRunsOnce(t *testing.T) {
	svc := new(mockNotificationService)
	log := logger.NewLogger(logger.LoggerConfig{Level: "debug", IsDev: "development"})
	ledger := newFakeLedger()

	c := consumer.NewTopicConsumerWithLedger(svc, nil, nil, log, ledger)

	svc.On("Send", mock.Anything, mock.MatchedBy(func(input dto.SendNotificationInput) bool {
		return input.UserID == 42 && input.Type == "order"
	})).Return(nil).Once()
	svc.On("NotifyAdmins", mock.Anything, mock.Anything).Return(nil).Once()

	body := orderCreatedBody(t, "evt-redelivery-1")
	assert.NoError(t, c.HandleEvent(orderCreatedHeaders(), body))
	assert.NoError(t, c.HandleEvent(orderCreatedHeaders(), body))

	svc.AssertExpectations(t)
	assert.True(t, ledger.marked["evt-redelivery-1"], "event must be recorded in the ledger")
}

// A failing side effect must neither mark the event nor lose it: the error
// returned makes the broker redeliver, and the retry applies the side effect.
func TestTopicConsumer_SideEffectFailure_RemainsRetryable(t *testing.T) {
	svc := new(mockNotificationService)
	log := logger.NewLogger(logger.LoggerConfig{Level: "debug", IsDev: "development"})
	ledger := newFakeLedger()

	c := consumer.NewTopicConsumerWithLedger(svc, nil, nil, log, ledger)

	svc.On("Send", mock.Anything, mock.Anything).Return(errors.New("db down")).Once()
	svc.On("Send", mock.Anything, mock.MatchedBy(func(input dto.SendNotificationInput) bool {
		return input.UserID == 42
	})).Return(nil).Once()
	svc.On("NotifyAdmins", mock.Anything, mock.Anything).Return(nil).Once()

	body := orderCreatedBody(t, "evt-retry-1")

	err := c.HandleEvent(orderCreatedHeaders(), body)
	assert.ErrorContains(t, err, "db down")
	assert.False(t, ledger.marked["evt-retry-1"], "failed event must stay unmarked for redelivery")

	assert.NoError(t, c.HandleEvent(orderCreatedHeaders(), body))
	svc.AssertExpectations(t)
	assert.True(t, ledger.marked["evt-retry-1"], "retried event must be marked after success")
}

// Duplicate deliveries of an already-processed event are dropped silently.
func TestTopicConsumer_DuplicateDelivery_SkipsSideEffects(t *testing.T) {
	svc := new(mockNotificationService)
	log := logger.NewLogger(logger.LoggerConfig{Level: "debug", IsDev: "development"})
	ledger := newFakeLedger()
	ledger.marked["evt-dup-1"] = true

	c := consumer.NewTopicConsumerWithLedger(svc, nil, nil, log, ledger)

	assert.NoError(t, c.HandleEvent(orderCreatedHeaders(), orderCreatedBody(t, "evt-dup-1")))
	svc.AssertNotCalled(t, "Send", mock.Anything, mock.Anything)
	svc.AssertNotCalled(t, "NotifyAdmins", mock.Anything, mock.Anything)
}

// A ledger write failure must surface: the event stays retryable instead of
// being acknowledged unrecorded.
func TestTopicConsumer_MarkFailure_ReturnsError(t *testing.T) {
	svc := new(mockNotificationService)
	log := logger.NewLogger(logger.LoggerConfig{Level: "debug", IsDev: "development"})
	ledger := newFakeLedger()
	ledger.failMark = errors.New("ledger write failed")

	c := consumer.NewTopicConsumerWithLedger(svc, nil, nil, log, ledger)

	svc.On("Send", mock.Anything, mock.Anything).Return(nil).Once()
	svc.On("NotifyAdmins", mock.Anything, mock.Anything).Return(nil).Once()

	err := c.HandleEvent(orderCreatedHeaders(), orderCreatedBody(t, "evt-markfail-1"))
	assert.ErrorContains(t, err, "ledger write failed")
	assert.Empty(t, ledger.marked, "event must not be recorded")
	svc.AssertExpectations(t)
}

// Envelope-less payloads (legacy publishers) deduplicate by content hash.
func TestTopicConsumer_LegacyPayloadWithoutEventID_StillDeduplicates(t *testing.T) {
	svc := new(mockNotificationService)
	log := logger.NewLogger(logger.LoggerConfig{Level: "debug", IsDev: "development"})
	ledger := newFakeLedger()

	c := consumer.NewTopicConsumerWithLedger(svc, nil, nil, log, ledger)

	body, err := json.Marshal(map[string]any{
		"order_number": "ORD-LEGACY",
		"user_id":      42,
		"total_amount": 5.0,
	})
	assert.NoError(t, err)

	svc.On("Send", mock.Anything, mock.MatchedBy(func(input dto.SendNotificationInput) bool {
		return input.UserID == 42 && input.Type == "order"
	})).Return(nil).Once()
	svc.On("NotifyAdmins", mock.Anything, mock.Anything).Return(nil).Once()

	assert.NoError(t, c.HandleEvent(orderCreatedHeaders(), body))
	assert.NoError(t, c.HandleEvent(orderCreatedHeaders(), body))
	svc.AssertExpectations(t)
	assert.Len(t, ledger.marked, 1, "legacy payload must be recorded once under its content hash")
}
