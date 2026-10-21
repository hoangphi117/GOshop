package worker

import (
	"context"
	"encoding/json"
	"fmt"

	v1service "goshop/internal/service/v1"
	"goshop/pkg/rabbitmq"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
)

// InventoryReturnRestockConsumer consumes order_return.completed events on topic queue inventory.order_events
// to idempotently restore returned items to warehouse inventory.
type InventoryReturnRestockConsumer struct {
	stockPort v1service.OrderStockPort
	logger    *zerolog.Logger
}

func NewInventoryReturnRestockConsumer(
	stockPort v1service.OrderStockPort,
	logger *zerolog.Logger,
) *InventoryReturnRestockConsumer {
	return &InventoryReturnRestockConsumer{
		stockPort: stockPort,
		logger:    logger,
	}
}

func (c *InventoryReturnRestockConsumer) Handle(headers amqp.Table, body []byte) error {
	var event rabbitmq.OrderReturnCompletedEvent
	if err := json.Unmarshal(body, &event); err != nil {
		if c.logger != nil {
			c.logger.Error().Err(err).Msg("Failed to unmarshal order return completed event in inventory consumer")
		}
		return err
	}

	if c.stockPort == nil || len(event.Items) == 0 {
		return nil
	}

	restoreItems := make([]v1service.RestoreItem, 0, len(event.Items))
	for _, it := range event.Items {
		if it.Restock && it.Quantity > 0 {
			restoreItems = append(restoreItems, v1service.RestoreItem{
				ProductID:   it.ProductID,
				ProductUUID: it.ProductUUID,
				Quantity:    it.Quantity,
			})
		}
	}

	if len(restoreItems) == 0 {
		if c.logger != nil {
			c.logger.Info().
				Str("return_uuid", event.ReturnUUID).
				Str("order_uuid", event.OrderUUID).
				Msg("No restockable items in order return completed event, skipping inventory update")
		}
		return nil
	}

	refID := event.ReturnUUID
	if refID == "" {
		refID = event.OrderNumber
	}

	ctx := context.Background()
	note := fmt.Sprintf("Restock from completed return (Order #%s, Return UUID %s)", event.OrderNumber, event.ReturnUUID)

	if err := c.stockPort.RestoreStockWithRef(ctx, restoreItems, refID, "return", nil, &note); err != nil {
		if c.logger != nil {
			c.logger.Error().
				Err(err).
				Str("return_uuid", event.ReturnUUID).
				Str("order_uuid", event.OrderUUID).
				Msg("Failed to restock returned items via inventory return restock consumer")
		}
		return err
	}

	if c.logger != nil {
		c.logger.Info().
			Str("return_uuid", event.ReturnUUID).
			Str("order_uuid", event.OrderUUID).
			Int("restocked_count", len(restoreItems)).
			Msg("Inventory restocked successfully from topic event order_return.completed")
	}

	return nil
}
