package worker

import (
	"context"
	"encoding/json"
	v1service "goshop/internal/service/v1"
	"goshop/pkg/rabbitmq"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
)

// InventoryOrderCancelledConsumer consumes order.cancelled events on topic queue inventory.order_events
// to idempotently release inventory stock reservations.
type InventoryOrderCancelledConsumer struct {
	stockPort v1service.OrderStockPort
	logger    *zerolog.Logger
}

func NewInventoryOrderCancelledConsumer(
	stockPort v1service.OrderStockPort,
	logger *zerolog.Logger,
) *InventoryOrderCancelledConsumer {
	return &InventoryOrderCancelledConsumer{
		stockPort: stockPort,
		logger:    logger,
	}
}

func (c *InventoryOrderCancelledConsumer) Handle(headers amqp.Table, body []byte) error {
	var event rabbitmq.OrderCancelledEvent
	if err := json.Unmarshal(body, &event); err != nil {
		if c.logger != nil {
			c.logger.Error().Err(err).Msg("Failed to unmarshal order cancelled event in inventory consumer")
		}
		return err
	}

	if c.stockPort == nil || len(event.Items) == 0 {
		return nil
	}

	restoreItems := make([]v1service.RestoreItem, len(event.Items))
	for i, it := range event.Items {
		restoreItems[i] = v1service.RestoreItem{
			ProductID: it.ProductID,
			Quantity:  it.Quantity,
		}
	}

	refID := event.OrderUUID
	if refID == "" {
		refID = event.OrderNumber
	}

	ctx := context.Background()
	reason := event.Reason
	if reason == "" {
		reason = "order_cancelled"
	}

	if err := c.stockPort.RestoreStockWithRef(ctx, restoreItems, refID, "order_cancel", nil, &reason); err != nil {
		if c.logger != nil {
			c.logger.Error().
				Err(err).
				Str("order_uuid", event.OrderUUID).
				Str("order_number", event.OrderNumber).
				Msg("Failed to restore stock via inventory order cancelled consumer")
		}
		return err
	}

	if c.logger != nil {
		c.logger.Info().
			Str("order_uuid", event.OrderUUID).
			Str("order_number", event.OrderNumber).
			Int("items_count", len(restoreItems)).
			Msg("Inventory restored successfully from topic event order.cancelled")
	}

	return nil
}
