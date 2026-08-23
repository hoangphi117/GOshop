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
	"goshop/pkg/logger"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	OrderStatusPending        int32 = 1
	OrderPaymentStatusPending int32 = 1
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

	reqCtx := ctx.Request.Context()

	// Get user UUID from context
	userUUIDStr, ok := ctx.Get("user_uuid")

	if !ok || userUUIDStr == "" {
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized")
	}

	userUUID, err := uuid.Parse(userUUIDStr.(string))
	if err != nil {
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeUnauthorized, "Invalid user UUID format")
	}

	user, err := s.userRepo.GetByUUID(reqCtx, userUUID)
	if err != nil {
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeNotFound, "User not found")
	}

	// Validate duplicate products in the order
	productMapInput := make(map[string]int32)
	for _, item := range input.Items {
		if _, exists := productMapInput[item.ProductUUID]; exists {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Duplicate product in order")
		}
		productMapInput[item.ProductUUID] = item.Quantity
	}

	// Deterministic Lock Ordering to prevent deadlock
	sort.Slice(input.Items, func(i, j int) bool {
		return input.Items[i].ProductUUID < input.Items[j].ProductUUID
	})

	uuids := make([]uuid.UUID, len(input.Items))
	for i, item := range input.Items {
		parsed, err := uuid.Parse(item.ProductUUID)
		if err != nil {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Invalid product UUID format: "+item.ProductUUID)
		}
		uuids[i] = parsed
	}

	// TRANSACTION START
	txCtx, cancel := context.WithTimeout(reqCtx, 10*time.Second)
	defer cancel()

	tx, err := db.BeginTx(txCtx)
	if err != nil {
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "Failed to start transaction", err)
	}
	defer tx.Rollback(txCtx)

	// Fetch and lock products for update ordered by UUID to prevent deadlocks
	products, err := s.productRepo.GetByUUIDs(txCtx, tx, uuids)
	if err != nil {
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "Failed to fetch & lock products", err)
	}

	if len(products) != len(input.Items) {
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeNotFound, "One or more products not found")
	}

	// Move locked products into a map for quick access
	productMap := make(map[string]sqlc.GetByUUIDsForUpdateRow)
	for _, p := range products {
		productMap[p.ProductUuid.String()] = p
	}

	var totalAmount float64
	preparedItems := make([]preparedItem, 0, len(input.Items))

	// Loop through the input items to validate (status, quantity) and prepare order items
	for _, item := range input.Items {

		if item.Quantity <= 0 {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Quantity must be greater than 0 for product: "+item.ProductUUID)
		}

		product, ok := productMap[item.ProductUUID]
		if !ok {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeNotFound, "Product not found: "+item.ProductUUID)
		}

		// Read 100% fresh product status and stock from the locked row
		if product.ProductStatus != ProductStatusActive {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Product is unavailable: "+product.ProductName)
		}

		if utils.Int32Value(product.ProductStock) < item.Quantity {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Insufficient stock for product: "+product.ProductName)
		}

		err = s.productRepo.DeductStock(txCtx, tx, product.ProductID, item.Quantity)
		if err != nil {
			return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeBadRequest, "Insufficient stock for product: "+product.ProductName)
		}

		subtotal := utils.RoundFloat(float64(item.Quantity)*utils.NumericToFloat(product.ProductPrice), 2)
		totalAmount = utils.RoundFloat(totalAmount+subtotal, 2)

		preparedItems = append(preparedItems, preparedItem{
			productID:   product.ProductID,
			productName: product.ProductName,
			price:       utils.NumericToFloat(product.ProductPrice),
			quantity:    item.Quantity,
			subtotal:    subtotal,
		})
	}

	orderNumber, err := s.generateOrderNumber()
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
		OrderPaymentStatus:   OrderPaymentStatusPending,
		OrderNote:            &input.Note,
		OrderStatus:          OrderStatusPending,
	}

	order, err := s.orderRepo.Create(txCtx, tx, createOrderParams)
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

		_, err := s.orderItemRepo.Create(txCtx, tx, createdItemParams)
		if err != nil {
			return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "failed to create order item", err)
		}
	}

	if err := tx.Commit(txCtx); err != nil {
		return sqlc.Order{}, nil, utils.NewError(utils.ErrCodeInternal, "failed to commit transaction")
	}

	orderItemsWithDetails, err := s.orderItemRepo.GetByOrderID(txCtx, order.OrderID)
	if err != nil {
		return sqlc.Order{}, nil, utils.WrapError(utils.ErrCodeInternal, "failed to get order items", err)
	}

	if err := s.cache.Clear("products:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear product cache")
	}
	if err := s.cache.Clear("orders:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear order cache")
	}

	return order, orderItemsWithDetails, nil
}

func (s *orderService) generateOrderNumber() (string, error) {
	today := time.Now().Format("20060102")
	redisKey := fmt.Sprintf("order_seq:%s", today)

	seq, err := s.cache.Incr(redisKey)
	if err != nil {
		return "", err
	}

	// First order of the day or Redis was restarted/cleared
	if seq == 1 {
		prefix := fmt.Sprintf("ORD-%s-", today)
		lastOrderNumber, err := s.orderRepo.GetLastOrderNumber(context.Background(), prefix+"%")
		if err == nil && lastOrderNumber != "" {
			parts := strings.Split(lastOrderNumber, "-")
			if len(parts) >= 3 {
				if lastSeq, err := strconv.Atoi(parts[len(parts)-1]); err == nil && lastSeq >= 1 {
					seq = int64(lastSeq + 1)
					_ = s.cache.Set(redisKey, seq, 24*time.Hour)
				}
			}
		}
		_, _ = s.cache.Expire(redisKey, 24*time.Hour)
	}

	return fmt.Sprintf("ORD-%s-%04d", today, seq), nil
}
