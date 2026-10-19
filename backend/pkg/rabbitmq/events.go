package rabbitmq

import (
	"time"

	"github.com/google/uuid"
)

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

	// Queues (Work Queue Point-to-Point)
	QueueOrderCreated         = "order.created"
	QueueOrderCancelled       = "order.cancelled"
	QueueOrderStatusChanged   = "order.status_changed"
	QueueOrderShipped         = "order.shipped"
	QueueOrderDelivered       = "order.delivered"
	QueueOrderReturnRequested = "order_return.requested"
	QueueOrderReturnUpdated   = "order_return.updated"
	QueueOrderReturnCompleted = "order_return.completed"
	QueuePaymentExpired       = "payment.expired"
	QueuePaymentSucceeded     = "payment.succeeded"
	QueuePaymentFailed        = "payment.failed"

	// Domain Event Queues (Pub/Sub Topic Exchange Subscribers)
	QueueInventoryOrderEvents  = "inventory.order_events"
	QueueInventoryReturnEvents = "inventory.return_events"
	QueueCouponOrderEvents     = "coupon.order_events"

	// User Sync Queues (Inter-Service Eventual Consistency)
	QueueUserCreated = "user.created"
	QueueUserUpdated = "user.updated"
	QueueUserDeleted = "user.deleted"

	// Topic Exchange Routing Keys (Pub/Sub)
	RoutingKeyOrderCreated           = "order.created"
	RoutingKeyOrderCancelled         = "order.cancelled"
	RoutingKeyOrderStatusChanged     = "order.status_changed"
	RoutingKeyOrderShipped           = "order.shipped"
	RoutingKeyOrderDelivered         = "order.delivered"
	RoutingKeyOrderReturnRequested   = "order_return.requested"
	RoutingKeyOrderReturnUpdated     = "order_return.updated"
	RoutingKeyOrderReturnCompleted   = "order_return.completed"
	RoutingKeyPaymentExpired       = "payment.expired"
	RoutingKeyPaymentSucceeded     = "payment.succeeded"
	RoutingKeyPaymentFailed        = "payment.failed"
	RoutingKeyUserCreated          = "user.created"
	RoutingKeyUserUpdated          = "user.updated"
	RoutingKeyUserDeleted          = "user.deleted"
	RoutingKeyInventoryStockLow    = "inventory.stock_low"
	RoutingKeyReviewReported       = "review.reported"

	// Topic Exchange Routing Patterns (Wildcards)
	PatternOrderAll     = "order.*"
	PatternOrderReturns = "order_return.*"
	PatternPaymentAll   = "payment.*"
	PatternUserAll      = "user.*"
	PatternInventoryAll = "inventory.*"
	PatternAllEvents    = "*.*"
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
	EventID            string    `json:"event_id"`
	OrderID            int32     `json:"order_id"`
	OrderUUID          string    `json:"order_uuid"`
	OrderNumber        string    `json:"order_number"`
	UserID             int32     `json:"user_id"`
	UserUUID           string    `json:"user_uuid"`
	TotalAmount        float64   `json:"total_amount"`
	PurchasedItems     []int32   `json:"purchased_items"`
	PurchasedItemUUIDs []string  `json:"purchased_item_uuids,omitempty"`
	Timestamp          time.Time `json:"timestamp"`
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

// WithItemUUIDs attaches product UUIDs to the event for microservice boundary decoupling.
func (e OrderCreatedEvent) WithItemUUIDs(uuids []string) OrderCreatedEvent {
	e.PurchasedItemUUIDs = uuids
	return e
}

// OrderCancelledItem details product item cancelled in an order.
type OrderCancelledItem struct {
	ProductID   int32  `json:"product_id"`
	ProductUUID string `json:"product_uuid,omitempty"`
	Quantity    int32  `json:"quantity"`
}

