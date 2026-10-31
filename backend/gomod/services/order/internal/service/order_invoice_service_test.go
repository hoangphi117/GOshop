package service_test

import (
	"context"
	"testing"
	"time"

	"goshop/services/order/internal/db/sqlc"
	orderservice "goshop/services/order/internal/service"
	"goshop/shared/pkg/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrderInvoiceService_GetOrderInvoicePDF(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	adminUUID := uuid.New()
	otherUserUUID := uuid.New()
	orderUUID := uuid.New()

	baseCustomer := orderservice.NewOrderCustomer(1, userUUID, "John Doe", "john@example.com", orderservice.RoleMember, 1)
	adminCustomer := orderservice.NewOrderCustomer(99, adminUUID, "Admin", "admin@example.com", orderservice.RoleAdmin, 1)
	otherCustomer := orderservice.NewOrderCustomer(2, otherUserUUID, "Other", "other@example.com", orderservice.RoleMember, 1)

	paidOrder := sqlc.GetOrderByUUIDRow{
		OrderID:              100,
		OrderUuid:            orderUUID,
		OrderUserID:          baseCustomer.UserID,
		OrderNumber:          "ORD-100",
		OrderStatus:          orderservice.OrderStatusConfirmed,
		OrderPaymentStatus:   orderservice.OrderPaymentStatusPaid,
		OrderPaymentMethod:   orderservice.OrderPaymentMethodCOD,
		OrderShippingName:    "Nguyen Van A",
		OrderShippingPhone:   "0987654321",
		OrderShippingAddress: "123 Le Loi, HCMC",
		OrderSubtotalAmount:  utils.FloatToNumeric(500000),
		OrderDiscountAmount:  utils.FloatToNumeric(0),
		OrderTotalAmount:     utils.FloatToNumeric(500000),
		OrderCreatedAt:       time.Now(),
	}

	t.Run("Case 1: User Not Found", func(t *testing.T) {
		mockCustomerReader := &MockCustomerReader{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (*orderservice.OrderCustomer, error) {
				return nil, pgx.ErrNoRows
			},
		}

		svc := orderservice.NewOrderInvoiceService(nil, nil, mockCustomerReader, nil)
		_, _, err := svc.GetOrderInvoicePDF(ctx, userUUID, orderUUID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "User not found")
	})

	t.Run("Case 2: Order Not Found", func(t *testing.T) {
		mockCustomerReader := &MockCustomerReader{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (*orderservice.OrderCustomer, error) {
				return baseCustomer, nil
			},
		}
		mockOrderRepo := &MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{}, pgx.ErrNoRows
			},
		}

		svc := orderservice.NewOrderInvoiceService(mockOrderRepo, nil, mockCustomerReader, nil)
		_, _, err := svc.GetOrderInvoicePDF(ctx, userUUID, orderUUID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Order not found")
	})

	t.Run("Case 3: Unpaid Order -> BadRequest", func(t *testing.T) {
		mockCustomerReader := &MockCustomerReader{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (*orderservice.OrderCustomer, error) {
				return baseCustomer, nil
			},
		}
		unpaidOrder := paidOrder
		unpaidOrder.OrderPaymentStatus = orderservice.OrderPaymentStatusUnpaid

		mockOrderRepo := &MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return unpaidOrder, nil
			},
		}

		svc := orderservice.NewOrderInvoiceService(mockOrderRepo, nil, mockCustomerReader, nil)
		_, _, err := svc.GetOrderInvoicePDF(ctx, userUUID, orderUUID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Invoice is not available until payment is confirmed")
	})

	t.Run("Case 4: Non-owner Member -> Forbidden", func(t *testing.T) {
		mockCustomerReader := &MockCustomerReader{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (*orderservice.OrderCustomer, error) {
				return otherCustomer, nil
			},
		}
		mockOrderRepo := &MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return paidOrder, nil
			},
		}

		svc := orderservice.NewOrderInvoiceService(mockOrderRepo, nil, mockCustomerReader, nil)
		_, _, err := svc.GetOrderInvoicePDF(ctx, otherUserUUID, orderUUID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Access denied")
	})

	t.Run("Case 5: Success -> PDF Generated", func(t *testing.T) {
		mockCustomerReader := &MockCustomerReader{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (*orderservice.OrderCustomer, error) {
				return baseCustomer, nil
			},
		}
		mockOrderRepo := &MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return paidOrder, nil
			},
		}
		mockItemRepo := &MockOrderItemRepository{
			GetByOrderIDFunc: func(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error) {
				return []sqlc.GetOrderItemsByOrderIDRow{
					{
						OrderItemProductName: "iPhone 15",
						OrderItemQuantity:    1,
						OrderItemPrice:       utils.FloatToNumeric(500000),
						OrderItemSubtotal:    utils.FloatToNumeric(500000),
					},
				}, nil
			},
		}

		svc := orderservice.NewOrderInvoiceService(mockOrderRepo, mockItemRepo, mockCustomerReader, nil)
		pdfBytes, filename, err := svc.GetOrderInvoicePDF(ctx, userUUID, orderUUID)
		require.NoError(t, err)
		assert.NotEmpty(t, pdfBytes)
		assert.Equal(t, "Invoice_ORD-100.pdf", filename)
	})

	t.Run("Case 6: Admin Access Allowed for Other User's Order", func(t *testing.T) {
		mockCustomerReader := &MockCustomerReader{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (*orderservice.OrderCustomer, error) {
				return adminCustomer, nil
			},
		}
		mockOrderRepo := &MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return paidOrder, nil
			},
		}
		mockItemRepo := &MockOrderItemRepository{
			GetByOrderIDFunc: func(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error) {
				return []sqlc.GetOrderItemsByOrderIDRow{
					{
						OrderItemProductName: "iPhone 15",
						OrderItemQuantity:    1,
						OrderItemPrice:       utils.FloatToNumeric(500000),
						OrderItemSubtotal:    utils.FloatToNumeric(500000),
					},
				}, nil
			},
		}

		svc := orderservice.NewOrderInvoiceService(mockOrderRepo, mockItemRepo, mockCustomerReader, nil)
		pdfBytes, filename, err := svc.GetOrderInvoicePDF(ctx, adminUUID, orderUUID)
		require.NoError(t, err)
		assert.NotEmpty(t, pdfBytes)
		assert.Equal(t, "Invoice_ORD-100.pdf", filename)
	})
}
