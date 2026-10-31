package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	inventorydto "goshop/services/inventory/internal/dto"
	inventoryservice "goshop/services/inventory/internal/service"
)

// ReturnCompletedConsumer restocks returned items once a return is completed.
// Inventory owns this state transition (branch F).
type ReturnCompletedConsumer struct {
	inventoryService inventoryservice.InventoryService
}

func NewReturnCompletedConsumer(inventoryService inventoryservice.InventoryService) *ReturnCompletedConsumer {
	return &ReturnCompletedConsumer{inventoryService: inventoryService}
}

// Apply restocks the items the customer returned to the warehouse.
func (c *ReturnCompletedConsumer) Apply(ctx context.Context, body []byte) error {
	var event struct {
		EventID     string `json:"event_id"`
		ReturnUUID  string `json:"return_uuid"`
		OrderUUID   string `json:"order_uuid"`
		OrderNumber string `json:"order_number"`
		Items       []struct {
			ProductID   int32  `json:"product_id"`
			ProductUUID string `json:"product_uuid"`
			Quantity    int32  `json:"quantity"`
			Restock     bool   `json:"restock"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("decode order_return.completed payload: %w", err)
	}

	items := make([]inventorydto.RestoreItem, 0, len(event.Items))
	for _, item := range event.Items {
		if !item.Restock || item.Quantity <= 0 {
			continue
		}
		items = append(items, inventorydto.RestoreItem{
			ProductID:   item.ProductID,
			ProductUUID: item.ProductUUID,
			Quantity:    item.Quantity,
		})
	}
	if len(items) == 0 {
		return nil
	}

	refID := event.ReturnUUID
	if refID == "" {
		refID = event.EventID
	}
	if refID == "" {
		refID = event.OrderNumber
	}
	if refID == "" {
		return fmt.Errorf("order_return.completed event carries no return_uuid, event_id or order_number")
	}

	note := fmt.Sprintf("Restock from completed return (Order #%s, Return UUID %s)", event.OrderNumber, event.ReturnUUID)

	if err := c.inventoryService.RestoreStockWithRef(ctx, items, refID, "return", nil, nil, &note); err != nil {
		return fmt.Errorf("restock returned items for return %s: %w", refID, err)
	}
	return nil
}
