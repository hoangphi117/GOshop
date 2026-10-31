package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	inventorydto "goshop/services/inventory/internal/dto"
	inventoryservice "goshop/services/inventory/internal/service"
)

// OrderCancelledConsumer restores reserved stock when an order is cancelled.
// Inventory owns this state transition (branch F); RestoreStockWithRef is
// idempotent per reference ID, so redeliveries cannot double-apply.
type OrderCancelledConsumer struct {
	inventoryService inventoryservice.InventoryService
}

func NewOrderCancelledConsumer(inventoryService inventoryservice.InventoryService) *OrderCancelledConsumer {
	return &OrderCancelledConsumer{inventoryService: inventoryService}
}

// Apply restores the stock the cancelled order had reserved.
func (c *OrderCancelledConsumer) Apply(ctx context.Context, body []byte) error {
	var event struct {
		OrderUUID   string `json:"order_uuid"`
		OrderNumber string `json:"order_number"`
		Reason      string `json:"reason"`
		Items       []struct {
			ProductID   int32  `json:"product_id"`
			ProductUUID string `json:"product_uuid"`
			Quantity    int32  `json:"quantity"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("decode order.cancelled payload: %w", err)
	}
	if len(event.Items) == 0 {
		return nil
	}

	refID := event.OrderUUID
	if refID == "" {
		refID = event.OrderNumber
	}
	if refID == "" {
		return fmt.Errorf("order.cancelled event carries neither order_uuid nor order_number")
	}

	items := make([]inventorydto.RestoreItem, 0, len(event.Items))
	for _, item := range event.Items {
		items = append(items, inventorydto.RestoreItem{
			ProductID:   item.ProductID,
			ProductUUID: item.ProductUUID,
			Quantity:    item.Quantity,
		})
	}

	reason := event.Reason
	if reason == "" {
		reason = "order_cancelled"
	}

	if err := c.inventoryService.RestoreStockWithRef(ctx, items, refID, "order_cancel", nil, nil, &reason); err != nil {
		return fmt.Errorf("restore stock for order %s: %w", refID, err)
	}
	return nil
}
