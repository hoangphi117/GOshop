package v1service

import (
	"errors"
	"fmt"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/payment"
	"goshop/pkg/rabbitmq"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	PaymentStatusPending  int32 = 1
	PaymentStatusPaid     int32 = 2
	PaymentStatusFailed   int32 = 3
	PaymentStatusExpired  int32 = 4
	PaymentStatusRefunded int32 = 5
)

type paymentService struct {
	paymentRepo    repository.PaymentRepository
	orderRepo      repository.OrderRepository
	userRepo       repository.UserRepository
	webhookRepo    repository.WebhookEventRepository
	providers      map[string]payment.PaymentProvider
	cache          cache.RedisCacheService
	rabbitmq       rabbitmq.RabbitMQService
	returnURL      string
	webhookBaseURL string
}

func NewPaymentService(
	paymentRepo repository.PaymentRepository,
	orderRepo repository.OrderRepository,
	userRepo repository.UserRepository,
	webhookRepo repository.WebhookEventRepository,
	providers map[string]payment.PaymentProvider,
	cache cache.RedisCacheService,
	rabbitmq rabbitmq.RabbitMQService,
	returnURL string,
	webhookBaseURL string,
) PaymentService {
	return &paymentService{
		paymentRepo:    paymentRepo,
		orderRepo:      orderRepo,
		userRepo:       userRepo,
		webhookRepo:    webhookRepo,
		providers:      providers,
		cache:          cache,
		rabbitmq:       rabbitmq,
		returnURL:      returnURL,
		webhookBaseURL: webhookBaseURL,
	}
}
func (s *paymentService) CreatePaymentIntent(ctx *gin.Context, input v1dto.CreatePaymentIntentInput) (v1dto.PaymentIntentDTO, error) {

	reqCtx := ctx.Request.Context()
	startTime := time.Now()

	userUuidStr, ok := ctx.Get("user_uuid")
	if !ok || userUuidStr == "" {
		return v1dto.PaymentIntentDTO{}, utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized")
	}

	userUuid, err := uuid.Parse(userUuidStr.(string))
	if err != nil {
		return v1dto.PaymentIntentDTO{}, utils.NewError(utils.ErrCodeUnauthorized, "Invalid user UUID format")
	}

	user, err := s.userRepo.GetByUUID(reqCtx, userUuid)
	if err != nil {
		return v1dto.PaymentIntentDTO{}, utils.NewError(utils.ErrCodeUnauthorized, "User not found")
	}

	order, err := s.orderRepo.GetByUUID(reqCtx, input.OrderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return v1dto.PaymentIntentDTO{}, utils.NewError(utils.ErrCodeNotFound, "Order not found")
		}
		return v1dto.PaymentIntentDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to get order", err)
	}

	// Check if the order belongs to the authenticated user
	if order.OrderUserID != user.UserID {
		return v1dto.PaymentIntentDTO{}, utils.NewError(utils.ErrCodeForbidden, "You are not authorized to pay for this order")
	}

	// Check order status
	if order.OrderPaymentStatus == OrderPaymentStatusPaid {
		return v1dto.PaymentIntentDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Order has already been paid")
	}

	if order.OrderStatus != OrderStatusPending && order.OrderStatus != OrderStatusConfirmed {
		return v1dto.PaymentIntentDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Order status does not allow payment")
	}

	if order.OrderPaymentMethod == OrderPaymentMethodCOD {
		return v1dto.PaymentIntentDTO{}, utils.NewError(utils.ErrCodeBadRequest, "COD orders cannot create online payment intent")
	}

	orderAmount := utils.NumericToFloat(order.OrderTotalAmount)
	if orderAmount <= 0 {
		return v1dto.PaymentIntentDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Order amount is 0, online payment is not required")
	}

	// IDEMPOTENCY LAYER 1 (Prevent double-charge when FE retries)
	idempotencyKey := strings.TrimSpace(ctx.GetHeader("X-Idempotency-Key"))
	if idempotencyKey == "" {
		return v1dto.PaymentIntentDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Missing X-Idempotency-Key header")
	}

	redisKey := fmt.Sprintf("idem:payment:%s", idempotencyKey)

	// Fast-path: Check redis cache
	var cachedDTO v1dto.PaymentIntentDTO
	if err := s.cache.Get(redisKey, &cachedDTO); err == nil {
		if cachedDTO.OrderUUID != order.OrderUuid.String() {
			return v1dto.PaymentIntentDTO{}, utils.NewError(utils.ErrCodeConflict, "Idempotency key already used for a different order")
		}
		return cachedDTO, nil
	}

	// Slow-path: Check DB. tx = nil because this is a independent query
	existingTxn, err := s.paymentRepo.GetByIdempotencyKey(reqCtx, nil, idempotencyKey)
	if err == nil {
		if existingTxn.PtOrderID != order.OrderID {
			return v1dto.PaymentIntentDTO{}, utils.NewError(utils.ErrCodeConflict, "Idempotency key already used for a different order")
		}
		dto := v1dto.MapToPaymentIntentDTO(existingTxn, order.OrderUuid)
		_ = s.cache.Set(redisKey, dto, 24*time.Hour)

		return dto, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return v1dto.PaymentIntentDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to check idempotency key", err)
	}

	// Get Provider and create payment url
	providerName := strings.ToLower(strings.TrimSpace(input.Provider))
	provider, exists := s.providers[providerName]
	if !exists {
		return v1dto.PaymentIntentDTO{}, utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("Unsupported payment provider: %s", providerName))
	}

	txnRef := fmt.Sprintf("%s_%d", order.OrderNumber, time.Now().Unix())

	paymentReq := payment.CreatePaymentRequest{
		OrderNumber: txnRef,
		Amount:      orderAmount,
		Currency:    "VND",
		Description: fmt.Sprintf("Payment for order %s", order.OrderNumber),
		ReturnURL:   s.returnURL,
		WebhookURL:  fmt.Sprintf("%s/%s", s.webhookBaseURL, providerName),
		ClientIP:    ctx.ClientIP(),
	}
	providerResp, err := provider.CreatePaymentURL(reqCtx, paymentReq)

	if err != nil {
		logger.Log.Error().
			Err(err).
			Str("trace_id", logger.GetTraceId(reqCtx)).
			Str("module", "payment").
			Str("action", "create_intent").
			Str("provider", providerName).
			Str("order_uuid", order.OrderUuid.String()).
			Msg("Failed to generate payment URL from provider")
		return v1dto.PaymentIntentDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to generate payment URL from provider", err)
	}

	// Write new PaymentTransaction to DB
	var providerTxnID *string
	if providerResp.ProviderTxnID != "" {
		providerTxnID = &providerResp.ProviderTxnID
	}

	var providerURL *string
	if providerResp.PaymentURL != "" {
		providerURL = &providerResp.PaymentURL
	}

	ptParams := sqlc.CreatePaymentTransactionParams{
		PtOrderID:        order.OrderID,
		PtIdempotencyKey: idempotencyKey,
		PtProvider:       providerName,
		PtProviderTxnID:  providerTxnID,
		PtProviderUrl:    providerURL,
		PtAmount:         order.OrderTotalAmount,
		PtCurrency:       "VND",
		PtStatus:         PaymentStatusPending,
	}

	paymentTxn, err := s.paymentRepo.Create(reqCtx, nil, ptParams)
	if err != nil {
		return v1dto.PaymentIntentDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to create payment transaction", err)
	}

	responseDTO := v1dto.MapToPaymentIntentDTO(paymentTxn, order.OrderUuid)

	// Save to redis cache for future idempotent requests
	_ = s.cache.Set(redisKey, responseDTO, 24*time.Hour)

	logger.Log.Info().
		Str("trace_id", logger.GetTraceId(reqCtx)).
		Str("module", "payment").
		Str("action", "create_intent").
		Str("order_uuid", order.OrderUuid.String()).
		Str("payment_uuid", paymentTxn.PtUuid.String()).
		Str("provider", providerName).
		Str("status", "Pending").
		Float64("amount", orderAmount).
		Int64("duration_ms", time.Since(startTime).Milliseconds()).
		Msg("Payment intent created successfully")

	return responseDTO, nil
}
