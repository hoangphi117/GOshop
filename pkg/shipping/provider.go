package shipping

import (
	"context"
	"time"
)

type CreateOrderRequest struct {
	ToName                string
	ToPhone               string
	ToAddress             string
	ToDistrictID          int
	ToWardCode            string
	Weight                int // gram
	Length, Width, Height int // cm
	Items                 []ShippingItem
	RequiredNote          string
	Note                  string
	CODAmount             float64
	CallbackURL           string // URL ngrok khi dev / domain khi prod
}

type ShippingItem struct {
	Name     string
	Quantity int
	Price    float64
	Weight   int
}

type CreateOrderResponse struct {
	ShippingCode     string
	ShippingFee      float64
	ExpectedDelivery *time.Time
}

type TrackingInfo struct {
	Status     string
	StatusText string
	Location   string
	UpdatedAt  *time.Time
}

type ShippingProvider interface {
	Name() string
	CreateShippingOrder(ctx context.Context, req CreateOrderRequest) (*CreateOrderResponse, error)
	GetTrackingInfo(ctx context.Context, shippingCode string) (*TrackingInfo, error)
	CancelShippingOrder(ctx context.Context, shippingCode string) error
	CalculateFee(ctx context.Context, req CreateOrderRequest) (float64, error)
}
