package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"goshop/pkg/rabbitmq"
	"goshop/services/analytics/internal/service"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
)

// OrderProjectionConsumer consumes order lifecycle events from RabbitMQ and
// maintains the reporting read model (analytics_orders_projection,
// analytics_order_items_projection) inside the analytics database.
type OrderProjectionConsumer struct {
	dbPool      *pgxpool.Pool
	orderReader service.OrderReader
	logger      *zerolog.Logger
}

// NewOrderProjectionConsumer constructs a new OrderProjectionConsumer.
func NewOrderProjectionConsumer(dbPool *pgxpool.Pool, orderReader service.OrderReader, logger *zerolog.Logger) *OrderProjectionConsumer {
	return &OrderProjectionConsumer{
		dbPool:      dbPool,
		orderReader: orderReader,
		logger:      logger,
	}
}

// HandleOrderCreated processes order.created event to upsert projection order and item rows.
func (c *OrderProjectionConsumer) HandleOrderCreated(headers amqp.Table, body []byte) error {
	var event rabbitmq.OrderCreatedEvent
	if err := json.Unmarshal(body, &event); err != nil {
		if c.logger != nil {
			c.logger.Error().Err(err).Msg("failed to unmarshal OrderCreatedEvent for projection")
		}
		return err
	}

	if c.dbPool == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	parsedOrderUUID, _ := uuid.Parse(event.OrderUUID)
	parsedUserUUID, _ := uuid.Parse(event.UserUUID)

	customerName := ""
	customerEmail := ""
	status := int32(1)
	totalAmount := event.TotalAmount

	if c.orderReader != nil && parsedOrderUUID != uuid.Nil {
		if summary, err := c.orderReader.GetOrderSummary(ctx, parsedOrderUUID); err == nil && summary != nil {
			if summary.CustomerName != "" {
				customerName = summary.CustomerName
			}
			if summary.CustomerEmail != "" {
				customerEmail = summary.CustomerEmail
			}
			if summary.Status > 0 {
				status = summary.Status
			}
			if summary.TotalAmount > 0 {
				totalAmount = summary.TotalAmount
			}
		}
	}

	upsertOrderSQL := `
		INSERT INTO analytics_orders_projection (
			order_id, order_uuid, order_number, order_user_id, order_user_uuid,
			order_customer_name, order_customer_email, order_shipping_name,
			order_shipping_phone, order_shipping_address, order_total_amount,
			order_refunded_amount, order_discount_amount,
			order_payment_method, order_payment_status, order_shipping_fee,
			order_status, order_created_at, order_updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8,
			$9, $10, $11,
			0.00, $12,
			COALESCE(NULLIF($13, 0), 1), 1, $14,
			$15, $16, $16
		)
		ON CONFLICT (order_id) DO UPDATE SET
			order_status = EXCLUDED.order_status,
			order_total_amount = EXCLUDED.order_total_amount,
			order_discount_amount = EXCLUDED.order_discount_amount,
			order_payment_method = EXCLUDED.order_payment_method,
			order_shipping_fee = EXCLUDED.order_shipping_fee,
			order_customer_name = CASE WHEN EXCLUDED.order_customer_name <> '' THEN EXCLUDED.order_customer_name ELSE analytics_orders_projection.order_customer_name END,
			order_customer_email = CASE WHEN EXCLUDED.order_customer_email <> '' THEN EXCLUDED.order_customer_email ELSE analytics_orders_projection.order_customer_email END,
			order_updated_at = NOW();
	`

	createdAt := event.Timestamp
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	shippingName := customerName
	if shippingName == "" {
		shippingName = "Customer"
	}

	_, err := c.dbPool.Exec(ctx, upsertOrderSQL,
		event.OrderID,
		parsedOrderUUID,
		event.OrderNumber,
		event.UserID,
		parsedUserUUID,
		customerName,
		customerEmail,
		shippingName,
		"N/A",
		"N/A",
		totalAmount,
		event.DiscountAmount,
		event.PaymentMethod,
		event.ShippingFee,
		status,
		createdAt,
	)
	if err != nil {
		if c.logger != nil {
			c.logger.Error().Err(err).Str("order_uuid", event.OrderUUID).Msg("failed to upsert analytics_orders_projection")
		}
		return err
	}

	upsertItemSQL := `
		INSERT INTO analytics_order_items_projection (
			order_item_uuid, order_item_order_id,
			order_item_product_id, order_item_product_name,
			order_item_price, order_item_quantity, order_item_subtotal, order_item_created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		)
		ON CONFLICT (order_item_uuid) DO NOTHING;
	`

	items := event.Items
	if len(items) == 0 {
		for idx, productID := range event.PurchasedItems {
			productUUID := ""
			if idx < len(event.PurchasedItemUUIDs) {
				productUUID = event.PurchasedItemUUIDs[idx]
			}
			items = append(items, rabbitmq.OrderCreatedItem{
				ProductID:   productID,
				ProductUUID: productUUID,
				ProductName: "Product",
				Price:       1,
				Quantity:    1,
				Subtotal:    1,
			})
		}
	}

	for idx, item := range items {
		itemUUID := uuid.New()
		if item.ItemUUID != "" {
			if parsed, err := uuid.Parse(item.ItemUUID); err == nil {
				itemUUID = parsed
			}
		} else {
			itemUUID = uuid.NewSHA1(uuid.Nil, []byte(fmt.Sprintf("%d:%d:%d", event.OrderID, item.ProductID, idx)))
		}
		_, err = c.dbPool.Exec(ctx, upsertItemSQL,
			itemUUID,
			event.OrderID,
			item.ProductID,
			item.ProductName,
			item.Price,
			item.Quantity,
			item.Subtotal,
			createdAt,
		)
		if err != nil {
			if c.logger != nil {
				c.logger.Error().Err(err).Int32("order_id", event.OrderID).Int32("product_id", item.ProductID).Msg("failed to upsert analytics_order_items_projection")
			}
			return err
		}
	}

	if c.logger != nil {
		c.logger.Info().Str("order_uuid", event.OrderUUID).Int32("order_id", event.OrderID).Msg("projected OrderCreatedEvent into analytics read model")
	}

	return nil
}