// OrderCancelledEvent is published when an order is cancelled.
type OrderCancelledEvent struct {
	EventID     string               `json:"event_id"`
	OrderID     int32                `json:"order_id"`
	OrderUUID   string               `json:"order_uuid"`
	OrderNumber string               `json:"order_number"`
	UserID      int32                `json:"user_id"`
	UserUUID    string               `json:"user_uuid"`
	Reason      string               `json:"reason,omitempty"`
	CouponID    *int32               `json:"coupon_id,omitempty"`
	CouponUUID  *string              `json:"coupon_uuid,omitempty"`
	Items       []OrderCancelledItem `json:"items,omitempty"`
	Timestamp   time.Time            `json:"timestamp"`
}

// NewOrderCancelledEvent constructs a new OrderCancelledEvent without detailed items (legacy).
func NewOrderCancelledEvent(eventID, orderUUID, orderNumber, userUUID string, orderID, userID int32, reason string) OrderCancelledEvent {
	return OrderCancelledEvent{
		EventID:     eventID,
		OrderID:     orderID,
		OrderUUID:   orderUUID,
		OrderNumber: orderNumber,
		UserID:      userID,
		UserUUID:    userUUID,
		Reason:      reason,
		Timestamp:   time.Now().UTC(),
	}
}

// NewOrderCancelledEventWithDetails constructs a complete OrderCancelledEvent with items and coupon.
func NewOrderCancelledEventWithDetails(
	eventID, orderUUID, orderNumber, userUUID string,
	orderID, userID int32,
	reason string,
	couponID *int32,
	items []OrderCancelledItem,
) OrderCancelledEvent {
	return OrderCancelledEvent{
		EventID:     eventID,
		OrderID:     orderID,
		OrderUUID:   orderUUID,
		OrderNumber: orderNumber,
		UserID:      userID,
		UserUUID:    userUUID,
		Reason:      reason,
		CouponID:    couponID,
		Items:       items,
		Timestamp:   time.Now().UTC(),
	}
}

