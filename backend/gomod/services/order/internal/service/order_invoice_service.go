package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"goshop/services/order/internal/repository"
	"goshop/shared/pkg/cache"
	"goshop/shared/pkg/pdf"
	"goshop/shared/pkg/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type CachedInvoicePDF struct {
	Bytes    []byte `json:"bytes"`
	FileName string `json:"file_name"`
}

type orderInvoiceService struct {
	orderRepo     repository.OrderRepository
	orderItemRepo repository.OrderItemRepository
	userRepo      OrderCustomerReader
	cache         cache.RedisCacheService
}

// NewOrderInvoiceService creates a dedicated single-responsibility service for invoice generation and caching.
func NewOrderInvoiceService(
	orderRepo repository.OrderRepository,
	orderItemRepo repository.OrderItemRepository,
	userRepo OrderCustomerReader,
	redisCache cache.RedisCacheService,
) OrderInvoiceService {
	return &orderInvoiceService{
		orderRepo:     orderRepo,
		orderItemRepo: orderItemRepo,
		userRepo:      userRepo,
		cache:         redisCache,
	}
}

func (s *orderInvoiceService) GetOrderInvoicePDF(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) ([]byte, string, error) {
	var user *OrderCustomer
	if cust, ok := GetOrderCustomerFromContext(ctx); ok && cust.UserUUID == userUUID && cust.UserID > 0 {
		user = cust
	} else if s.userRepo != nil {
		var err error
		user, err = s.userRepo.GetByUUID(ctx, userUUID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, "", utils.NewError(utils.ErrCodeNotFound, "User not found")
			}
			return nil, "", utils.WrapError(utils.ErrCodeInternal, "Failed to get user", err)
		}
	} else {
		return nil, "", utils.NewError(utils.ErrCodeNotFound, "User not found")
	}

	order, err := s.orderRepo.GetByUUID(ctx, orderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", utils.NewError(utils.ErrCodeNotFound, "Order not found")
		}
		return nil, "", utils.WrapError(utils.ErrCodeInternal, "Failed to get order", err)
	}

	if order.OrderPaymentStatus == OrderPaymentStatusUnpaid {
		return nil, "", utils.NewError(utils.ErrCodeBadRequest, "Invoice is not available until payment is confirmed")
	}

	if order.OrderUserID != user.UserID && user.UserLevel != RoleAdmin && user.UserLevel != RoleModerator {
		return nil, "", utils.NewError(utils.ErrCodeForbidden, "Access denied")
	}

	// 1. Check Redis Cache
	cacheKey := fmt.Sprintf("invoice:pdf:%s", orderUUID.String())
	var cached CachedInvoicePDF
	if s.cache != nil {
		if err := s.cache.Get(cacheKey, &cached); err == nil && len(cached.Bytes) > 0 {
			return cached.Bytes, cached.FileName, nil
		}
	}

	orderItems, err := s.orderItemRepo.GetByOrderID(ctx, order.OrderID)
	if err != nil {
		return nil, "", utils.WrapError(utils.ErrCodeInternal, "Failed to get order items", err)
	}

	items := make([]pdf.InvoiceItem, len(orderItems))
	for i, item := range orderItems {
		items[i] = pdf.InvoiceItem{
			ProductName: item.OrderItemProductName,
			Quantity:    int(item.OrderItemQuantity),
			UnitPrice:   utils.NumericToFloat(item.OrderItemPrice),
			Subtotal:    utils.NumericToFloat(item.OrderItemSubtotal),
		}
	}

	paymentMethod := "COD (Cash on Delivery)"
	switch order.OrderPaymentMethod {
	case OrderPaymentMethodOnline:
		paymentMethod = "Bank Transfer / Online Payment"
	case OrderPaymentMethodEWallet:
		paymentMethod = "E-Wallet Payment"
	}

	paymentStatus := "UNPAID"
	switch order.OrderPaymentStatus {
	case OrderPaymentStatusPaid:
		paymentStatus = "PAID"
	case OrderPaymentStatusPartiallyRefunded:
		paymentStatus = "PARTIALLY REFUNDED"
	case OrderPaymentStatusFullyRefunded:
		paymentStatus = "FULLY REFUNDED"
	}

	couponCode := ""
	if order.OrderCouponCode != nil {
		couponCode = *order.OrderCouponCode
	}

	invoiceData := pdf.InvoiceData{
		OrderNumber:     order.OrderNumber,
		OrderDate:       order.OrderCreatedAt,
		CustomerName:    order.OrderShippingName,
		CustomerPhone:   order.OrderShippingPhone,
		CustomerAddress: order.OrderShippingAddress,
		PaymentMethod:   paymentMethod,
		PaymentStatus:   paymentStatus,
		Items:           items,
		SubtotalAmount:  utils.NumericToFloat(order.OrderSubtotalAmount),
		DiscountAmount:  utils.NumericToFloat(order.OrderDiscountAmount),
		CouponCode:      couponCode,
		TotalAmount:     utils.NumericToFloat(order.OrderTotalAmount),
	}

	pdfBytes, err := pdf.GenerateInvoicePDF(invoiceData)
	if err != nil {
		return nil, "", utils.WrapError(utils.ErrCodeInternal, "Failed to generate invoice PDF", err)
	}

	fileName := fmt.Sprintf("Invoice_%s.pdf", order.OrderNumber)

	// 2. Save to Redis Cache (TTL 24h)
	if s.cache != nil {
		_ = s.cache.Set(cacheKey, CachedInvoicePDF{
			Bytes:    pdfBytes,
			FileName: fileName,
		}, 24*time.Hour)
	}

	return pdfBytes, fileName, nil
}
