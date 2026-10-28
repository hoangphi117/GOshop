package worker

import (
	"encoding/json"
	"goshop/pkg/rabbitmq"
	"testing"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
)

func TestOrderProjectionConsumer_HandleInvalidJSON(t *testing.T) {
	consumer := NewOrderProjectionConsumer(nil, nil, nil)
	badBody := []byte("{invalid-json}")
	headers := amqp.Table{}

	assert.Error(t, consumer.HandleOrderCreated(headers, badBody))
	assert.Error(t, consumer.HandleOrderStatusChanged(headers, badBody))
	assert.Error(t, consumer.HandleOrderCancelled(headers, badBody))
	assert.Error(t, consumer.HandlePaymentSucceeded(headers, badBody))
}

func TestOrderProjectionConsumer_HandleNilDB(t *testing.T) {
	consumer := NewOrderProjectionConsumer(nil, nil, nil)
	headers := amqp.Table{}

	// Order Created
	createdEvt := rabbitmq.NewOrderCreatedEvent(
		uuid.New().String(),
		uuid.New().String(),
		"ORD-TEST-001",
		uuid.New().String(),
		1001,
		501,
		250000.0,
		[]int32{1, 2},
	)
	body, err := json.Marshal(createdEvt)
	assert.NoError(t, err)
	assert.NoError(t, consumer.HandleOrderCreated(headers, body))

	// Order Status Changed
	statusEvt := rabbitmq.NewOrderStatusChangedEvent(
		uuid.New().String(),
		uuid.New().String(),
		"ORD-TEST-001",
		uuid.New().String(),
		1001,
		501,
		1,
		4,
		"GHN123456",
	)
	body, err = json.Marshal(statusEvt)
	assert.NoError(t, err)
	assert.NoError(t, consumer.HandleOrderStatusChanged(headers, body))

	// Order Cancelled
	cancelEvt := rabbitmq.NewOrderCancelledEvent(
		uuid.New().String(),
		uuid.New().String(),
		"ORD-TEST-001",
		uuid.New().String(),
		1001,
		501,
		"Customer requested",
	)
	body, err = json.Marshal(cancelEvt)
	assert.NoError(t, err)
	assert.NoError(t, consumer.HandleOrderCancelled(headers, body))

	// Payment Succeeded
	payEvt := rabbitmq.NewPaymentSucceededEvent(
		uuid.New().String(),
		uuid.New().String(),
		uuid.New().String(),
		"ORD-TEST-001",
		uuid.New().String(),
		"VNPAY",
		"TXN-999",
		250000.0,
	)
	body, err = json.Marshal(payEvt)
	assert.NoError(t, err)
	assert.NoError(t, consumer.HandlePaymentSucceeded(headers, body))
}

func TestOrderProjectionConsumer_StructConstruction(t *testing.T) {
	c := NewOrderProjectionConsumer(nil, nil, nil)
	assert.NotNil(t, c)
	assert.Nil(t, c.dbPool)
	assert.Nil(t, c.orderReader)
	assert.Nil(t, c.logger)
}

func TestOrderProjectionConsumer_TimestampFallback(t *testing.T) {
	consumer := NewOrderProjectionConsumer(nil, nil, nil)
	headers := amqp.Table{}

	// Event with zero timestamp should fall back to time.Now()
	createdEvt := rabbitmq.OrderCreatedEvent{
		EventID:     uuid.New().String(),
		OrderID:     1002,
		OrderUUID:   uuid.New().String(),
		OrderNumber: "ORD-002",
		Timestamp:   time.Time{}, // zero value
	}
	body, err := json.Marshal(createdEvt)
	assert.NoError(t, err)
	assert.NoError(t, consumer.HandleOrderCreated(headers, body))
}
