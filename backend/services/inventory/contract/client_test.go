package contract_test

import (
	"context"
	"net"
	"testing"

	inventoryv1 "goshop/proto/inventory/v1"
	"goshop/services/inventory/contract"
	"goshop/services/inventory/internal/dto"
	inventorygrpc "goshop/services/inventory/internal/grpc"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type mockInventoryService struct {
	deductFunc  func(ctx context.Context, items []dto.DeductItem, refID string, refType string, actorID *int32, actorUUID *string, note *string) (map[string]dto.ItemStockDTO, error)
	restoreFunc func(ctx context.Context, items []dto.RestoreItem, refID string, refType string, actorID *int32, actorUUID *string, note *string) error
	getStockFn  func(ctx context.Context, productUUIDs []string, productIDs []int32) ([]dto.ItemStockDTO, error)
	adjustFn    func(ctx context.Context, actorUUID *uuid.UUID, actorName *string, input dto.AdjustStockInput) (*dto.StockMovementDTO, error)
	listMoveFn  func(ctx context.Context, params dto.GetStockMovementsParam) ([]dto.StockMovementDTO, int64, error)
}

func (m *mockInventoryService) LockAndDeductStockWithRef(ctx context.Context, items []dto.DeductItem, refID string, refType string, actorID *int32, actorUUID *string, note *string) (map[string]dto.ItemStockDTO, error) {
	if m.deductFunc != nil {
		return m.deductFunc(ctx, items, refID, refType, actorID, actorUUID, note)
	}
	res := make(map[string]dto.ItemStockDTO)
	for _, it := range items {
		res[it.ProductUUID] = dto.ItemStockDTO{
			ProductUUID:       it.ProductUUID,
			Quantity:          50,
			AvailableQuantity: 50,
		}
	}
	return res, nil
}

func (m *mockInventoryService) RestoreStockWithRef(ctx context.Context, items []dto.RestoreItem, refID string, refType string, actorID *int32, actorUUID *string, note *string) error {
	if m.restoreFunc != nil {
		return m.restoreFunc(ctx, items, refID, refType, actorID, actorUUID, note)
	}
	return nil
}

func (m *mockInventoryService) GetStock(ctx context.Context, productUUIDs []string, productIDs []int32) ([]dto.ItemStockDTO, error) {
	if m.getStockFn != nil {
		return m.getStockFn(ctx, productUUIDs, productIDs)
	}
	res := make([]dto.ItemStockDTO, 0)
	for _, u := range productUUIDs {
		res = append(res, dto.ItemStockDTO{
			ProductUUID:       u,
			Quantity:          100,
			AvailableQuantity: 100,
		})
	}
	return res, nil
}

func (m *mockInventoryService) AdjustStock(ctx context.Context, actorUUID *uuid.UUID, actorName *string, input dto.AdjustStockInput) (*dto.StockMovementDTO, error) {
	if m.adjustFn != nil {
		return m.adjustFn(ctx, actorUUID, actorName, input)
	}
	return &dto.StockMovementDTO{
		MovementUUID:   uuid.NewString(),
		ProductUUID:    input.ProductUUID,
		QuantityChange: input.QuantityChange,
		StockBefore:    10,
		StockAfter:     10 + input.QuantityChange,
	}, nil
}

func (m *mockInventoryService) ListStockMovements(ctx context.Context, params dto.GetStockMovementsParam) ([]dto.StockMovementDTO, int64, error) {
	if m.listMoveFn != nil {
		return m.listMoveFn(ctx, params)
	}
	return nil, 0, nil
}

func setupTestServer(t *testing.T, svc *mockInventoryService) (contract.InventoryClient, func()) {
	lis := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	inventoryv1.RegisterInventoryServiceServer(server, inventorygrpc.NewInventoryGRPCServer(svc))

	go func() {
		_ = server.Serve(lis)
	}()

	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	client := contract.NewGRPCInventoryClientWithConn(conn)

	cleanup := func() {
		_ = client.Close()
		server.GracefulStop()
		_ = lis.Close()
	}

	return client, cleanup
}

func TestInventoryContract_ReserveStock(t *testing.T) {
	mockSvc := &mockInventoryService{}
	client, cleanup := setupTestServer(t, mockSvc)
	defer cleanup()

	targetUUID := uuid.NewString()

	resp, err := client.ReserveStock(context.Background(), contract.ReserveStockInput{
		Items: []contract.StockItem{
			{ProductUUID: targetUUID, Quantity: 2},
		},
		ReferenceID:   "order-123",
		ReferenceType: "order",
	})

	require.NoError(t, err)
	assert.Len(t, resp, 1)
	assert.Equal(t, targetUUID, resp[0].ProductUUID)
}

func TestInventoryContract_ReleaseStock(t *testing.T) {
	mockSvc := &mockInventoryService{}
	client, cleanup := setupTestServer(t, mockSvc)
	defer cleanup()

	err := client.ReleaseStock(context.Background(), contract.ReleaseStockInput{
		Items: []contract.StockItem{
			{ProductUUID: uuid.NewString(), Quantity: 2},
		},
		ReferenceID:   "order-123",
		ReferenceType: "order",
	})

	require.NoError(t, err)
}

func TestInventoryContract_GetStock(t *testing.T) {
	mockSvc := &mockInventoryService{}
	client, cleanup := setupTestServer(t, mockSvc)
	defer cleanup()

	targetUUID := uuid.NewString()
	resp, err := client.GetStock(context.Background(), []string{targetUUID}, nil)

	require.NoError(t, err)
	assert.Len(t, resp, 1)
	assert.Equal(t, targetUUID, resp[0].ProductUUID)
	assert.Equal(t, int32(100), resp[0].Quantity)
}
