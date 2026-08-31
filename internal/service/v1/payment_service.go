package v1service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/payment"
	"goshop/pkg/rabbitmq"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	amqp "github.com/rabbitmq/amqp091-go"
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

// 1. TẠO PAYMENT INTENT
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

	if order.OrderUserID != user.UserID {
		return v1dto.PaymentIntentDTO{}, utils.NewError(utils.ErrCodeForbidden, "You are not authorized to pay for this order")
	}

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

	// Idempotency check via Redis and DB
	idempotencyKey := strings.TrimSpace(ctx.GetHeader("X-Idempotency-Key"))
	if idempotencyKey == "" {
		return v1dto.PaymentIntentDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Missing X-Idempotency-Key header")
	}

	redisKey := fmt.Sprintf("idem:payment:%s", idempotencyKey)
	var cachedDTO v1dto.PaymentIntentDTO
	if err := s.cache.Get(redisKey, &cachedDTO); err == nil {
		if cachedDTO.OrderUUID != order.OrderUuid.String() {
			return v1dto.PaymentIntentDTO{}, utils.NewError(utils.ErrCodeConflict, "Idempotency key already used for a different order")
		}
		return cachedDTO, nil
	}

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

	providerName := strings.ToLower(strings.TrimSpace(input.Provider))
	provider, exists := s.providers[providerName]
	if !exists {
		return v1dto.PaymentIntentDTO{}, utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("Unsupported payment provider: %s", providerName))
	}

	// Ghép OrderNumber (16 ký tự) + Timestamp để tạo TxnRef duy nhất
	txnRef := fmt.Sprintf("%s_%d", order.OrderNumber, time.Now().Unix())

	webhookBase := strings.TrimRight(s.webhookBaseURL, "/")
	if !strings.Contains(webhookBase, "/api/v1/webhooks/payment") {
		webhookBase = fmt.Sprintf("%s/api/v1/webhooks/payment", webhookBase)
	}
	webhookURL := fmt.Sprintf("%s/%s", webhookBase, providerName)

	paymentReq := payment.CreatePaymentRequest{
		OrderNumber: txnRef,
		Amount:      orderAmount,
		Currency:    "VND",
		Description: fmt.Sprintf("Payment for order %s", order.OrderNumber),
		ReturnURL:   s.returnURL,
		WebhookURL:  webhookURL,
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

	var providerURL *string
	if providerResp.PaymentURL != "" {
		providerURL = &providerResp.PaymentURL
	}

	ptParams := sqlc.CreatePaymentTransactionParams{
		PtOrderID:        order.OrderID,
		PtIdempotencyKey: idempotencyKey,
		PtProvider:       providerName,
		PtProviderTxnID:  &txnRef,
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
	_ = s.cache.Set(redisKey, responseDTO, 24*time.Hour)

	logger.Log.Info().
		Str("trace_id", logger.GetTraceId(reqCtx)).
		Str("module", "payment").
		Str("action", "create_intent").
		Str("order_number", order.OrderNumber).
		Str("order_uuid", order.OrderUuid.String()).
		Str("payment_uuid", paymentTxn.PtUuid.String()).
		Str("provider", providerName).
		Str("status", "Pending").
		Float64("amount", orderAmount).
		Int64("duration_ms", time.Since(startTime).Milliseconds()).
		Msg("Payment intent created successfully")

	return responseDTO, nil
}

// 2. NHẬN WEBHOOK TỪ CỔNG (VERIFY + DEDUP + PUSH RABBITMQ)
func (s *paymentService) HandleWebhook(ctx *gin.Context, providerName string) error {
	reqCtx := ctx.Request.Context()
	startTime := time.Now()

	providerName = strings.ToLower(strings.TrimSpace(providerName))
	provider, exists := s.providers[providerName]
	if !exists {
		return utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("Unsupported payment provider: %s", providerName))
	}

	var rawPayload []byte
	var err error
	if ctx.Request.Method == http.MethodGet {
		rawPayload = []byte(ctx.Request.URL.RawQuery)
	} else {
		rawPayload, err = io.ReadAll(ctx.Request.Body)
		if err != nil {
			return utils.NewError(utils.ErrCodeBadRequest, "Failed to read request body")
		}
	}

	headers := make(map[string]string)
	for k, v := range ctx.Request.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	// Verify HMAC signature
	if err := provider.VerifyWebhook(rawPayload, headers); err != nil {
		logger.Log.Warn().
			Err(err).
			Str("module", "payment").
			Str("action", "webhook_received").
			Str("provider", providerName).
			Msg("Invalid webhook signature")
		return utils.NewError(utils.ErrCodeUnauthorized, "Invalid webhook signature")
	}

	result, err := provider.ParseWebhookPayload(rawPayload)
	if err != nil {
		return utils.WrapError(utils.ErrCodeBadRequest, "Failed to parse webhook payload", err)
	}

	// Dedup qua bảng webhook_events
	eventParams := sqlc.InsertWebhookEventParams{
		WeProvider:        providerName,
		WeExternalEventID: result.EventID,
		WeEventType:       result.EventType,
		WeRawBody:         string(rawPayload),
	}

	event, err := s.webhookRepo.InsertEvent(reqCtx, nil, eventParams)
	if err != nil || event.WeID == 0 {
		logger.Log.Info().
			Str("module", "payment").
			Str("action", "webhook_received").
			Str("event_id", result.EventID).
			Msg("Duplicate webhook event received, skipping")
		return nil
	}

	// Tách lấy OrderNumber gốc (16 ký tự)
	orderNumberParts := strings.Split(result.OrderNumber, "_")
	realOrderNumber := orderNumberParts[0]

	// Lấy order để lấy OrderUUID
	var orderUUID string
	order, err := s.orderRepo.GetByOrderNumber(reqCtx, realOrderNumber)
	if err == nil {
		orderUUID = order.OrderUuid.String()
	}

	msgPayload := v1dto.PaymentWebhookMessage{
		WebhookEventID: event.WeID,
		OrderUUID:      orderUUID,
		OrderNumber:    result.OrderNumber,
		Provider:       providerName,
		EventType:      result.EventType,
		ProviderTxnID:  result.ProviderTxnID,
		Amount:         result.Amount,
		Status:         string(result.Status),
		Metadata:       result.Metadata,
	}

	if err := s.rabbitmq.Publish(reqCtx, "payment.webhook", msgPayload); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to publish payment webhook to RabbitMQ")
		return utils.WrapError(utils.ErrCodeInternal, "failed to enqueue webhook event", err)
	}

	logger.Log.Info().
		Str("module", "payment").
		Str("action", "webhook_received").
		Str("provider", providerName).
		Str("event_id", result.EventID).
		Str("order_number", realOrderNumber).
		Int64("duration_ms", time.Since(startTime).Milliseconds()).
		Msg("Payment webhook accepted and enqueued")

	return nil
}

