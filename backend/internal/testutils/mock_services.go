package testutils

import (
	"context"
	"time"

	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Ensure mocks implement service interfaces at compile time
var (
	_ v1service.OrderPricingService = (*MockOrderPricingService)(nil)
	_ v1service.InventoryService    = (*MockInventoryService)(nil)
	_ v1service.NotificationService = (*MockNotificationService)(nil)
	_ v1service.CartService         = (*MockCartService)(nil)
)

type MockOrderPricingService struct {
	CalculateItemsFunc               func(items []v1service.ItemPriceInput) ([]v1service.ItemPriceResult, float64, error)
	ValidateAndCalculateDiscountFunc func(coupon *sqlc.Coupon, subtotal float64, now time.Time) (float64, error)
	CalculateOrderTotalsFunc         func(ctx context.Context, items []v1service.ItemPriceInput, couponCode *string, userID int32, shippingFee float64, now time.Time) (*v1service.OrderPricingResult, error)
}

func (m *MockOrderPricingService) CalculateItems(items []v1service.ItemPriceInput) ([]v1service.ItemPriceResult, float64, error) {
	if m.CalculateItemsFunc != nil {
		return m.CalculateItemsFunc(items)
	}
	return nil, 0, nil
}

func (m *MockOrderPricingService) ValidateAndCalculateDiscount(coupon *sqlc.Coupon, subtotal float64, now time.Time) (float64, error) {
	if m.ValidateAndCalculateDiscountFunc != nil {
		return m.ValidateAndCalculateDiscountFunc(coupon, subtotal, now)
	}
	return 0, nil
}

func (m *MockOrderPricingService) CalculateOrderTotals(ctx context.Context, items []v1service.ItemPriceInput, couponCode *string, userID int32, shippingFee float64, now time.Time) (*v1service.OrderPricingResult, error) {
	if m.CalculateOrderTotalsFunc != nil {
		return m.CalculateOrderTotalsFunc(ctx, items, couponCode, userID, shippingFee, now)
	}
	return &v1service.OrderPricingResult{}, nil
}

type MockInventoryService struct {
	LockAndDeductStockFunc func(ctx context.Context, items []v1service.DeductItem) (map[string]sqlc.GetByUUIDsForUpdateRow, error)
	RestoreStockFunc       func(ctx context.Context, items []v1service.RestoreItem) error
}

func (m *MockInventoryService) LockAndDeductStock(ctx context.Context, items []v1service.DeductItem) (map[string]sqlc.GetByUUIDsForUpdateRow, error) {
	if m.LockAndDeductStockFunc != nil {
		return m.LockAndDeductStockFunc(ctx, items)
	}
	return nil, nil
}

func (m *MockInventoryService) RestoreStock(ctx context.Context, items []v1service.RestoreItem) error {
	if m.RestoreStockFunc != nil {
		return m.RestoreStockFunc(ctx, items)
	}
	return nil
}

type MockNotificationService struct {
	SendFunc                       func(ctx context.Context, input v1dto.SendNotificationInput) error
	CreateNotificationFunc         func(ctx *gin.Context, input v1dto.CreateNotificationInput) (v1dto.NotificationDTO, error)
	BroadcastNotificationFunc      func(ctx *gin.Context, input v1dto.BroadcastNotificationInput) error
	GetMyNotificationsFunc         func(ctx *gin.Context, query v1dto.GetNotificationsQuery) ([]v1dto.NotificationDTO, int32, error)
	GetUnreadNotificationCountFunc func(ctx *gin.Context) (v1dto.UnreadCountDTO, error)
	MarkNotificationAsReadFunc     func(ctx *gin.Context, notificationUUID uuid.UUID) (v1dto.NotificationDTO, error)
	MarkAllNotificationsAsReadFunc func(ctx *gin.Context) error
	DeleteNotificationFunc         func(ctx *gin.Context, notificationUUID uuid.UUID) error
}

func (m *MockNotificationService) Send(ctx context.Context, input v1dto.SendNotificationInput) error {
	if m.SendFunc != nil {
		return m.SendFunc(ctx, input)
	}
	return nil
}

func (m *MockNotificationService) CreateNotification(ctx *gin.Context, input v1dto.CreateNotificationInput) (v1dto.NotificationDTO, error) {
	if m.CreateNotificationFunc != nil {
		return m.CreateNotificationFunc(ctx, input)
	}
	return v1dto.NotificationDTO{}, nil
}

func (m *MockNotificationService) BroadcastNotification(ctx *gin.Context, input v1dto.BroadcastNotificationInput) error {
	if m.BroadcastNotificationFunc != nil {
		return m.BroadcastNotificationFunc(ctx, input)
	}
	return nil
}

func (m *MockNotificationService) GetMyNotifications(ctx *gin.Context, query v1dto.GetNotificationsQuery) ([]v1dto.NotificationDTO, int32, error) {
	if m.GetMyNotificationsFunc != nil {
		return m.GetMyNotificationsFunc(ctx, query)
	}
	return nil, 0, nil
}

func (m *MockNotificationService) GetUnreadNotificationCount(ctx *gin.Context) (v1dto.UnreadCountDTO, error) {
	if m.GetUnreadNotificationCountFunc != nil {
		return m.GetUnreadNotificationCountFunc(ctx)
	}
	return v1dto.UnreadCountDTO{}, nil
}

func (m *MockNotificationService) MarkNotificationAsRead(ctx *gin.Context, notificationUUID uuid.UUID) (v1dto.NotificationDTO, error) {
	if m.MarkNotificationAsReadFunc != nil {
		return m.MarkNotificationAsReadFunc(ctx, notificationUUID)
	}
	return v1dto.NotificationDTO{}, nil
}

func (m *MockNotificationService) MarkAllNotificationsAsRead(ctx *gin.Context) error {
	if m.MarkAllNotificationsAsReadFunc != nil {
		return m.MarkAllNotificationsAsReadFunc(ctx)
	}
	return nil
}

func (m *MockNotificationService) DeleteNotification(ctx *gin.Context, notificationUUID uuid.UUID) error {
	if m.DeleteNotificationFunc != nil {
		return m.DeleteNotificationFunc(ctx, notificationUUID)
	}
	return nil
}

type MockCartService struct {
	GetMyCartFunc              func(ctx *gin.Context) (v1dto.CartDTO, error)
	AddItemToCartFunc          func(ctx *gin.Context, input v1dto.AddCartItemInput) (v1dto.CartDTO, error)
	UpdateCartItemQuantityFunc func(ctx *gin.Context, itemUUID uuid.UUID, input v1dto.UpdateCartItemQuantityInput) (v1dto.CartDTO, error)
	RemoveItemFromCartFunc     func(ctx *gin.Context, itemUUID uuid.UUID) (v1dto.CartDTO, error)
	ClearMyCartFunc            func(ctx *gin.Context) error
}

func (m *MockCartService) GetMyCart(ctx *gin.Context) (v1dto.CartDTO, error) {
	if m.GetMyCartFunc != nil {
		return m.GetMyCartFunc(ctx)
	}
	return v1dto.CartDTO{}, nil
}

func (m *MockCartService) AddItemToCart(ctx *gin.Context, input v1dto.AddCartItemInput) (v1dto.CartDTO, error) {
	if m.AddItemToCartFunc != nil {
		return m.AddItemToCartFunc(ctx, input)
	}
	return v1dto.CartDTO{}, nil
}

func (m *MockCartService) UpdateCartItemQuantity(ctx *gin.Context, itemUUID uuid.UUID, input v1dto.UpdateCartItemQuantityInput) (v1dto.CartDTO, error) {
	if m.UpdateCartItemQuantityFunc != nil {
		return m.UpdateCartItemQuantityFunc(ctx, itemUUID, input)
	}
	return v1dto.CartDTO{}, nil
}

func (m *MockCartService) RemoveItemFromCart(ctx *gin.Context, itemUUID uuid.UUID) (v1dto.CartDTO, error) {
	if m.RemoveItemFromCartFunc != nil {
		return m.RemoveItemFromCartFunc(ctx, itemUUID)
	}
	return v1dto.CartDTO{}, nil
}

func (m *MockCartService) ClearMyCart(ctx *gin.Context) error {
	if m.ClearMyCartFunc != nil {
		return m.ClearMyCartFunc(ctx)
	}
	return nil
}
