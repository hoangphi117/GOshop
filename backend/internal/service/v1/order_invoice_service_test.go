package v1service_test

import (
	"context"
	"testing"
	"time"

	"goshop/internal/db/sqlc"
	v1service "goshop/internal/service/v1"
	"goshop/internal/testutils"
	"goshop/internal/utils"

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

	baseUser := sqlc.User{UserID: 1, UserUuid: userUUID, UserLevel: v1service.RoleMember}
	adminUser := sqlc.User{UserID: 99, UserUuid: adminUUID, UserLevel: v1service.RoleAdmin}
	otherUser := sqlc.User{UserID: 2, UserUuid: otherUserUUID, UserLevel: v1service.RoleMember}

	paidOrder := sqlc.GetOrderByUUIDRow{
		OrderID:              100,
		OrderUuid:            orderUUID,
		OrderUserID:          baseUser.UserID,
		OrderNumber:          "ORD-100",
		OrderStatus:          v1service.OrderStatusConfirmed,
		OrderPaymentStatus:   v1service.OrderPaymentStatusPaid,
		OrderPaymentMethod:   v1service.OrderPaymentMethodCOD,
		OrderShippingName:    "Nguyen Van A",
		OrderShippingPhone:   "0987654321",
		OrderShippingAddress: "123 Le Loi, HCMC",
		OrderSubtotalAmount:  utils.FloatToNumeric(500000),
		OrderDiscountAmount:  utils.FloatToNumeric(0),
		OrderTotalAmount:     utils.FloatToNumeric(500000),
		OrderCreatedAt:       time.Now(),
	}

	t.Run("Case 1: User Not Found", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewOrderInvoiceService(nil, nil, v1service.NewOrderCustomerAdapter(mockUserRepo), nil)
		_, _, err := svc.GetOrderInvoicePDF(ctx, userUUID, orderUUID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "User not found")
	})

	t.Run("Case 2: Order Not Found", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewOrderInvoiceService(mockOrderRepo, nil, v1service.NewOrderCustomerAdapter(mockUserRepo), nil)
		_, _, err := svc.GetOrderInvoicePDF(ctx, userUUID, orderUUID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Order not found")
	})

	t.Run("Case 3: Unpaid Order -> BadRequest", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		unpaidOrder := paidOrder
		unpaidOrder.OrderPaymentStatus = v1service.OrderPaymentStatusUnpaid

		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return unpaidOrder, nil
			},
		}

		svc := v1service.NewOrderInvoiceService(mockOrderRepo, nil, v1service.NewOrderCustomerAdapter(mockUserRepo), nil)
		_, _, err := svc.GetOrderInvoicePDF(ctx, userUUID, orderUUID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Invoice is not available until payment is confirmed")
	})

	t.Run("Case 4: Non-owner Member -> Forbidden", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return otherUser, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return paidOrder, nil
			},
		}

		svc := v1service.NewOrderInvoiceService(mockOrderRepo, nil, v1service.NewOrderCustomerAdapter(mockUserRepo), nil)
		_, _, err := svc.GetOrderInvoicePDF(ctx, otherUserUUID, orderUUID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Access denied")
	})

	t.Run("Case 5: Success -> PDF Generated", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return baseUser, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return paidOrder, nil
			},
		}
		mockItemRepo := &testutils.MockOrderItemRepository{
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

		svc := v1service.NewOrderInvoiceService(mockOrderRepo, mockItemRepo, v1service.NewOrderCustomerAdapter(mockUserRepo), nil)
		pdfBytes, filename, err := svc.GetOrderInvoicePDF(ctx, userUUID, orderUUID)
		require.NoError(t, err)
		assert.NotEmpty(t, pdfBytes)
		assert.Equal(t, "Invoice_ORD-100.pdf", filename)
	})

	t.Run("Case 6: Admin Access Allowed for Other User's Order", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return adminUser, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return paidOrder, nil
			},
		}
		mockItemRepo := &testutils.MockOrderItemRepository{
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

		svc := v1service.NewOrderInvoiceService(mockOrderRepo, mockItemRepo, v1service.NewOrderCustomerAdapter(mockUserRepo), nil)
		pdfBytes, filename, err := svc.GetOrderInvoicePDF(ctx, adminUUID, orderUUID)
		require.NoError(t, err)
		assert.NotEmpty(t, pdfBytes)
		assert.Equal(t, "Invoice_ORD-100.pdf", filename)
	})
}
