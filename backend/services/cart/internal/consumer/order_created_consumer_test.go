package consumer_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	cartconsumer "goshop/services/cart/internal/consumer"
	cartdto "goshop/services/cart/internal/dto"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCartService struct {
	removedUserUUID *uuid.UUID
	removedUserID   *int32
	removedUUIDs    []uuid.UUID
	removedIDs      []int32
	err             error
	calls           int
}

func (f *fakeCartService) GetMyCart(context.Context, uuid.UUID, *int32) (cartdto.CartDTO, error) {
	return cartdto.CartDTO{}, nil
}
func (f *fakeCartService) AddItemToCart(context.Context, uuid.UUID, *int32, cartdto.AddCartItemInput) (cartdto.CartDTO, error) {
	return cartdto.CartDTO{}, nil
}
func (f *fakeCartService) UpdateCartItemQuantity(context.Context, uuid.UUID, *int32, uuid.UUID, cartdto.UpdateCartItemQuantityInput) (cartdto.CartDTO, error) {
	return cartdto.CartDTO{}, nil
}
func (f *fakeCartService) RemoveItemFromCart(context.Context, uuid.UUID, *int32, uuid.UUID) (cartdto.CartDTO, error) {
	return cartdto.CartDTO{}, nil
}
func (f *fakeCartService) ClearMyCart(context.Context, uuid.UUID, *int32) error { return nil }
func (f *fakeCartService) RemovePurchasedItems(_ context.Context, userUUID *uuid.UUID, userID *int32, productUUIDs []uuid.UUID, productIDs []int32) error {
	f.calls++
	f.removedUserUUID = userUUID
	f.removedUserID = userID
	f.removedUUIDs = productUUIDs
	f.removedIDs = productIDs
	return f.err
}

func orderCreatedBody(t *testing.T, userUUID, productUUID string) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"event_id":             "evt-cart-1",
		"order_uuid":           "11111111-1111-1111-1111-111111111111",
		"order_number":         "ORD-1",
		"user_id":              42,
		"user_uuid":            userUUID,
		"purchased_items":      []int32{7},
		"purchased_item_uuids": []string{productUUID},
	})
	require.NoError(t, err)
	return body
}

func TestOrderCreatedConsumer_RemovesPurchasedProducts(t *testing.T) {
	svc := &fakeCartService{}
	consumer := cartconsumer.NewOrderCreatedConsumer(svc)

	userUUID := uuid.New()
	productUUID := uuid.New()

	err := consumer.Apply(context.Background(), orderCreatedBody(t, userUUID.String(), productUUID.String()))

	require.NoError(t, err)
	assert.Equal(t, 1, svc.calls)
	assert.Equal(t, userUUID, *svc.removedUserUUID)
	assert.Equal(t, int32(42), *svc.removedUserID)
	assert.Equal(t, []uuid.UUID{productUUID}, svc.removedUUIDs)
	assert.Equal(t, []int32{7}, svc.removedIDs)
}

// Side effect failure must surface so the event stays retryable.
func TestOrderCreatedConsumer_SurfacesFailure(t *testing.T) {
	svc := &fakeCartService{err: errors.New("db down")}
	consumer := cartconsumer.NewOrderCreatedConsumer(svc)

	err := consumer.Apply(context.Background(), orderCreatedBody(t, uuid.New().String(), uuid.New().String()))

	assert.ErrorContains(t, err, "db down")
}

// Malformed buyer identity must not be silently skipped.
func TestOrderCreatedConsumer_RejectsInvalidUserUUID(t *testing.T) {
	svc := &fakeCartService{}
	consumer := cartconsumer.NewOrderCreatedConsumer(svc)

	err := consumer.Apply(context.Background(), orderCreatedBody(t, "not-a-uuid", uuid.New().String()))

	assert.ErrorContains(t, err, "parse user_uuid")
	assert.Zero(t, svc.calls)
}

func TestOrderCreatedConsumer_NoPurchasedItems_Noop(t *testing.T) {
	svc := &fakeCartService{}
	consumer := cartconsumer.NewOrderCreatedConsumer(svc)

	body, err := json.Marshal(map[string]any{"event_id": "evt-empty", "user_id": 42})
	require.NoError(t, err)

	require.NoError(t, consumer.Apply(context.Background(), body))
	assert.Zero(t, svc.calls)
}
