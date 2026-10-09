package rabbitmq

import "time"

const (
	QueueNotificationEvents       = "notification_events"
	QueueAdminNotificationsEvents = QueueNotificationEvents // Backward compatibility alias

	// Event Types
	EventTypeOrderCreated   = "order"
	EventTypeStockLow       = "inventory"
	EventReturnRequested    = "order_return"
	EventTypeReviewReported = "review"

	// Actions
	ActionCreated   = "created"
	ActionStockLow  = "stock_low"
	ActionRequested = "requested"
	ActionReported  = "reported"

	// Queues
	QueueOrderCreated     = "order.created"
	QueuePaymentExpired   = "payment.expired"
	QueuePaymentSucceeded = "payment.succeeded"
	QueuePaymentFailed    = "payment.failed"
)

// PaymentExpiredEvent is emitted when a payment transaction exceeds its timeout and expires.
type PaymentExpiredEvent struct {
	EventID       string    `json:"event_id"`
	PtUUID        string    `json:"pt_uuid"`
	OrderUUID     string    `json:"order_uuid"`
	OrderNumber   string    `json:"order_number"`
	UserUUID      string    `json:"user_uuid"`
	PaymentMethod string    `json:"payment_method"`
	Timestamp     time.Time `json:"timestamp"`
}

// NewPaymentExpiredEvent constructs a new PaymentExpiredEvent.
func NewPaymentExpiredEvent(eventID, ptUUID, orderUUID, orderNumber, userUUID, paymentMethod string) PaymentExpiredEvent {
	return PaymentExpiredEvent{
		EventID:       eventID,
		PtUUID:        ptUUID,
		OrderUUID:     orderUUID,
		OrderNumber:   orderNumber,
		UserUUID:      userUUID,
		PaymentMethod: paymentMethod,
		Timestamp:     time.Now().UTC(),
	}
}

// PaymentSucceededEvent is emitted when a payment transaction succeeds.
type PaymentSucceededEvent struct {
	EventID         string    `json:"event_id"`
	PtUUID          string    `json:"pt_uuid"`
	OrderUUID       string    `json:"order_uuid"`
	OrderNumber     string    `json:"order_number"`
	UserUUID        string    `json:"user_uuid"`
	Provider        string    `json:"provider"`
	ProviderTxnID   string    `json:"provider_txn_id"`
	Amount          float64   `json:"amount"`
	Timestamp       time.Time `json:"timestamp"`
}

// NewPaymentSucceededEvent constructs a new PaymentSucceededEvent.
func NewPaymentSucceededEvent(eventID, ptUUID, orderUUID, orderNumber, userUUID, provider, providerTxnID string, amount float64) PaymentSucceededEvent {
	return PaymentSucceededEvent{
		EventID:         eventID,
		PtUUID:          ptUUID,
		OrderUUID:       orderUUID,
		OrderNumber:     orderNumber,
		UserUUID:        userUUID,
		Provider:        provider,
		ProviderTxnID:   providerTxnID,
		Amount:          amount,
		Timestamp:       time.Now().UTC(),
	}
}

// PaymentFailedEvent is emitted when a payment transaction fails.
type PaymentFailedEvent struct {
	EventID     string    `json:"event_id"`
	PtUUID      string    `json:"pt_uuid"`
	OrderUUID   string    `json:"order_uuid"`
	OrderNumber string    `json:"order_number"`
	UserUUID    string    `json:"user_uuid"`
	Provider    string    `json:"provider"`
	Reason      string    `json:"reason"`
	Timestamp   time.Time `json:"timestamp"`
}

// NewPaymentFailedEvent constructs a new PaymentFailedEvent.
func NewPaymentFailedEvent(eventID, ptUUID, orderUUID, orderNumber, userUUID, provider, reason string) PaymentFailedEvent {
	return PaymentFailedEvent{
		EventID:     eventID,
		PtUUID:      ptUUID,
		OrderUUID:   orderUUID,
		OrderNumber: orderNumber,
		UserUUID:    userUUID,
		Provider:    provider,
		Reason:      reason,
		Timestamp:   time.Now().UTC(),
	}
}

// OrderCreatedEvent is published when an order has been successfully placed and committed to DB.
type OrderCreatedEvent struct {
	EventID        string    `json:"event_id"`
	OrderID        int32     `json:"order_id"`
	OrderUUID      string    `json:"order_uuid"`
	OrderNumber    string    `json:"order_number"`
	UserID         int32     `json:"user_id"`
	UserUUID       string    `json:"user_uuid"`
	TotalAmount    float64   `json:"total_amount"`
	PurchasedItems []int32   `json:"purchased_items"`
	Timestamp      time.Time `json:"timestamp"`
}

// NewOrderCreatedEvent constructs a new OrderCreatedEvent.
func NewOrderCreatedEvent(eventID, orderUUID, orderNumber, userUUID string, orderID, userID int32, totalAmount float64, purchasedItems []int32) OrderCreatedEvent {
	return OrderCreatedEvent{
		EventID:        eventID,
		OrderID:        orderID,
		OrderUUID:      orderUUID,
		OrderNumber:    orderNumber,
		UserID:         userID,
		UserUUID:       userUUID,
		TotalAmount:    totalAmount,
		PurchasedItems: purchasedItems,
		Timestamp:      time.Now().UTC(),
	}
}

// NotificationEvent is the unified event envelope for both Customer and Admin notifications.
type NotificationEvent struct {
	EventID   string         `json:"event_id"`
	UserID    *int32         `json:"user_id,omitempty"` // nil = Admin Fanout, non-nil = Customer Unicast
	Type      string         `json:"type"`
	Action    string         `json:"action"`
	Title     string         `json:"title"`
	Body      string         `json:"body"`
	Data      map[string]any `json:"data,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// AdminNotificationEvent is an alias for backward compatibility.
type AdminNotificationEvent = NotificationEvent

// NewAdminNotificationEvent creates an event intended for all administrators (UserID = nil).
func NewAdminNotificationEvent(eventID, eventType, action, title, body string, data map[string]any) NotificationEvent {
	return NotificationEvent{
		EventID:   eventID,
		UserID:    nil,
		Type:      eventType,
		Action:    action,
		Title:     title,
		Body:      body,
		Data:      data,
		Timestamp: time.Now().UTC(),
	}
}

// NewCustomerNotificationEvent creates an event targeted to a specific customer.
func NewCustomerNotificationEvent(eventID string, userID int32, eventType, action, title, body string, data map[string]any) NotificationEvent {
	return NotificationEvent{
		EventID:   eventID,
		UserID:    &userID,
		Type:      eventType,
		Action:    action,
		Title:     title,
		Body:      body,
		Data:      data,
		Timestamp: time.Now().UTC(),
	}
}
