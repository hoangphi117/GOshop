package contract

import (
	"context"
	"fmt"

	"goshop/pkg/grpcclient"
	inventoryv1 "goshop/proto/inventory/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type StockItem struct {
	ProductID   int32
	ProductUUID string
	Quantity    int32
}

type StockItemInfo struct {
	ProductID         int32
	ProductUUID       string
	ProductName       string
	ProductSKU        string
	ProductImageURL   string
	Quantity          int32
	ReservedQuantity  int32
	AvailableQuantity int32
}

type ReserveStockInput struct {
	Items         []StockItem
	ReferenceID   string
	ReferenceType string
	ActorID       *int32
	ActorUUID     *string
	Note          *string
}

type ReleaseStockInput struct {
	Items         []StockItem
	ReferenceID   string
	ReferenceType string
	ActorID       *int32
	ActorUUID     *string
	Note          *string
}

type AdjustStockInput struct {
	ProductUUID    string
	ProductID      int32
	QuantityChange int32
	ChangeType     string
	Note           *string
	ActorID        *int32
	ActorUUID      *string
	ActorName      *string
}

type AdjustStockResult struct {
	Success      bool
	CurrentStock int32
	MovementUUID string
	StockInfo    *StockItemInfo
}

type InventoryClient interface {
	ReserveStock(ctx context.Context, input ReserveStockInput) ([]StockItemInfo, error)
	ReleaseStock(ctx context.Context, input ReleaseStockInput) error
	GetStock(ctx context.Context, productUUIDs []string, productIDs []int32) ([]StockItemInfo, error)
	AdjustStock(ctx context.Context, input AdjustStockInput) (*AdjustStockResult, error)
	Close() error
}

type GRPCInventoryClient struct {
	client inventoryv1.InventoryServiceClient
	conn   *grpc.ClientConn
}

func NewGRPCInventoryClient(target string) (*GRPCInventoryClient, error) {
	conn, err := grpcclient.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create grpc client for inventory at %s: %w", target, err)
	}

	return NewGRPCInventoryClientWithConn(conn), nil
}

func NewGRPCInventoryClientWithConn(conn *grpc.ClientConn) *GRPCInventoryClient {
	return &GRPCInventoryClient{
		client: inventoryv1.NewInventoryServiceClient(conn),
		conn:   conn,
	}
}

func (c *GRPCInventoryClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *GRPCInventoryClient) ReserveStock(ctx context.Context, input ReserveStockInput) ([]StockItemInfo, error) {
	protoItems := make([]*inventoryv1.StockItem, len(input.Items))
	for i, it := range input.Items {
		protoItems[i] = &inventoryv1.StockItem{
			ProductId:   it.ProductID,
			ProductUuid: it.ProductUUID,
			Quantity:    it.Quantity,
		}
	}

	var actorID int32
	if input.ActorID != nil {
		actorID = *input.ActorID
	}
	var actorUUID string
	if input.ActorUUID != nil {
		actorUUID = *input.ActorUUID
	}
	var note string
	if input.Note != nil {
		note = *input.Note
	}

	resp, err := c.client.ReserveStock(ctx, &inventoryv1.ReserveStockRequest{
		Items:         protoItems,
		ReferenceId:   input.ReferenceID,
		ReferenceType: input.ReferenceType,
		ActorId:       actorID,
		ActorUuid:     actorUUID,
		Note:          note,
	})
	if err != nil {
		return nil, fmt.Errorf("grpc ReserveStock failed: %w", err)
	}

	results := make([]StockItemInfo, len(resp.Items))
	for i, it := range resp.Items {
		results[i] = StockItemInfo{
			ProductID:         it.ProductId,
			ProductUUID:       it.ProductUuid,
			ProductName:       it.ProductName,
			ProductSKU:        it.ProductSku,
			ProductImageURL:   it.ProductImageUrl,
			Quantity:          it.Quantity,
			ReservedQuantity:  it.ReservedQuantity,
			AvailableQuantity: it.AvailableQuantity,
		}
	}

	return results, nil
}

