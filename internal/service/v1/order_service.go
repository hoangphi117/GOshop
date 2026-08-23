package v1service

import (
	"context"
	"fmt"
	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/cache"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type orderService struct {
	orderRepo     repository.OrderRepository
	orderItemRepo repository.OrderItemRepository
	productRepo   repository.ProductRepository
	userRepo      repository.UserRepository
	cache         cache.RedisCacheService
}

func NewOrderService(orderRepo repository.OrderRepository, orderItemRepo repository.OrderItemRepository, productRepo repository.ProductRepository, userRepo repository.UserRepository, redisClient *redis.Client) OrderService {
	return &orderService{
		orderRepo:     orderRepo,
		orderItemRepo: orderItemRepo,
		productRepo:   productRepo,
		userRepo:      userRepo,
		cache:         cache.NewRedisCacheService(redisClient),
	}
}

type preparedItem struct {
	productID   int32
	productName string
	price       float64
	quantity    int32
	subtotal    float64
}

func (s *orderService) CreateOrder(ctx *gin.Context, input v1dto.CreateOrderInput) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error) {

	context := ctx.Request.Context()

	userUUIDStr, ok := ctx.Get("user_uuid")

	if !ok || userUUIDStr == "" {
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized")
	}

	userUUID, err := uuid.Parse(userUUIDStr.(string))
	if err != nil {
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeUnauthorized, "Invalid user UUID format")
	}

	user, err := s.userRepo.GetByUUID(context, userUUID)
	if err != nil {
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeNotFound, "User not found")
	}

	productMap := make(map[string]int32)
	for _, item := range input.Items {
		if _, exists := productMap[item.ProductUUID]; exists {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Duplicate product in order")
		}
		productMap[item.ProductUUID] = item.Quantity
	}

	tx, err := db.BeginTx(ctx)
	if err != nil {
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "Failed to start transaction", err)
	}
	defer tx.Rollback(ctx)

	var totalAmount float64
	preparedItems := make([]preparedItem, 0, len(input.Items))

	for _, item := range input.Items {
		productUUID, err := uuid.Parse(item.ProductUUID)
		if err != nil {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Invalid product UUID format: "+item.ProductUUID)
		}

		product, err := s.productRepo.GetByUUID(context, productUUID)
		if err != nil {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeNotFound, "Product not found: "+item.ProductUUID)
		}

		if product.ProductStatus != 1 {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Product is unavailable: "+product.ProductName)
		}

		err = s.productRepo.DeductStock(context, tx, product.ProductID, item.Quantity)
		if err != nil {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Insufficient stock for product: "+product.ProductName)
		}

		subtotal := float64(item.Quantity) * utils.NumericToFloat(product.ProductPrice)
		totalAmount += subtotal

		preparedItems = append(preparedItems, preparedItem{
			productID:   product.ProductID,
			productName: product.ProductName,
			price:       utils.NumericToFloat(product.ProductPrice),
			quantity:    item.Quantity,
			subtotal:    subtotal,
		})
	}

	orderNumber, err := s.generateOrderNumber(context)
	if err != nil {
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeInternal, "Failed to generate order number")
	}

	createOrderParams := sqlc.CreateOrderParams{
		OrderNumber:          orderNumber,
		OrderUserID:          user.UserID,
		OrderShippingName:    input.ShippingName,
		OrderShippingPhone:   input.ShippingPhone,
		OrderShippingAddress: input.ShippingAddress,
		OrderTotalAmount:     utils.FloatToNumeric(totalAmount),
		OrderPaymentMethod:   input.PaymentMethod,
		OrderPaymentStatus:   1,
		OrderNote:            &input.Note,
		OrderStatus:          1,
	}

	order, err := s.orderRepo.Create(context, tx, createOrderParams)
	if err != nil {
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "failed to create order", err)
	}

	for _, item := range preparedItems {
		createdItemParams := sqlc.CreateOrderItemParams{
			OrderItemOrderID:     order.OrderID,
			OrderItemProductID:   item.productID,
			OrderItemProductName: item.productName,
			OrderItemPrice:       utils.FloatToNumeric(item.price),
			OrderItemQuantity:    item.quantity,
			OrderItemSubtotal:    utils.FloatToNumeric(item.subtotal),
		}

		_, err := s.orderItemRepo.Create(context, tx, createdItemParams)
		if err != nil {
			return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "failed to create order item", err)
		}
	}

	if err := tx.Commit(context); err != nil {
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeInternal, "failed to commit transaction")
	}

	orderItemsWithDetails, err := s.orderItemRepo.GetByOrderID(context, order.OrderID)
	if err != nil {
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "failed to get order items", err)
	}

	return order, orderItemsWithDetails, nil
}

func (s *orderService) generateOrderNumber(ctx context.Context) (string, error) {

	prefix := fmt.Sprintf("ORD-%s-", time.Now().Format("20060102"))

	lastOrderNumber, err := s.orderRepo.GetLastOrderNumber(ctx, prefix+"%")
	if err != nil || lastOrderNumber == "" {
		return prefix + "0001", nil
	}

	parts := strings.Split(lastOrderNumber, "-")
	if len(parts) < 3 {
		return prefix + "0001", nil
	}

	lastSeq, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return prefix + "0001", nil
	}

	return fmt.Sprintf("%s%04d", prefix, lastSeq+1), nil
}
