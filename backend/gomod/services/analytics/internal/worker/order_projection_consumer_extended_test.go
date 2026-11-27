package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"goshop/contracts/events"
	ordercontract "goshop/services/order/contract"
	"goshop/shared/pkg/messaging"
)

type mockDBExecutor struct {
	mock.Mock
}

func (m *mockDBExecutor) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	args := m.Called(ctx, sql, arguments)
	return args.Get(0).(pgconn.CommandTag), args.Error(1)
}

type mockOrderReader struct {
	mock.Mock
}

func (m *mockOrderReader) GetOrderSummary(ctx context.Context, orderUUID uuid.UUID) (*ordercontract.OrderSummary, error) {
	args := m.Called(ctx, orderUUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ordercontract.OrderSummary), args.Error(1)
}

func TestOrderProjectionConsumer_HandleOrderCreated_WithDB(t *testing.T) {
	logger := zerolog.Nop()
	orderUUID := uuid.New()
	userUUID := uuid.New()

	t.Run("success with items and order reader enrichment", func(t *testing.T) {
		db := new(mockDBExecutor)
		orderReader := new(mockOrderReader)
		consumer := NewOrderProjectionConsumer(db, orderReader, &logger)

		orderReader.On("GetOrderSummary", mock.Anything, orderUUID).Return(&ordercontract.OrderSummary{
			OrderUUID:     orderUUID,
			CustomerName:  "Alice",
			CustomerEmail: "alice@example.com",
			Status:        2,
			TotalAmount:   300000.0,
		}, nil)

		db.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return len(sql) > 0
		}), mock.Anything).Return(pgconn.NewCommandTag("INSERT 1"), nil).Twice() // 1 order + 1 item

		itemUUID := uuid.New().String()
		createdEvt := events.OrderCreatedEvent{
			EventID:     uuid.New().String(),
			OrderID:     101,
			OrderUUID:   orderUUID.String(),
			OrderNumber: "ORD-101",
			UserUUID:    userUUID.String(),
			UserID:      55,
			TotalAmount: 200000.0,
			Items: []events.OrderCreatedItem{
				{
					ItemUUID:    itemUUID,
					ProductID:   1,
					ProductName: "Item 1",
					Price:       100000,
					Quantity:    2,
					Subtotal:    200000,
				},
			},
			Timestamp: time.Now().UTC(),
		}
		body, err := json.Marshal(createdEvt)
		assert.NoError(t, err)

		err = consumer.HandleOrderCreated(messaging.Header{}, body)
		assert.NoError(t, err)
		db.AssertExpectations(t)
		orderReader.AssertExpectations(t)
	})

	t.Run("fallback purchased items and db order exec error", func(t *testing.T) {
		db := new(mockDBExecutor)
		consumer := NewOrderProjectionConsumer(db, nil, &logger)

		db.On("Exec", mock.Anything, mock.Anything, mock.Anything).Return(pgconn.CommandTag{}, errors.New("db insert fail")).Once()

		createdEvt := events.OrderCreatedEvent{
			OrderID:            102,
			OrderUUID:          uuid.New().String(),
			PurchasedItems:     []int32{10, 20},
			PurchasedItemUUIDs: []string{uuid.New().String()},
		}
		body, err := json.Marshal(createdEvt)
		assert.NoError(t, err)

		err = consumer.HandleOrderCreated(messaging.Header{}, body)
		assert.Error(t, err)
	})

	t.Run("db item insert error", func(t *testing.T) {
		db := new(mockDBExecutor)
		consumer := NewOrderProjectionConsumer(db, nil, &logger)

		db.On("Exec", mock.Anything, mock.Anything, mock.Anything).Return(pgconn.NewCommandTag("INSERT 1"), nil).Once()
		db.On("Exec", mock.Anything, mock.Anything, mock.Anything).Return(pgconn.CommandTag{}, errors.New("item insert fail")).Once()

		createdEvt := events.OrderCreatedEvent{
			OrderID:   103,
			OrderUUID: uuid.New().String(),
			Items: []events.OrderCreatedItem{
				{ProductID: 1, ProductName: "P1", Price: 10, Quantity: 1, Subtotal: 10},
			},
		}
		body, err := json.Marshal(createdEvt)
		assert.NoError(t, err)

		err = consumer.HandleOrderCreated(messaging.Header{}, body)
		assert.Error(t, err)
	})
}

func TestOrderProjectionConsumer_HandleOrderStatusChanged_WithDB(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("success", func(t *testing.T) {
		db := new(mockDBExecutor)
		consumer := NewOrderProjectionConsumer(db, nil, &logger)

		db.On("Exec", mock.Anything, mock.Anything, mock.Anything).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()

		evt := events.OrderStatusChangedEvent{
			OrderUUID:     uuid.New().String(),
			OrderID:       201,
			NewStatus:     4, // Delivered
			ShippingCode:  "SHIP-123",
			PaymentStatus: 2,
		}
		body, err := json.Marshal(evt)
		assert.NoError(t, err)

		err = consumer.HandleOrderStatusChanged(messaging.Header{}, body)
		assert.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		db := new(mockDBExecutor)
		consumer := NewOrderProjectionConsumer(db, nil, &logger)

		db.On("Exec", mock.Anything, mock.Anything, mock.Anything).Return(pgconn.CommandTag{}, errors.New("update fail")).Once()

		evt := events.OrderStatusChangedEvent{OrderUUID: uuid.New().String()}
		body, _ := json.Marshal(evt)

		err := consumer.HandleOrderStatusChanged(messaging.Header{}, body)
		assert.Error(t, err)
	})
}

