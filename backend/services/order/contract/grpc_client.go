package contract

import (
	"context"
	"fmt"
	"time"

	orderv1 "goshop/proto/order/v1"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type OrderItemSummary struct {
	ProductUUID string
	ProductName string
	Quantity    int32
	Price       float64
}

type OrderSummary struct {
	OrderID       int32
	OrderUUID     uuid.UUID
	OrderNumber   string
	UserID        int32
	UserUUID      uuid.UUID
	CustomerName  string
	CustomerEmail string
	Status        int32
	TotalAmount   float64
	DeliveredAt   *time.Time
	UpdatedAt     time.Time
	Items         []OrderItemSummary
}

type PurchaseVerification struct {
	OrderID       int32
	OrderUUID     uuid.UUID
	UserID        int32
	IsPurchased   bool
	CanReview     bool
	DeliveredAt   *time.Time
	CustomerName  string
	CustomerEmail string
	ShippingName  string
}

type OrderPaymentInfo struct {
	OrderUUID   uuid.UUID
	UserID      int32
	UserUUID    uuid.UUID
	TotalAmount float64
	Status      int32
}

type OrderClient interface {
	GetOrderSummary(ctx context.Context, orderUUID uuid.UUID) (*OrderSummary, error)
	VerifyPurchase(ctx context.Context, orderUUID uuid.UUID, userID, productID int32, userUUID, productUUID *uuid.UUID) (*PurchaseVerification, error)
	GetOrderForPayment(ctx context.Context, orderUUID uuid.UUID) (*OrderPaymentInfo, error)
	Close() error
}

type orderClient struct {
	client orderv1.OrderServiceClient
	conn   *grpc.ClientConn
}

func NewOrderClient(target string, opts ...grpc.DialOption) (OrderClient, error) {
	if len(opts) == 0 {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to order service at %s: %w", target, err)
	}

	return &orderClient{
		client: orderv1.NewOrderServiceClient(conn),
		conn:   conn,
	}, nil
}

// NewGRPCOrderClient provides consistent naming matching other service contracts.
func NewGRPCOrderClient(target string, opts ...grpc.DialOption) (OrderClient, error) {
	return NewOrderClient(target, opts...)
}

func NewOrderClientFromConn(conn *grpc.ClientConn) OrderClient {
	return &orderClient{
		client: orderv1.NewOrderServiceClient(conn),
		conn:   conn,
	}
}

func (c *orderClient) GetOrderSummary(ctx context.Context, orderUUID uuid.UUID) (*OrderSummary, error) {
	resp, err := c.client.GetOrderSummary(ctx, &orderv1.GetOrderSummaryRequest{
		OrderUuid: orderUUID.String(),
	})
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Order == nil {
		return nil, fmt.Errorf("order summary response is empty")
	}

	o := resp.Order
	parsedOrderUUID, err := uuid.Parse(o.OrderUuid)
	if err != nil {
		parsedOrderUUID = orderUUID
	}
	var parsedUserUUID uuid.UUID
	if o.UserUuid != "" {
		parsedUserUUID, _ = uuid.Parse(o.UserUuid)
	}

	var deliveredAt *time.Time
	if o.DeliveredAt != "" {
		if t, err := time.Parse(time.RFC3339, o.DeliveredAt); err == nil {
			deliveredAt = &t
		}
	}
	var updatedAt time.Time
	if o.UpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339, o.UpdatedAt); err == nil {
			updatedAt = t
		}
	}

	items := make([]OrderItemSummary, 0, len(o.Items))
	for _, it := range o.Items {
		items = append(items, OrderItemSummary{
			ProductUUID: it.ProductUuid,
			ProductName: it.ProductName,
			Quantity:    it.Quantity,
			Price:       it.Price,
		})
	}

	return &OrderSummary{
		OrderID:       o.OrderId,
		OrderUUID:     parsedOrderUUID,
		OrderNumber:   o.OrderNumber,
		UserID:        o.UserId,
		UserUUID:      parsedUserUUID,
		CustomerName:  o.CustomerName,
		CustomerEmail: o.CustomerEmail,
		Status:        o.Status,
		TotalAmount:   o.TotalAmount,
		DeliveredAt:   deliveredAt,
		UpdatedAt:     updatedAt,
		Items:         items,
	}, nil
}

func (c *orderClient) VerifyPurchase(ctx context.Context, orderUUID uuid.UUID, userID, productID int32, userUUID, productUUID *uuid.UUID) (*PurchaseVerification, error) {
	req := &orderv1.VerifyPurchaseRequest{
		OrderUuid: orderUUID.String(),
		UserId:    userID,
		ProductId: productID,
	}
	if userUUID != nil {
		req.UserUuid = userUUID.String()
	}
	if productUUID != nil {
		req.ProductUuid = productUUID.String()
	}

	resp, err := c.client.VerifyPurchase(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("verify purchase response is empty")
	}

	parsedOrderUUID, err := uuid.Parse(resp.OrderUuid)
	if err != nil {
		parsedOrderUUID = orderUUID
	}

	var deliveredAt *time.Time
	if resp.DeliveredAt != "" {
		if t, err := time.Parse(time.RFC3339, resp.DeliveredAt); err == nil {
			deliveredAt = &t
		}
	}

	return &PurchaseVerification{
		OrderID:       resp.OrderId,
		OrderUUID:     parsedOrderUUID,
		UserID:        resp.UserId,
		IsPurchased:   resp.IsPurchased,
		CanReview:     resp.CanReview,
		DeliveredAt:   deliveredAt,
		CustomerName:  resp.CustomerName,
		CustomerEmail: resp.CustomerEmail,
		ShippingName:  resp.ShippingName,
	}, nil
}

func (c *orderClient) GetOrderForPayment(ctx context.Context, orderUUID uuid.UUID) (*OrderPaymentInfo, error) {
	resp, err := c.client.GetOrderForPayment(ctx, &orderv1.GetOrderForPaymentRequest{
		OrderUuid: orderUUID.String(),
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("get order for payment response is empty")
	}

	parsedOrderUUID, err := uuid.Parse(resp.OrderUuid)
	if err != nil {
		parsedOrderUUID = orderUUID
	}
	var parsedUserUUID uuid.UUID
	if resp.UserUuid != "" {
		parsedUserUUID, _ = uuid.Parse(resp.UserUuid)
	}

	return &OrderPaymentInfo{
		OrderUUID:   parsedOrderUUID,
		UserID:      resp.UserId,
		UserUUID:    parsedUserUUID,
		TotalAmount: resp.TotalAmount,
		Status:      resp.Status,
	}, nil
}

func (c *orderClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