// OrderStatusChangedEvent is published when an order status changes (e.g. Confivered, Shipped, Delivered).
type OrderStatusChangedEvent struct {
	EventID      string    `json:"event_id"`
	OrderID      int32     `json:"order_id"`
	OrderUUID    string    `json:"order_uuid"`
	OrderNumber  string    `json:"order_number"`
	UserID       int32     `json:"user_id"`
	UserUUID     string    `json:"user_uuid"`
	OldStatus    int32     `json:"old_status"`
	NewStatus    int32     `json:"new_status"`
	ShippingCode string    `json:"shipping_code,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// NewOrderStatusChangedEvent constructs a new OrderStatusChangedEvent.
func NewOrderStatusChangedEvent(eventID, orderUUID, orderNumber, userUUID string, orderID, userID, oldStatus, newStatus int32, shippingCode string) OrderStatusChangedEvent {
	return OrderStatusChangedEvent{
		EventID:      eventID,
		OrderID:      orderID,
		OrderUUID:    orderUUID,
		OrderNumber:  orderNumber,
		UserID:       userID,
		UserUUID:     userUUID,
		OldStatus:    oldStatus,
		NewStatus:    newStatus,
		ShippingCode: shippingCode,
		Timestamp:    time.Now().UTC(),
	}
}

// OrderReturnRequestedEvent is published when a customer creates a return request.
type OrderReturnRequestedEvent struct {
	EventID      string    `json:"event_id"`
	ReturnID     int32     `json:"return_id"`
	ReturnUUID   string    `json:"return_uuid"`
	OrderID      int32     `json:"order_id"`
	OrderUUID    string    `json:"order_uuid"`
	OrderNumber  string    `json:"order_number"`
	UserID       int32     `json:"user_id"`
	ReturnReason string    `json:"return_reason"`
	Timestamp    time.Time `json:"timestamp"`
}

// NewOrderReturnRequestedEvent constructs a new OrderReturnRequestedEvent.
func NewOrderReturnRequestedEvent(eventID, returnUUID, orderUUID, orderNumber string, returnID, orderID, userID int32, returnReason string) OrderReturnRequestedEvent {
	return OrderReturnRequestedEvent{
		EventID:      eventID,
		ReturnID:     returnID,
		ReturnUUID:   returnUUID,
		OrderID:      orderID,
		OrderUUID:    orderUUID,
		OrderNumber:  orderNumber,
		UserID:       userID,
		ReturnReason: returnReason,
		Timestamp:    time.Now().UTC(),
	}
}

// OrderReturnUpdatedEvent is published when an admin approves/rejects/updates a return request.
type OrderReturnUpdatedEvent struct {
	EventID      string    `json:"event_id"`
	ReturnID     int32     `json:"return_id"`
	ReturnUUID   string    `json:"return_uuid"`
	OrderID      int32     `json:"order_id"`
	OrderUUID    string    `json:"order_uuid"`
	OrderNumber  string    `json:"order_number"`
	UserID       int32     `json:"user_id"`
	ReturnStatus int16     `json:"return_status"`
	AdminNote    string    `json:"admin_note,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// NewOrderReturnUpdatedEvent constructs a new OrderReturnUpdatedEvent.
func NewOrderReturnUpdatedEvent(eventID, returnUUID, orderUUID, orderNumber string, returnID, orderID, userID int32, returnStatus int16, adminNote string) OrderReturnUpdatedEvent {
	return OrderReturnUpdatedEvent{
		EventID:      eventID,
		ReturnID:     returnID,
		ReturnUUID:   returnUUID,
		OrderID:      orderID,
		OrderUUID:    orderUUID,
		OrderNumber:  orderNumber,
		UserID:       userID,
		ReturnStatus: returnStatus,
		AdminNote:    adminNote,
		Timestamp:    time.Now().UTC(),
	}
}

// ReturnRestockItem specifies product items and whether they are restocked.
type ReturnRestockItem struct {
	ProductID   int32  `json:"product_id"`
	ProductUUID string `json:"product_uuid,omitempty"`
	Quantity    int32  `json:"quantity"`
	Restock     bool   `json:"restock"`
}

// OrderReturnCompletedEvent is published when a return request is completed and warehouse goods are restocked.
type OrderReturnCompletedEvent struct {
	EventID     string              `json:"event_id"`
	ReturnUUID  string              `json:"return_uuid"`
	OrderUUID   string              `json:"order_uuid"`
	OrderNumber string              `json:"order_number"`
	Items       []ReturnRestockItem `json:"items"`
	Timestamp   time.Time           `json:"timestamp"`
}

// NewOrderReturnCompletedEvent constructs a new OrderReturnCompletedEvent.
func NewOrderReturnCompletedEvent(
	eventID, returnUUID, orderUUID, orderNumber string,
	items []ReturnRestockItem,
) OrderReturnCompletedEvent {
	return OrderReturnCompletedEvent{
		EventID:     eventID,
		ReturnUUID:  returnUUID,
		OrderUUID:   orderUUID,
		OrderNumber: orderNumber,
		Items:       items,
		Timestamp:   time.Now().UTC(),
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

// UserSyncEvent is emitted when a user profile is created, updated, or deleted in Identity Service.
type UserSyncEvent struct {
	EventID       string    `json:"event_id"`
	EventType     string    `json:"event_type"` // "user.created", "user.updated", "user.deleted"
	UserID        int32     `json:"user_id"`
	UserUUID      string    `json:"user_uuid"`
	UserFullname  string    `json:"user_fullname"`
	UserEmail     string    `json:"user_email"`
	UserAvatar    string    `json:"user_avatar"`
	UserAvatarUrl *string   `json:"user_avatar_url,omitempty"`
	UserStatus    int32     `json:"user_status"`
	UserLevel     int32     `json:"user_level"`
	Timestamp     time.Time `json:"timestamp"`
}

// NewUserSyncEvent creates an event for inter-service user data synchronization.
func NewUserSyncEvent(eventType string, userID int32, userUUID, fullname, email, avatar string, avatarURL *string, status, level int32) UserSyncEvent {
	return UserSyncEvent{
		EventID:       uuid.New().String(),
		EventType:     eventType,
		UserID:        userID,
		UserUUID:      userUUID,
		UserFullname:  fullname,
		UserEmail:     email,
		UserAvatar:    avatar,
		UserAvatarUrl: avatarURL,
		UserStatus:    status,
		UserLevel:     level,
		Timestamp:     time.Now().UTC(),
	}
}