// 3. CONSUMER XỬ LÝ MESSAGE CẬP NHẬT TRẠNG THÁI TRONG DB
func (s *paymentService) ProcessWebhookMessage(headers amqp.Table, body []byte) error {
	ctx := context.Background()
	startTime := time.Now()

	var msg v1dto.PaymentWebhookMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to unmarshal payment webhook message")
		return err
	}

	var newPaymentStatus int32 = PaymentStatusFailed
	if msg.Status == "success" {
		newPaymentStatus = PaymentStatusPaid
	}

	/* =================== BEGIN TRANSACTION =================== */
	tx, err := db.DBPool.Begin(ctx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to begin database transaction")
		return err
	}
	defer tx.Rollback(ctx) // Tự động rollback nếu có lỗi xảy ra trước khi commit

	// 2. TÌM CHÍNH XÁC TRANSACTION BẰNG TXN_REF TRONG 1MS (Dùng index pt_provider_txn_id)
	// msg.OrderNumber lúc này chứa chuỗi "260831-05-BZ7AMC_1788100000"
	pt, err := s.paymentRepo.GetByProviderTxnID(ctx, tx, msg.Provider, msg.OrderNumber)
	if err != nil {
		logger.Log.Error().
			Err(err).
			Str("provider", msg.Provider).
			Str("txn_ref", msg.OrderNumber).
			Msg("Payment transaction not found by txn_ref")
		return err // Trả lỗi để RabbitMQ retry
	}

	// 3. CẬP NHẬT TRẠNG THÁI TRANSACTION VÀ GHI ĐÈ MÃ GIAO DỊCH NGÂN HÀNG THẬT
	_, err = s.paymentRepo.UpdateStatusAtomically(ctx, tx, sqlc.UpdatePaymentStatusAtomicallyParams{
		PtUuid:              pt.PtUuid,
		NewStatus:           newPaymentStatus,
		ProviderTxnID:       &msg.ProviderTxnID, // Lưu mã ngân hàng thật (ví dụ: transId MoMo)
		AllowedFromStatuses: []int32{PaymentStatusPending},
	})
	if err != nil {
		logger.Log.Error().Err(err).Str("pt_uuid", pt.PtUuid.String()).Msg("Failed to update payment transaction")
		return err
	}

	// 4. CẬP NHẬT TRẠNG THÁI ORDER SANG PAID (Dùng pt.PtOrderID hoặc msg.OrderUUID)
	if msg.Status == "success" {
		// Tách mã đơn gốc (ví dụ: "260831-05-BZ7AMC")
		realOrderNumber := strings.Split(msg.OrderNumber, "_")[0]

		order, err := s.orderRepo.GetByOrderNumber(ctx, realOrderNumber)
		if err == nil {
			_, err = s.orderRepo.UpdateOrderPaymentStatus(ctx, sqlc.UpdateOrderPaymentStatusParams{
				OrderUuid:           order.OrderUuid,
				PaymentStatus:       OrderPaymentStatusPaid,
				RefundedAmount:      utils.FloatToNumeric(0),
				AllowedFromStatuses: []int32{OrderPaymentStatusUnpaid},
			})
			if err != nil {
				logger.Log.Error().Err(err).Str("order_number", realOrderNumber).Msg("Failed to update order payment status")
				return err
			}
		}
	}

	// 5. ĐÁNH DẤU WEBHOOK EVENT ĐÃ XỬ LÝ
	_ = s.webhookRepo.MarkProcessed(ctx, tx, msg.WebhookEventID)

	/* =================== COMMIT TRANSACTION =================== */
	if err := tx.Commit(ctx); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to commit database transaction")
		return err
	}

	logger.Log.Info().
		Str("module", "payment").
		Str("action", "webhook_processed").
		Str("order_number", msg.OrderNumber).
		Str("status", msg.Status).
		Int64("duration_ms", time.Since(startTime).Milliseconds()).
		Msg("Payment webhook processed successfully")

	return nil
}