func TestOrderProjectionConsumer_HandleOrderCancelled_WithDB(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("success", func(t *testing.T) {
		db := new(mockDBExecutor)
		consumer := NewOrderProjectionConsumer(db, nil, &logger)

		db.On("Exec", mock.Anything, mock.Anything, mock.Anything).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()

		evt := events.OrderCancelledEvent{
			OrderUUID: uuid.New().String(),
			OrderID:   301,
		}
		body, _ := json.Marshal(evt)

		err := consumer.HandleOrderCancelled(messaging.Header{}, body)
		assert.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		db := new(mockDBExecutor)
		consumer := NewOrderProjectionConsumer(db, nil, &logger)

		db.On("Exec", mock.Anything, mock.Anything, mock.Anything).Return(pgconn.CommandTag{}, errors.New("cancel fail")).Once()

		evt := events.OrderCancelledEvent{OrderUUID: uuid.New().String()}
		body, _ := json.Marshal(evt)

		err := consumer.HandleOrderCancelled(messaging.Header{}, body)
		assert.Error(t, err)
	})
}

func TestOrderProjectionConsumer_HandleOrderReturnUpdated_WithDB(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("refund amount zero is no-op", func(t *testing.T) {
		db := new(mockDBExecutor)
		consumer := NewOrderProjectionConsumer(db, nil, &logger)

		evt := events.OrderReturnUpdatedEvent{
			OrderUUID:    uuid.New().String(),
			ChangeType:   events.ReturnChangeTypeRefunded,
			RefundAmount: 0,
		}
		body, _ := json.Marshal(evt)

		err := consumer.HandleOrderReturnUpdated(messaging.Header{}, body)
		assert.NoError(t, err)
	})

	t.Run("refund amount executes update", func(t *testing.T) {
		db := new(mockDBExecutor)
		consumer := NewOrderProjectionConsumer(db, nil, &logger)

		db.On("Exec", mock.Anything, mock.Anything, mock.Anything).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()

		evt := events.OrderReturnUpdatedEvent{
			OrderUUID:    uuid.New().String(),
			ChangeType:   events.ReturnChangeTypeRefunded,
			RefundAmount: 50000,
		}
		body, _ := json.Marshal(evt)

		err := consumer.HandleOrderReturnUpdated(messaging.Header{}, body)
		assert.NoError(t, err)
	})
}

func TestOrderProjectionConsumer_HandlePaymentRefunded_WithDB(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("invalid json", func(t *testing.T) {
		consumer := NewOrderProjectionConsumer(nil, nil, &logger)
		err := consumer.HandlePaymentRefunded(messaging.Header{}, []byte("{bad}"))
		assert.Error(t, err)
	})

	t.Run("refund amount zero is no-op", func(t *testing.T) {
		db := new(mockDBExecutor)
		consumer := NewOrderProjectionConsumer(db, nil, &logger)

		body := []byte(`{"order_uuid":"` + uuid.New().String() + `","amount":0}`)
		err := consumer.HandlePaymentRefunded(messaging.Header{}, body)
		assert.NoError(t, err)
	})

	t.Run("refund amount executes update", func(t *testing.T) {
		db := new(mockDBExecutor)
		consumer := NewOrderProjectionConsumer(db, nil, &logger)

		db.On("Exec", mock.Anything, mock.Anything, mock.Anything).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()

		body := []byte(`{"order_uuid":"` + uuid.New().String() + `","order_id":401,"amount":75000}`)
		err := consumer.HandlePaymentRefunded(messaging.Header{}, body)
		assert.NoError(t, err)
	})
}

func TestOrderProjectionConsumer_HandlePaymentSucceeded_WithDB(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("success", func(t *testing.T) {
		db := new(mockDBExecutor)
		consumer := NewOrderProjectionConsumer(db, nil, &logger)

		db.On("Exec", mock.Anything, mock.Anything, mock.Anything).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()

		evt := events.PaymentSucceededEvent{
			OrderUUID:   uuid.New().String(),
			OrderNumber: "ORD-999",
		}
		body, _ := json.Marshal(evt)

		err := consumer.HandlePaymentSucceeded(messaging.Header{}, body)
		assert.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		db := new(mockDBExecutor)
		consumer := NewOrderProjectionConsumer(db, nil, &logger)

		db.On("Exec", mock.Anything, mock.Anything, mock.Anything).Return(pgconn.CommandTag{}, errors.New("update payment fail")).Once()

		evt := events.PaymentSucceededEvent{
			OrderUUID:   uuid.New().String(),
			OrderNumber: "ORD-999",
		}
		body, _ := json.Marshal(evt)

		err := consumer.HandlePaymentSucceeded(messaging.Header{}, body)
		assert.Error(t, err)
	})
}
