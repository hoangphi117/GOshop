package consumer_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	couponconsumer "goshop/services/coupon/internal/consumer"
	coupondto "goshop/services/coupon/internal/dto"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCouponService struct {
	releaseInputs []coupondto.ReleaseCouponInput
	err           error
}

func (f *fakeCouponService) PreviewCoupon(context.Context, coupondto.PreviewCouponInput) (*coupondto.PreviewCouponDTO, error) {
	return nil, nil
}
func (f *fakeCouponService) ReserveCoupon(context.Context, coupondto.ReserveCouponInput) (string, error) {
	return "", nil
}
func (f *fakeCouponService) ReleaseCoupon(_ context.Context, input coupondto.ReleaseCouponInput) error {
	f.releaseInputs = append(f.releaseInputs, input)
	return f.err
}
func (f *fakeCouponService) GetByCode(context.Context, string) (*coupondto.CouponDTO, error) {
	return nil, nil
}
func (f *fakeCouponService) GetByUUID(context.Context, uuid.UUID) (*coupondto.CouponDTO, error) {
	return nil, nil
}
func (f *fakeCouponService) List(context.Context, *string, *int32, *bool, int32, int32) ([]coupondto.CouponDTO, int64, error) {
	return nil, 0, nil
}
func (f *fakeCouponService) Create(context.Context, coupondto.CreateCouponInput) (*coupondto.CouponDTO, error) {
	return nil, nil
}
func (f *fakeCouponService) Update(context.Context, uuid.UUID, coupondto.UpdateCouponInput) (*coupondto.CouponDTO, error) {
	return nil, nil
}
func (f *fakeCouponService) Delete(context.Context, uuid.UUID) error { return nil }

func cancelledWithCoupon(t *testing.T) (orderUUID uuid.UUID, body []byte) {
	t.Helper()
	orderUUID = uuid.New()
	couponID := int32(9)
	b, err := json.Marshal(map[string]any{
		"event_id":     "evt-coupon-1",
		"order_id":     77,
		"order_uuid":   orderUUID.String(),
		"order_number": "ORD-9",
		"coupon_id":    couponID,
	})
	require.NoError(t, err)
	return orderUUID, b
}

func TestOrderCancelledConsumer_ReleasesCoupon(t *testing.T) {
	svc := &fakeCouponService{}
	consumer := couponconsumer.NewOrderCancelledConsumer(svc)

	orderUUID, body := cancelledWithCoupon(t)
	err := consumer.Apply(context.Background(), body)

	require.NoError(t, err)
	require.Len(t, svc.releaseInputs, 1)
	assert.Equal(t, int32(9), svc.releaseInputs[0].CouponID)
	assert.Equal(t, int32(77), svc.releaseInputs[0].OrderID)
	assert.Equal(t, orderUUID.String(), svc.releaseInputs[0].OrderUUID)
}

// Release failure must surface so the event stays retryable.
func TestOrderCancelledConsumer_SurfacesFailure(t *testing.T) {
	svc := &fakeCouponService{err: errors.New("reservation locked")}
	consumer := couponconsumer.NewOrderCancelledConsumer(svc)

	_, body := cancelledWithCoupon(t)
	err := consumer.Apply(context.Background(), body)

	assert.ErrorContains(t, err, "reservation locked")
}

// An order without coupon_id still releases by order identity: the by-order
// release (branch D) is idempotent and covers reservations whose event omits
// the coupon id.
func TestOrderCancelledConsumer_NoCoupon_ReleasesByOrder(t *testing.T) {
	svc := &fakeCouponService{}
	consumer := couponconsumer.NewOrderCancelledConsumer(svc)

	orderUUID := uuid.New()
	body, err := json.Marshal(map[string]any{
		"event_id":   "evt-coupon-none",
		"order_id":   5,
		"order_uuid": orderUUID.String(),
	})
	require.NoError(t, err)

	require.NoError(t, consumer.Apply(context.Background(), body))
	require.Len(t, svc.releaseInputs, 1)
	assert.Equal(t, orderUUID.String(), svc.releaseInputs[0].OrderUUID)
}
