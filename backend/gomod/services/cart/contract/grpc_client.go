package contract

import (
	"context"
	"fmt"

	"goshop/shared/pkg/grpcclient"
	cartv1 "goshop/shared/proto/cart/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type CartItemInfo struct {
	ItemUUID        string
	ProductUUID     string
	ProductID       int32
	ProductName     string
	ProductSlug     string
	ProductImageURL string
	Price           float64
	Quantity        int32
	Subtotal        float64
	StockAvailable  int32
	IsAvailable     bool
}

type CartInfo struct {
	CartUUID    string
	UserUUID    string
	UserID      int32
	TotalItems  int32
	TotalAmount float64
	Items       []CartItemInfo
}

type CartClient interface {
	GetCart(ctx context.Context, userUUID string, userID int32) (*CartInfo, error)
	AddItem(ctx context.Context, userUUID string, userID int32, productUUID string, quantity int32) (*CartInfo, error)
	UpdateItemQuantity(ctx context.Context, userUUID string, userID int32, itemUUID string, quantity int32) (*CartInfo, error)
	RemoveItem(ctx context.Context, userUUID string, userID int32, itemUUID string) (*CartInfo, error)
	ClearCart(ctx context.Context, userUUID string, userID int32) (bool, error)
	RemovePurchasedItems(ctx context.Context, userUUID string, userID int32, productUUIDs []string, productIDs []int32) (bool, error)
	Close() error
}

type GRPCCartClient struct {
	client cartv1.CartServiceClient
	conn   *grpc.ClientConn
}

func NewGRPCCartClient(target string) (*GRPCCartClient, error) {
	conn, err := grpcclient.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create grpc client for cart at %s: %w", target, err)
	}

	return NewGRPCCartClientWithConn(conn), nil
}

func NewGRPCCartClientWithConn(conn *grpc.ClientConn) *GRPCCartClient {
	return &GRPCCartClient{
		client: cartv1.NewCartServiceClient(conn),
		conn:   conn,
	}
}

func (c *GRPCCartClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func mapProtoToCartInfo(cart *cartv1.CartInfo) *CartInfo {
	if cart == nil {
		return nil
	}
	items := make([]CartItemInfo, len(cart.Items))
	for i, it := range cart.Items {
		items[i] = CartItemInfo{
			ItemUUID:        it.ItemUuid,
			ProductUUID:     it.ProductUuid,
			ProductID:       it.ProductId,
			ProductName:     it.ProductName,
			ProductSlug:     it.ProductSlug,
			ProductImageURL: it.ProductImageUrl,
			Price:           it.Price,
			Quantity:        it.Quantity,
			Subtotal:        it.Subtotal,
			StockAvailable:  it.StockAvailable,
			IsAvailable:     it.IsAvailable,
		}
	}

	return &CartInfo{
		CartUUID:    cart.CartUuid,
		UserUUID:    cart.UserUuid,
		UserID:      cart.UserId,
		TotalItems:  cart.TotalItems,
		TotalAmount: cart.TotalAmount,
		Items:       items,
	}
}

func (c *GRPCCartClient) GetCart(ctx context.Context, userUUID string, userID int32) (*CartInfo, error) {
	resp, err := c.client.GetCart(ctx, &cartv1.GetCartRequest{
		UserUuid: userUUID,
		UserId:   userID,
	})
	if err != nil {
		return nil, err
	}
	return mapProtoToCartInfo(resp.Cart), nil
}

func (c *GRPCCartClient) AddItem(ctx context.Context, userUUID string, userID int32, productUUID string, quantity int32) (*CartInfo, error) {
	resp, err := c.client.AddItem(ctx, &cartv1.AddItemRequest{
		UserUuid:    userUUID,
		UserId:      userID,
		ProductUuid: productUUID,
		Quantity:    quantity,
	})
	if err != nil {
		return nil, err
	}
	return mapProtoToCartInfo(resp.Cart), nil
}

func (c *GRPCCartClient) UpdateItemQuantity(ctx context.Context, userUUID string, userID int32, itemUUID string, quantity int32) (*CartInfo, error) {
	resp, err := c.client.UpdateItemQuantity(ctx, &cartv1.UpdateItemQuantityRequest{
		UserUuid: userUUID,
		UserId:   userID,
		ItemUuid: itemUUID,
		Quantity: quantity,
	})
	if err != nil {
		return nil, err
	}
	return mapProtoToCartInfo(resp.Cart), nil
}

func (c *GRPCCartClient) RemoveItem(ctx context.Context, userUUID string, userID int32, itemUUID string) (*CartInfo, error) {
	resp, err := c.client.RemoveItem(ctx, &cartv1.RemoveItemRequest{
		UserUuid: userUUID,
		UserId:   userID,
		ItemUuid: itemUUID,
	})
	if err != nil {
		return nil, err
	}
	return mapProtoToCartInfo(resp.Cart), nil
}

func (c *GRPCCartClient) ClearCart(ctx context.Context, userUUID string, userID int32) (bool, error) {
	resp, err := c.client.ClearCart(ctx, &cartv1.ClearCartRequest{
		UserUuid: userUUID,
		UserId:   userID,
	})
	if err != nil {
		return false, err
	}
	return resp.Success, nil
}

func (c *GRPCCartClient) RemovePurchasedItems(ctx context.Context, userUUID string, userID int32, productUUIDs []string, productIDs []int32) (bool, error) {
	resp, err := c.client.RemovePurchasedItems(ctx, &cartv1.RemovePurchasedItemsRequest{
		UserUuid:     userUUID,
		UserId:       userID,
		ProductUuids: productUUIDs,
		ProductIds:   productIDs,
	})
	if err != nil {
		return false, err
	}
	return resp.Success, nil
}
