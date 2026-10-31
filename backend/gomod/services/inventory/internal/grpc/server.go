package grpc

import (
	"context"
	"errors"

	"goshop/services/inventory/internal/dto"
	"goshop/services/inventory/internal/service"
	"goshop/shared/pkg/utils"
	inventoryv1 "goshop/shared/proto/inventory/v1"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// referenceIDOrNil maps an empty proto string to a nil reference pointer.
func referenceIDOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// grpcCodeFromError preserves the caller's error class across the gRPC boundary
// instead of collapsing every failure to InvalidArgument.
func grpcCodeFromError(err error) codes.Code {
	var appErr *utils.AppError
	if errors.As(err, &appErr) {
		switch appErr.Code {
		case utils.ErrCodeServiceUnavailable:
			return codes.Unavailable
		case utils.ErrCodeConflict:
			return codes.AlreadyExists
		case utils.ErrCodeNotFound:
			return codes.NotFound
		case utils.ErrCodeBadRequest:
			return codes.InvalidArgument
		}
	}
	return codes.InvalidArgument
}

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
			Quantity:          st.Quantity,
			ReservedQuantity:  st.ReservedQuantity,
			AvailableQuantity: st.AvailableQuantity,
		}
	}

	return &inventoryv1.GetStockResponse{
		Items: items,
	}, nil
}

// ListLowStockProducts serves analytics low-stock alerts from the inventory
// database (available = quantity - reserved), analytics never reads it.
func (s *InventoryGRPCServer) ListLowStockProducts(ctx context.Context, req *inventoryv1.ListLowStockProductsRequest) (*inventoryv1.ListLowStockProductsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	stocks, total, err := s.inventoryService.ListLowStockProducts(ctx, req.Threshold, req.Limit)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list low stock products: %v", err)
	}

	items := make([]*inventoryv1.ItemStockInfo, len(stocks))
	for i, st := range stocks {
		items[i] = &inventoryv1.ItemStockInfo{
			ProductId:         st.ProductID,
			ProductUuid:       st.ProductUUID,
			AvailableQuantity: st.AvailableQuantity,
		}
	}

	return &inventoryv1.ListLowStockProductsResponse{
		Items:      items,
		TotalCount: total,
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
	var targetQty *int32
	if req.TargetQuantity != nil {
		t := req.GetTargetQuantity()
		targetQty = &t
	}
	var productID *int32
	if req.ProductId > 0 {
		productID = &req.ProductId
	}

	movement, err := s.inventoryService.AdjustStock(ctx, actorUUID, actorName, dto.AdjustStockInput{
		ProductUUID:    req.ProductUuid,
		ProductID:      productID,
		QuantityChange: req.QuantityChange,
		ChangeType:     req.ChangeType,
		Note:           note,
		ReferenceID:    referenceIDOrNil(req.ReferenceId),
		TargetQuantity: targetQty,
	})
	if err != nil {
		return nil, status.Errorf(grpcCodeFromError(err), "failed to adjust stock: %v", err)
	}

	pID := int32(0)
	if movement.ProductID != nil {
		pID = *movement.ProductID
	}

	return &inventoryv1.AdjustStockResponse{
		Success:      true,
		CurrentStock: movement.StockAfter,
		MovementUuid: movement.MovementUUID,
		StockInfo: &inventoryv1.ItemStockInfo{
			ProductId:         pID,
			ProductUuid:       movement.ProductUUID,
			Quantity:          movement.StockAfter,
			ReservedQuantity:  0,
			AvailableQuantity: movement.StockAfter,
		},
	}, nil
}