func (c *GRPCInventoryClient) ReleaseStock(ctx context.Context, input ReleaseStockInput) error {
	protoItems := make([]*inventoryv1.StockItem, len(input.Items))
	for i, it := range input.Items {
		protoItems[i] = &inventoryv1.StockItem{
			ProductId:   it.ProductID,
			ProductUuid: it.ProductUUID,
			Quantity:    it.Quantity,
		}
	}

	var actorID int32
	if input.ActorID != nil {
		actorID = *input.ActorID
	}
	var actorUUID string
	if input.ActorUUID != nil {
		actorUUID = *input.ActorUUID
	}
	var note string
	if input.Note != nil {
		note = *input.Note
	}

	_, err := c.client.ReleaseStock(ctx, &inventoryv1.ReleaseStockRequest{
		Items:         protoItems,
		ReferenceId:   input.ReferenceID,
		ReferenceType: input.ReferenceType,
		ActorId:       actorID,
		ActorUuid:     actorUUID,
		Note:          note,
	})
	if err != nil {
		return fmt.Errorf("grpc ReleaseStock failed: %w", err)
	}
	return nil
}

func (c *GRPCInventoryClient) GetStock(ctx context.Context, productUUIDs []string, productIDs []int32) ([]StockItemInfo, error) {
	resp, err := c.client.GetStock(ctx, &inventoryv1.GetStockRequest{
		ProductUuids: productUUIDs,
		ProductIds:   productIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("grpc GetStock failed: %w", err)
	}

	results := make([]StockItemInfo, len(resp.Items))
	for i, it := range resp.Items {
		results[i] = StockItemInfo{
			ProductID:         it.ProductId,
			ProductUUID:       it.ProductUuid,
			ProductName:       it.ProductName,
			ProductSKU:        it.ProductSku,
			ProductImageURL:   it.ProductImageUrl,
			Quantity:          it.Quantity,
			ReservedQuantity:  it.ReservedQuantity,
			AvailableQuantity: it.AvailableQuantity,
		}
	}
	return results, nil
}

func (c *GRPCInventoryClient) AdjustStock(ctx context.Context, input AdjustStockInput) (*AdjustStockResult, error) {
	var actorID int32
	if input.ActorID != nil {
		actorID = *input.ActorID
	}
	var actorUUID string
	if input.ActorUUID != nil {
		actorUUID = *input.ActorUUID
	}
	var actorName string
	if input.ActorName != nil {
		actorName = *input.ActorName
	}
	var note string
	if input.Note != nil {
		note = *input.Note
	}

	resp, err := c.client.AdjustStock(ctx, &inventoryv1.AdjustStockRequest{
		ProductUuid:    input.ProductUUID,
		ProductId:      input.ProductID,
		QuantityChange: input.QuantityChange,
		ChangeType:     input.ChangeType,
		Note:           note,
		ActorId:        actorID,
		ActorUuid:      actorUUID,
		ActorName:      actorName,
	})
	if err != nil {
		return nil, fmt.Errorf("grpc AdjustStock failed: %w", err)
	}

	var stockInfo *StockItemInfo
	if resp.StockInfo != nil {
		stockInfo = &StockItemInfo{
			ProductID:         resp.StockInfo.ProductId,
			ProductUUID:       resp.StockInfo.ProductUuid,
			ProductName:       resp.StockInfo.ProductName,
			ProductSKU:        resp.StockInfo.ProductSku,
			ProductImageURL:   resp.StockInfo.ProductImageUrl,
			Quantity:          resp.StockInfo.Quantity,
			ReservedQuantity:  resp.StockInfo.ReservedQuantity,
			AvailableQuantity: resp.StockInfo.AvailableQuantity,
		}
	}

	return &AdjustStockResult{
		Success:      resp.Success,
		CurrentStock: resp.CurrentStock,
		MovementUUID: resp.MovementUuid,
		StockInfo:    stockInfo,
	}, nil
}