// HandleOrderStatusChanged updates order status, shipping code, and delivered_at in projection.
func (c *OrderProjectionConsumer) HandleOrderStatusChanged(headers amqp.Table, body []byte) error {
	var event rabbitmq.OrderStatusChangedEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return err
	}

	if c.dbPool == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	parsedOrderUUID, _ := uuid.Parse(event.OrderUUID)

	query := `
		UPDATE analytics_orders_projection
		SET order_status = $1,
		    order_payment_status = CASE WHEN $5 > 0 THEN $5 ELSE order_payment_status END,
		    order_shipping_code = CASE WHEN $2 <> '' THEN $2 ELSE order_shipping_code END,
		    order_delivered_at = CASE WHEN $1 = 4 THEN COALESCE(order_delivered_at, NOW()) ELSE order_delivered_at END,
		    order_updated_at = NOW()
		WHERE order_uuid = $3 OR order_id = $4;
	`

	_, err := c.dbPool.Exec(ctx, query, event.NewStatus, event.ShippingCode, parsedOrderUUID, event.OrderID, event.PaymentStatus)
	if err != nil && c.logger != nil {
		c.logger.Error().Err(err).Str("order_uuid", event.OrderUUID).Msg("failed to update status in analytics_orders_projection")
	}
	return err
}

// HandleOrderCancelled updates status to cancelled (5) and records cancellation time.
func (c *OrderProjectionConsumer) HandleOrderCancelled(headers amqp.Table, body []byte) error {
	var event rabbitmq.OrderCancelledEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return err
	}

	if c.dbPool == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	parsedOrderUUID, _ := uuid.Parse(event.OrderUUID)

	query := `
		UPDATE analytics_orders_projection
		SET order_status = 5,
		    order_cancelled_at = COALESCE(order_cancelled_at, NOW()),
		    order_updated_at = NOW()
		WHERE order_uuid = $1 OR order_id = $2;
	`

	_, err := c.dbPool.Exec(ctx, query, parsedOrderUUID, event.OrderID)
	if err != nil && c.logger != nil {
		c.logger.Error().Err(err).Str("order_uuid", event.OrderUUID).Msg("failed to record cancellation in analytics_orders_projection")
	}
	return err
}

// HandlePaymentSucceeded updates payment status to paid (2) in projection.
func (c *OrderProjectionConsumer) HandlePaymentSucceeded(headers amqp.Table, body []byte) error {
	var event rabbitmq.PaymentSucceededEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return err
	}

	if c.dbPool == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	parsedOrderUUID, _ := uuid.Parse(event.OrderUUID)

	query := `
		UPDATE analytics_orders_projection
		SET order_payment_status = 2,
		    order_updated_at = NOW()
		WHERE order_uuid = $1 OR order_number = $2;
	`

	_, err := c.dbPool.Exec(ctx, query, parsedOrderUUID, event.OrderNumber)
	if err != nil && c.logger != nil {
		c.logger.Error().Err(err).Str("order_uuid", event.OrderUUID).Msg("failed to update payment status in analytics_orders_projection")
	}
	return err
}