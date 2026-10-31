package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	cartservice "goshop/services/cart/internal/service"

	"github.com/google/uuid"
)

// OrderCreatedConsumer removes purchased products from the buyer's cart.
// Cart owns this state transition (branch F), replacing the legacy core-worker
// handler. RemovePurchasedItems deletes by product identity, so redeliveries
// cannot double-apply.
type OrderCreatedConsumer struct {
	cartService cartservice.CartService
}

func NewOrderCreatedConsumer(cartService cartservice.CartService) *OrderCreatedConsumer {
	return &OrderCreatedConsumer{cartService: cartService}
}

// Apply removes every purchased product from the buyer's cart.
func (c *OrderCreatedConsumer) Apply(ctx context.Context, body []byte) error {
	var event struct {
		OrderUUID          string   `json:"order_uuid"`
		OrderNumber        string   `json:"order_number"`
		UserID             int32    `json:"user_id"`
		UserUUID           string   `json:"user_uuid"`
		PurchasedItems     []int32  `json:"purchased_items"`
		PurchasedItemUUIDs []string `json:"purchased_item_uuids"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("decode order.created payload: %w", err)
	}

	if len(event.PurchasedItems) == 0 && len(event.PurchasedItemUUIDs) == 0 {
		return nil
	}

	var userUUID *uuid.UUID
	if event.UserUUID != "" {
		parsed, err := uuid.Parse(event.UserUUID)
		if err != nil {
			return fmt.Errorf("parse user_uuid %q: %w", event.UserUUID, err)
		}
		userUUID = &parsed
	}
	var userID *int32
	if event.UserID > 0 {
		id := event.UserID
		userID = &id
	}
	if userUUID == nil && userID == nil {
		return fmt.Errorf("order %s (%s) carries no buyer identity", event.OrderUUID, event.OrderNumber)
	}

	productUUIDs := make([]uuid.UUID, 0, len(event.PurchasedItemUUIDs))
	for _, raw := range event.PurchasedItemUUIDs {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			return fmt.Errorf("parse purchased product uuid %q: %w", raw, err)
		}
		productUUIDs = append(productUUIDs, parsed)
	}

	if err := c.cartService.RemovePurchasedItems(ctx, userUUID, userID, productUUIDs, event.PurchasedItems); err != nil {
		return fmt.Errorf("clean cart for order %s: %w", event.OrderUUID, err)
	}
	return nil
}
