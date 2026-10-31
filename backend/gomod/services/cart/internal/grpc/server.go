package grpc

import (
	"context"

	"goshop/services/cart/internal/dto"
	"goshop/services/cart/internal/service"
	cartv1 "goshop/shared/proto/cart/v1"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CartGRPCServer struct {
	cartv1.UnimplementedCartServiceServer
	cartService service.CartService
}

func NewCartGRPCServer(cartService service.CartService) *CartGRPCServer {
	return &CartGRPCServer{
		cartService: cartService,
	}
}

func mapCartDTOToProto(cart dto.CartDTO, userUUID string, userID int32) *cartv1.CartInfo {
	items := make([]*cartv1.CartItemInfo, len(cart.Items))
	for i, it := range cart.Items {
		items[i] = &cartv1.CartItemInfo{
			ItemUuid:        it.UUID,
			ProductUuid:     it.ProductUUID,
			ProductId:       it.ProductID,
			ProductName:     it.ProductName,
			ProductSlug:     it.ProductSlug,
			ProductImageUrl: it.ProductImage,
			Price:           it.Price,
			Quantity:        it.Quantity,
			Subtotal:        it.Subtotal,
			StockAvailable:  it.StockAvailable,
			IsAvailable:     it.IsAvailable,
		}
	}

	return &cartv1.CartInfo{
		CartUuid:    cart.UUID,
		UserUuid:    userUUID,
		UserId:      userID,
		TotalItems:  cart.TotalItems,
		TotalAmount: cart.TotalAmount,
		Items:       items,
	}
}

func parseUserIdentity(userUUIDStr string, userID int32) (uuid.UUID, *int32, error) {
	var userUUID uuid.UUID
	var err error

	if userUUIDStr != "" {
		userUUID, err = uuid.Parse(userUUIDStr)
		if err != nil {
			return uuid.Nil, nil, status.Error(codes.InvalidArgument, "invalid user_uuid format")
		}
	}

	var uID *int32
	if userID > 0 {
		uID = &userID
	}

	if userUUID == uuid.Nil && uID == nil {
		return uuid.Nil, nil, status.Error(codes.InvalidArgument, "user_uuid or user_id is required")
	}

	return userUUID, uID, nil
}

func (s *CartGRPCServer) GetCart(ctx context.Context, req *cartv1.GetCartRequest) (*cartv1.GetCartResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	userUUID, userID, err := parseUserIdentity(req.UserUuid, req.UserId)
	if err != nil {
		return nil, err
	}

	cart, err := s.cartService.GetMyCart(ctx, userUUID, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get cart: %v", err)
	}

	return &cartv1.GetCartResponse{
		Cart: mapCartDTOToProto(cart, req.UserUuid, req.UserId),
	}, nil
}

func (s *CartGRPCServer) AddItem(ctx context.Context, req *cartv1.AddItemRequest) (*cartv1.AddItemResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	userUUID, userID, err := parseUserIdentity(req.UserUuid, req.UserId)
	if err != nil {
		return nil, err
	}

	cart, err := s.cartService.AddItemToCart(ctx, userUUID, userID, dto.AddCartItemInput{
		ProductUUID: req.ProductUuid,
		Quantity:    req.Quantity,
	})
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to add item: %v", err)
	}

	return &cartv1.AddItemResponse{
		Cart: mapCartDTOToProto(cart, req.UserUuid, req.UserId),
	}, nil
}

func (s *CartGRPCServer) UpdateItemQuantity(ctx context.Context, req *cartv1.UpdateItemQuantityRequest) (*cartv1.UpdateItemQuantityResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	userUUID, userID, err := parseUserIdentity(req.UserUuid, req.UserId)
	if err != nil {
		return nil, err
	}

	itemUUID, err := uuid.Parse(req.ItemUuid)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid item_uuid format")
	}

	qty := req.Quantity
	cart, err := s.cartService.UpdateCartItemQuantity(ctx, userUUID, userID, itemUUID, dto.UpdateCartItemQuantityInput{
		Quantity: &qty,
	})
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to update item quantity: %v", err)
	}

	return &cartv1.UpdateItemQuantityResponse{
		Cart: mapCartDTOToProto(cart, req.UserUuid, req.UserId),
	}, nil
}

func (s *CartGRPCServer) RemoveItem(ctx context.Context, req *cartv1.RemoveItemRequest) (*cartv1.RemoveItemResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	userUUID, userID, err := parseUserIdentity(req.UserUuid, req.UserId)
	if err != nil {
		return nil, err
	}

	itemUUID, err := uuid.Parse(req.ItemUuid)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid item_uuid format")
	}

	cart, err := s.cartService.RemoveItemFromCart(ctx, userUUID, userID, itemUUID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to remove item: %v", err)
	}

	return &cartv1.RemoveItemResponse{
		Cart: mapCartDTOToProto(cart, req.UserUuid, req.UserId),
	}, nil
}

func (s *CartGRPCServer) ClearCart(ctx context.Context, req *cartv1.ClearCartRequest) (*cartv1.ClearCartResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	userUUID, userID, err := parseUserIdentity(req.UserUuid, req.UserId)
	if err != nil {
		return nil, err
	}

	if err := s.cartService.ClearMyCart(ctx, userUUID, userID); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to clear cart: %v", err)
	}

	return &cartv1.ClearCartResponse{
		Success: true,
	}, nil
}

func (s *CartGRPCServer) RemovePurchasedItems(ctx context.Context, req *cartv1.RemovePurchasedItemsRequest) (*cartv1.RemovePurchasedItemsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	var userUUIDPtr *uuid.UUID
	if req.UserUuid != "" {
		if parsed, err := uuid.Parse(req.UserUuid); err == nil {
			userUUIDPtr = &parsed
		}
	}

	var userIDPtr *int32
	if req.UserId > 0 {
		userIDPtr = &req.UserId
	}

	var productUUIDs []uuid.UUID
	for _, uStr := range req.ProductUuids {
		if u, err := uuid.Parse(uStr); err == nil {
			productUUIDs = append(productUUIDs, u)
		}
	}

	if err := s.cartService.RemovePurchasedItems(ctx, userUUIDPtr, userIDPtr, productUUIDs, req.ProductIds); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to remove purchased items: %v", err)
	}

	return &cartv1.RemovePurchasedItemsResponse{
		Success: true,
	}, nil
}