func (s *paymentService) GetPaymentHistory(ctx *gin.Context, orderUUID uuid.UUID) ([]v1dto.PaymentTransactionDTO, error) {
	reqCtx := ctx.Request.Context()

	userUuidStr, ok := ctx.Get("user_uuid")
	if !ok || userUuidStr == "" {
		return nil, utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized")
	}

	userUuid, err := uuid.Parse(userUuidStr.(string))
	if err != nil {
		return nil, utils.NewError(utils.ErrCodeUnauthorized, "Invalid user UUID format")
	}

	user, err := s.userRepo.GetByUUID(reqCtx, userUuid)
	if err != nil {
		return nil, utils.NewError(utils.ErrCodeUnauthorized, "User not found")
	}

	order, err := s.orderRepo.GetByUUID(reqCtx, orderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "Order not found")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get order", err)
	}

	if order.OrderUserID != user.UserID && user.UserLevel != RoleAdmin && user.UserLevel != RoleModerator {
		return nil, utils.NewError(utils.ErrCodeForbidden, "You are not authorized to view payment history for this order")
	}

	paymentTxns, err := s.paymentRepo.GetByOrderID(reqCtx, nil, order.OrderID)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get payment transactions", err)
	}

	var history []v1dto.PaymentTransactionDTO
	for _, txn := range paymentTxns {
		history = append(history, v1dto.MapToPaymentTransactionDTO(txn, order.OrderUuid))
	}

	return history, nil
}
