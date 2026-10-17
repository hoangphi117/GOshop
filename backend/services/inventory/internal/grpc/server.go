package grpc

import (
	"context"

	inventoryv1 "goshop/proto/inventory/v1"
	"goshop/services/inventory/internal/dto"
	"goshop/services/inventory/internal/service"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type InventoryGRPCServer struct {
	inventoryv1.UnimplementedInventoryServiceServer
	inventoryService service.InventoryService
}

func NewInventoryGRPCServer(inventoryService service.InventoryService) *InventoryGRPCServer {
	return &InventoryGRPCServer{
		inventoryService: inventoryService,
	}
}

func (s *InventoryGRPCServer) ReserveStock(ctx context.Context, req *inventoryv1.ReserveStockRequest) (*inventoryv1.ReserveStockResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	items := make([]dto.DeductItem, len(req.Items))
	for i, it := range req.Items {
		items[i] = dto.DeductItem{
			ProductUUID: it.ProductUuid,
			Quantity:    it.Quantity,
		}
	}

	var actorID *int32
	if req.ActorId > 0 {
		actorID = &req.ActorId
	}
	var actorUUID *string
	if req.ActorUuid != "" {
		actorUUID = &req.ActorUuid
	}
	var note *string
	if req.Note != "" {
		note = &req.Note
	}

	stockMap, err := s.inventoryService.LockAndDeductStockWithRef(
		ctx,
		items,
		req.ReferenceId,
		req.ReferenceType,
		actorID,
		actorUUID,
		note,
	)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "failed to reserve stock: %v", err)
	}

	resItems := make([]*inventoryv1.ItemStockInfo, 0, len(stockMap))
	for _, it := range stockMap {
		resItems = append(resItems, &inventoryv1.ItemStockInfo{
			ProductId:         it.ProductID,
			ProductUuid:       it.ProductUUID,
			ProductName:       it.ProductName,
			ProductSku:        it.ProductSKU,
			ProductImageUrl:   it.ProductImageURL,
			Quantity:          it.Quantity,
			ReservedQuantity:  it.ReservedQuantity,
			AvailableQuantity: it.AvailableQuantity,
		})
	}

	return &inventoryv1.ReserveStockResponse{
		Success: true,
		Items:   resItems,
	}, nil
}

func (s *InventoryGRPCServer) ReleaseStock(ctx context.Context, req *inventoryv1.ReleaseStockRequest) (*inventoryv1.ReleaseStockResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	items := make([]dto.RestoreItem, len(req.Items))
	for i, it := range req.Items {
		items[i] = dto.RestoreItem{
			ProductID:   it.ProductId,
			ProductUUID: it.ProductUuid,
			Quantity:    it.Quantity,
		}
	}

	var actorID *int32
	if req.ActorId > 0 {
		actorID = &req.ActorId
	}
	var actorUUID *string
	if req.ActorUuid != "" {
		actorUUID = &req.ActorUuid
	}
	var note *string
	if req.Note != "" {
		note = &req.Note
	}

	err := s.inventoryService.RestoreStockWithRef(
		ctx,
		items,
		req.ReferenceId,
		req.ReferenceType,
		actorID,
		actorUUID,
		note,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to release stock: %v", err)
	}

	return &inventoryv1.ReleaseStockResponse{
		Success: true,
	}, nil
}

func (s *InventoryGRPCServer) GetStock(ctx context.Context, req *inventoryv1.GetStockRequest) (*inventoryv1.GetStockResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	stocks, err := s.inventoryService.GetStock(ctx, req.ProductUuids, req.ProductIds)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get stock: %v", err)
	}

	items := make([]*inventoryv1.ItemStockInfo, len(stocks))
	for i, st := range stocks {
		items[i] = &inventoryv1.ItemStockInfo{
			ProductId:         st.ProductID,
			ProductUuid:       st.ProductUUID,
			ProductName:       st.ProductName,
			ProductSku:        st.ProductSKU,
			ProductImageUrl:   st.ProductImageURL,
			Quantity:          st.Quantity,
			ReservedQuantity:  st.ReservedQuantity,
			AvailableQuantity: st.AvailableQuantity,
		}
	}

	return &inventoryv1.GetStockResponse{
		Items: items,
	}, nil
}

func (s *InventoryGRPCServer) AdjustStock(ctx context.Context, req *inventoryv1.AdjustStockRequest) (*inventoryv1.AdjustStockResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	var actorUUID *uuid.UUID
	if req.ActorUuid != "" {
		if u, err := uuid.Parse(req.ActorUuid); err == nil {
			actorUUID = &u
		}
	}
	var actorName *string
	if req.ActorName != "" {
		actorName = &req.ActorName
	}
	var note *string
	if req.Note != "" {
		note = &req.Note
	}

	movement, err := s.inventoryService.AdjustStock(ctx, actorUUID, actorName, dto.AdjustStockInput{
		ProductUUID:    req.ProductUuid,
		QuantityChange: req.QuantityChange,
		ChangeType:     req.ChangeType,
		Note:           note,
	})
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to adjust stock: %v", err)
	}

	pID := int32(0)
	if movement.ProductID != nil {
		pID = *movement.ProductID
	}
	sku := ""
	if movement.ProductSKU != nil {
		sku = *movement.ProductSKU
	}
	img := ""
	if movement.ProductImageURL != nil {
		img = *movement.ProductImageURL
	}

	return &inventoryv1.AdjustStockResponse{
		Success:      true,
		CurrentStock: movement.StockAfter,
		MovementUuid: movement.MovementUUID,
		StockInfo: &inventoryv1.ItemStockInfo{
			ProductId:         pID,
			ProductUuid:       movement.ProductUUID,
			ProductName:       movement.ProductName,
			ProductSku:        sku,
			ProductImageUrl:   img,
			Quantity:          movement.StockAfter,
			ReservedQuantity:  0,
			AvailableQuantity: movement.StockAfter,
		},
	}, nil
}
