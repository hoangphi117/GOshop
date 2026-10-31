package consumer_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	inventoryconsumer "goshop/services/inventory/internal/consumer"
	inventorydto "goshop/services/inventory/internal/dto"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type restoreCall struct {
	items   []inventorydto.RestoreItem
	refID   string
	refType string
}

type fakeInventoryService struct {
	calls []restoreCall
	err   error
}

func (f *fakeInventoryService) LockAndDeductStockWithRef(context.Context, []inventorydto.DeductItem, string, string, *int32, *string, *string) (map[string]inventorydto.ItemStockDTO, error) {
	return nil, nil
}
func (f *fakeInventoryService) RestoreStockWithRef(_ context.Context, items []inventorydto.RestoreItem, refID string, refType string, _ *int32, _ *string, _ *string) error {
	f.calls = append(f.calls, restoreCall{items: items, refID: refID, refType: refType})
	return f.err
}
func (f *fakeInventoryService) GetStock(context.Context, []string, []int32) ([]inventorydto.ItemStockDTO, error) {
	return nil, nil
}
func (f *fakeInventoryService) AdjustStock(context.Context, *uuid.UUID, *string, inventorydto.AdjustStockInput) (*inventorydto.StockMovementDTO, error) {
	return nil, nil
}
func (f *fakeInventoryService) ListStockMovements(context.Context, inventorydto.GetStockMovementsParam) ([]inventorydto.StockMovementDTO, int64, error) {
	return nil, 0, nil
}

func TestOrderCancelledConsumer_RestoresStock(t *testing.T) {
	svc := &fakeInventoryService{}
	consumer := inventoryconsumer.NewOrderCancelledConsumer(svc)

	orderUUID := uuid.New()
	body, err := json.Marshal(map[string]any{
		"event_id":     "evt-inv-1",
		"order_uuid":   orderUUID.String(),
		"order_number": "ORD-INV",
		"reason":       "customer request",
		"items": []map[string]any{
			{"product_id": 4, "quantity": 2},
		},
	})
	require.NoError(t, err)

	require.NoError(t, consumer.Apply(context.Background(), body))
	require.Len(t, svc.calls, 1)
	assert.Equal(t, orderUUID.String(), svc.calls[0].refID)
	assert.Equal(t, "order_cancel", svc.calls[0].refType)
	assert.Equal(t, []inventorydto.RestoreItem{{ProductID: 4, Quantity: 2}}, svc.calls[0].items)
}

// Restore failure must surface so the event stays retryable.
func TestOrderCancelledConsumer_SurfacesFailure(t *testing.T) {
	svc := &fakeInventoryService{err: errors.New("inventory unavailable")}
	consumer := inventoryconsumer.NewOrderCancelledConsumer(svc)

	body, err := json.Marshal(map[string]any{
		"event_id":   "evt-inv-2",
		"order_uuid": uuid.New().String(),
		"items":      []map[string]any{{"product_id": 4, "quantity": 1}},
	})
	require.NoError(t, err)

	assert.ErrorContains(t, consumer.Apply(context.Background(), body), "inventory unavailable")
}

// A cancelled order without items touches no stock.
func TestOrderCancelledConsumer_NoItems_Noop(t *testing.T) {
	svc := &fakeInventoryService{}
	consumer := inventoryconsumer.NewOrderCancelledConsumer(svc)

	body, err := json.Marshal(map[string]any{"event_id": "evt-inv-empty", "order_uuid": uuid.New().String()})
	require.NoError(t, err)

	require.NoError(t, consumer.Apply(context.Background(), body))
	assert.Empty(t, svc.calls)
}

func TestReturnCompletedConsumer_RestocksOnlyReturnedItems(t *testing.T) {
	svc := &fakeInventoryService{}
	consumer := inventoryconsumer.NewReturnCompletedConsumer(svc)

	returnUUID := uuid.New()
	body, err := json.Marshal(map[string]any{
		"event_id":     "evt-ret-1",
		"return_uuid":  returnUUID.String(),
		"order_uuid":   uuid.New().String(),
		"order_number": "ORD-RET",
		"items": []map[string]any{
			{"product_id": 10, "quantity": 3, "restock": true},
			{"product_id": 11, "quantity": 1, "restock": false},
			{"product_id": 12, "quantity": 0, "restock": true},
		},
	})
	require.NoError(t, err)

	require.NoError(t, consumer.Apply(context.Background(), body))
	require.Len(t, svc.calls, 1)
	assert.Equal(t, "return", svc.calls[0].refType)
	assert.Equal(t, returnUUID.String(), svc.calls[0].refID)
	assert.Equal(t, []inventorydto.RestoreItem{{ProductID: 10, Quantity: 3}}, svc.calls[0].items)
}

func TestReturnCompletedConsumer_NoRestockableItems_Noop(t *testing.T) {
	svc := &fakeInventoryService{}
	consumer := inventoryconsumer.NewReturnCompletedConsumer(svc)

	body, err := json.Marshal(map[string]any{
		"event_id":    "evt-ret-none",
		"return_uuid": uuid.New().String(),
		"items":       []map[string]any{{"product_id": 10, "quantity": 1, "restock": false}},
	})
	require.NoError(t, err)

	require.NoError(t, consumer.Apply(context.Background(), body))
	assert.Empty(t, svc.calls)
}

func TestReturnCompletedConsumer_SurfacesFailure(t *testing.T) {
	svc := &fakeInventoryService{err: errors.New("deadlock detected")}
	consumer := inventoryconsumer.NewReturnCompletedConsumer(svc)

	body, err := json.Marshal(map[string]any{
		"event_id":    "evt-ret-err",
		"return_uuid": uuid.New().String(),
		"items":       []map[string]any{{"product_id": 10, "quantity": 1, "restock": true}},
	})
	require.NoError(t, err)

	assert.ErrorContains(t, consumer.Apply(context.Background(), body), "deadlock detected")
}

func (f *fakeInventoryService) ListLowStockProducts(context.Context, int32, int32) ([]inventorydto.ItemStockDTO, int64, error) {
	return nil, 0, nil
}
