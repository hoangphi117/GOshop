package v1service

import (
	"context"
	"crypto/sha256"
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
	"github.com/jackc/pgx/v5/pgconn"
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
		// Kiểm tra nếu là lỗi trùng Idempotency Key
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			logger.Log.Warn().Str("key", idempotencyKey).Msg("Concurrent duplicate payment intent request detected, fetching existing transaction")

			// Graceful Recovery: Lấy transaction vừa được tạo bởi request chạy trước
			existingTxn, getErr := s.paymentRepo.GetByIdempotencyKey(reqCtx, nil, idempotencyKey)
			if getErr == nil {
				return v1dto.MapToPaymentIntentDTO(existingTxn, order.OrderUuid), nil
			}
		}
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
	if err != nil {
		// Chỉ coi là duplicate khi DB trả về ErrNoRows (do ON CONFLICT DO NOTHING)
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Log.Info().
				Str("provider", providerName).
				Str("event_id", result.EventID).
				Msg("Duplicate webhook event received (ON CONFLICT), acknowledging gateway")
			return nil // Báo 200 OK cho Gateway biết đã nhận rồi, đừng gửi nữa
		}
		// Nếu là lỗi DB thật (mất kết nối, timeout...) -> BẮT BUỘC trả lỗi để Handler trả 500
		// Gateway nhận mã HTTP 500 sẽ tự động Retry lại sau 1p, 5p, 15p
		logger.Log.Error().
			Err(err).
			Str("provider", providerName).
			Str("event_id", result.EventID).
			Msg("Database failure while inserting webhook event")
		return utils.WrapError(utils.ErrCodeInternal, "failed to persist webhook event", err)
	}

	// Nếu không lỗi nhưng WeID == 0 (phòng trường hợp thư viện sqlc không trả ErrNoRows)
	if event.WeID == 0 {
		logger.Log.Info().
			Str("provider", providerName).
			Str("event_id", result.EventID).
			Msg("Duplicate webhook event received (WeID=0), acknowledging gateway")
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

	// KIỂM TRA IDEMPOTENCY / REDELIVERY: Nếu giao dịch đã ở trạng thái Paid từ trước, ACK ngay lập tức
	if pt.PtStatus == PaymentStatusPaid {
		logger.Log.Info().
			Str("module", "payment").
			Str("action", "webhook_idempotent_skip").
			Str("pt_uuid", pt.PtUuid.String()).
			Str("order_number", msg.OrderNumber).
			Msg("Payment transaction is already Paid, acknowledging redelivered message as idempotent success")

		_ = s.webhookRepo.MarkProcessed(ctx, tx, msg.WebhookEventID)
		_ = tx.Commit(ctx)
		return nil // rả về nil để RabbitMQ ACK message thành công, KHÔNG retry vào DLQ
	}

	// 3. CẬP NHẬT TRẠNG THÁI TRANSACTION VÀ GHI ĐÈ MÃ GIAO DỊCH NGÂN HÀNG THẬT (LƯU KÈM TXN_REF VÀO METADATA)
	metaBytes, _ := json.Marshal(map[string]interface{}{
		"txn_ref":  msg.OrderNumber,
		"trans_id": msg.ProviderTxnID,
	})
	_, err = s.paymentRepo.UpdateStatusAtomically(ctx, tx, sqlc.UpdatePaymentStatusAtomicallyParams{
		PtUuid:              pt.PtUuid,
		NewStatus:           newPaymentStatus,
		ProviderTxnID:       &msg.ProviderTxnID, // Lưu mã ngân hàng thật (ví dụ: transId MoMo)
		Metadata:            metaBytes,
		AllowedFromStatuses: []int32{PaymentStatusPending},
	})
	if err != nil {
		logger.Log.Error().Err(err).Str("pt_uuid", pt.PtUuid.String()).Msg("Failed to update payment transaction")
		return err
	}

	// 4. CẬP NHẬT TRẠNG THÁI ORDER SANG PAID (Dùng pt.PtOrderID hoặc msg.OrderUUID)
	if msg.Status == "success" {
		// Tách mã đơn gốc (ví dụ: "260831-05-BZ7AMC")
		realOrderNumber, _, _ := strings.Cut(msg.OrderNumber, "_")

		order, err := s.orderRepo.GetByOrderNumber(ctx, realOrderNumber)
		if err == nil {
			_, err = s.orderRepo.UpdateOrderPaymentStatus(ctx, tx, sqlc.UpdateOrderPaymentStatusParams{
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
	if err := s.webhookRepo.MarkProcessed(ctx, tx, msg.WebhookEventID); err != nil {
		logger.Log.Error().
			Err(err).
			Int32("webhook_event_id", msg.WebhookEventID).
			Msg("Failed to mark webhook event as processed")
		return utils.WrapError(utils.ErrCodeInternal, "failed to mark webhook event as processed", err)
	}

	/* =================== COMMIT TRANSACTION =================== */
	if err := tx.Commit(ctx); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to commit database transaction")
		return err
	}

	_ = s.cache.Clear("orders:*")

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

func (s *paymentService) ConfirmCODPayment(ctx *gin.Context, orderUUID uuid.UUID) error {
	reqCtx := ctx.Request.Context()
	startTime := time.Now()

	tx, err := db.DBPool.Begin(reqCtx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to begin database transaction for COD confirmation")
		return utils.WrapError(utils.ErrCodeInternal, "failed to begin database transaction", err)
	}
	defer tx.Rollback(reqCtx)

	order, err := s.orderRepo.GetByUUID(reqCtx, orderUUID)
	if err != nil {
		_ = tx.Rollback(reqCtx)
		if errors.Is(err, pgx.ErrNoRows) {
			return utils.NewError(utils.ErrCodeNotFound, "Order not found")
		}
		return utils.WrapError(utils.ErrCodeInternal, "failed to get order", err)
	}

	if order.OrderPaymentMethod != OrderPaymentMethodCOD {
		_ = tx.Rollback(reqCtx)
		return utils.NewError(utils.ErrCodeBadRequest, "Order is not a COD order")
	}

	if order.OrderPaymentStatus == OrderPaymentStatusPaid {
		_ = tx.Rollback(reqCtx)
		return utils.NewError(utils.ErrCodeBadRequest, "Order has already been confirmed as paid")
	}

	if order.OrderStatus == OrderStatusCancelled {
		_ = tx.Rollback(reqCtx)
		return utils.NewError(utils.ErrCodeBadRequest, "Order is cancelled, cannot confirm payment")
	}

	// Tạo record lưu vết trong payment_transactions
	idempotencyKey := fmt.Sprintf("cod:%s", order.OrderNumber)
	providerTxnID := fmt.Sprintf("COD_%s", order.OrderNumber)

	ptParams := sqlc.CreatePaymentTransactionParams{
		PtOrderID:        order.OrderID,
		PtIdempotencyKey: idempotencyKey,
		PtProvider:       "cod",
		PtProviderTxnID:  &providerTxnID,
		PtAmount:         order.OrderTotalAmount,
		PtCurrency:       "VND",
		PtStatus:         PaymentStatusPaid,
	}

	paymentTxn, err := s.paymentRepo.Create(reqCtx, tx, ptParams)
	if err != nil {
		logger.Log.Error().Err(err).Str("order_number", order.OrderNumber).Msg("Failed to create COD payment transaction")
		return utils.WrapError(utils.ErrCodeInternal, "failed to create payment transaction", err)
	}

	_, err = s.orderRepo.UpdateOrderPaymentStatus(reqCtx, tx, sqlc.UpdateOrderPaymentStatusParams{
		OrderUuid:           order.OrderUuid,
		PaymentStatus:       OrderPaymentStatusPaid,
		RefundedAmount:      utils.FloatToNumeric(0),
		AllowedFromStatuses: []int32{OrderPaymentStatusUnpaid},
	})

	if err != nil {
		logger.Log.Error().Err(err).Str("order_number", order.OrderNumber).Msg("Failed to update order payment status for COD confirmation")
		return utils.WrapError(utils.ErrCodeInternal, "failed to update order payment status", err)
	}

	if err := tx.Commit(reqCtx); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to commit database transaction for COD confirmation")
		return utils.WrapError(utils.ErrCodeInternal, "failed to commit database transaction", err)
	}

	_ = s.cache.Clear("orders:*")

	logger.Log.Info().
		Str("trace_id", logger.GetTraceId(reqCtx)).
		Str("module", "payment").
		Str("action", "confirm_cod").
		Str("order_number", order.OrderNumber).
		Str("order_uuid", order.OrderUuid.String()).
		Str("payment_uuid", paymentTxn.PtUuid.String()).
		Float64("amount", utils.NumericToFloat(order.OrderTotalAmount)).
		Int64("duration_ms", time.Since(startTime).Milliseconds()).
		Msg("COD payment confirmed successfully")
	return nil
}

func (s *paymentService) RefundPayment(ctx *gin.Context, input v1dto.RefundPaymentInput) (v1dto.PaymentTransactionDTO, error) {
	reqCtx := ctx.Request.Context()
	startTime := time.Now()

	orderUUID, err := uuid.Parse(input.OrderUUID)
	if err != nil {
		return v1dto.PaymentTransactionDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Invalid order UUID format")
	}

	/* ============================================================
	   PHASE 1: RESERVE & CREATE PENDING AUDIT RECORD (DB TX)
	   ============================================================ */
	txPhase1, err := db.DBPool.Begin(reqCtx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to begin phase 1 db transaction for refund")
		return v1dto.PaymentTransactionDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to begin db transaction", err)
	}
	defer txPhase1.Rollback(reqCtx)

	// A. Pessimistic Lock (FOR UPDATE) để chặn Race Condition hoàn tiền vượt hạn mức
	order, err := s.orderRepo.GetByUUIDForUpdate(reqCtx, txPhase1, orderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return v1dto.PaymentTransactionDTO{}, utils.NewError(utils.ErrCodeNotFound, "Order not found")
		}
		return v1dto.PaymentTransactionDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to get order", err)
	}

	// B. Kiểm tra điều kiện nghiệp vụ
	if order.OrderPaymentStatus != OrderPaymentStatusPaid && order.OrderPaymentStatus != OrderPaymentStatusPartiallyRefunded {
		return v1dto.PaymentTransactionDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Only Paid or Partially Refunded orders can be refunded")
	}

	totalAmount := utils.NumericToFloat(order.OrderTotalAmount)
	currentRefunded := utils.NumericToFloat(order.OrderRefundedAmount)
	remainingRefundable := totalAmount - currentRefunded

	if input.Amount <= 0 {
		return v1dto.PaymentTransactionDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Refund amount must be greater than 0")
	}
	if input.Amount > remainingRefundable {
		return v1dto.PaymentTransactionDTO{}, utils.NewError(utils.ErrCodeBadRequest,
			fmt.Sprintf("Refund amount exceeds refundable balance. Maximum refundable: %.2f VND", remainingRefundable))
	}

	// 1. Chuẩn hóa Idempotency Key: Ưu tiên Client Header -> Fallback sang Deterministic Hash (Không dùng time.Now)
	idempotencyKey := ctx.GetHeader("X-Idempotency-Key")
	if idempotencyKey == "" {
		raw := fmt.Sprintf("%s:%.2f:%s:%.2f", order.OrderNumber, input.Amount, strings.TrimSpace(input.Reason), currentRefunded)
		hash := sha256.Sum256([]byte(raw))
		idempotencyKey = fmt.Sprintf("refund:%s:%x", order.OrderNumber, hash[:8])
	}

	// C. Tìm giao dịch thanh toán gốc đã Paid
	txns, err := s.paymentRepo.GetByOrderID(reqCtx, txPhase1, order.OrderID)
	if err != nil {
		return v1dto.PaymentTransactionDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to get payment history", err)
	}

	var originalTxn *sqlc.PaymentTransaction
	for _, t := range txns {
		if t.PtStatus == PaymentStatusPaid && t.PtRefundOf == nil {
			originalTxn = &t
			break
		}
	}
	if originalTxn == nil {
		return v1dto.PaymentTransactionDTO{}, utils.NewError(utils.ErrCodeBadRequest, "No successful original payment transaction found to refund")
	}

	// D. Tạo bản ghi giao dịch Refund ở trạng thái PENDING (Lưu vết trước khi ra Gateway)
	refundNumeric := utils.FloatToNumeric(input.Amount)
	pendingRefundTxn, err := s.paymentRepo.CreateRefund(reqCtx, txPhase1, sqlc.CreateRefundTransactionParams{
		OrderID:        order.OrderID,
		IdempotencyKey: idempotencyKey,
		Provider:       originalTxn.PtProvider,
		ProviderTxnID:  nil,
		Amount:         refundNumeric,
		Currency:       "VND",
		Status:         PaymentStatusPending, // 1: Pending trong quá trình gọi Gateway
		RefundOf:       &originalTxn.PtID,
		RefundAmount:   refundNumeric,
	})
	if err != nil {
		// Kiểm tra nếu là lỗi trùng lặp Idempotency Key
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			_ = txPhase1.Rollback(reqCtx)

			// Tra cứu bản ghi giao dịch refund đã tồn tại
			existingTxn, getErr := s.paymentRepo.GetByIdempotencyKey(reqCtx, nil, idempotencyKey)
			if getErr == nil {
				if existingTxn.PtStatus == PaymentStatusPaid {
					logger.Log.Info().
						Str("order_number", order.OrderNumber).
						Str("idempotency_key", idempotencyKey).
						Msg("Duplicate refund request detected, returning existing successful refund transaction")
					return v1dto.MapToPaymentTransactionDTO(existingTxn, order.OrderUuid), nil
				}
				if existingTxn.PtStatus == PaymentStatusPending {
					return v1dto.PaymentTransactionDTO{}, utils.NewError(utils.ErrCodeConflict, "A refund request for this order is currently in progress. Please wait.")
				}
			}

			logger.Log.Warn().
				Str("order_number", order.OrderNumber).
				Str("idempotency_key", idempotencyKey).
				Msg("Duplicate refund request with conflict idempotency key")
			return v1dto.PaymentTransactionDTO{}, utils.NewError(utils.ErrCodeConflict, "Duplicate refund request detected. This operation has already been submitted.")
		}

		logger.Log.Error().Err(err).Str("order_number", order.OrderNumber).Msg("Failed to reserve pending refund transaction")
		return v1dto.PaymentTransactionDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to reserve refund transaction", err)
	}

	if err := txPhase1.Commit(reqCtx); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to commit phase 1 refund transaction")
		return v1dto.PaymentTransactionDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to commit phase 1", err)
	}

	/* ============================================================
	   PHASE 2: CALL EXTERNAL GATEWAY REFUND API
	   ============================================================ */
	var refundTxnID string
	var gatewayErr error

	if originalTxn.PtProvider != "cod" {
		provider, exists := s.providers[originalTxn.PtProvider]
		if !exists {
			gatewayErr = fmt.Errorf("payment provider %s is not supported for refund", originalTxn.PtProvider)
		} else {
			providerTxnID := ""
			if originalTxn.PtProviderTxnID != nil {
				providerTxnID = *originalTxn.PtProviderTxnID
			}

			refundResp, err := provider.Refund(reqCtx, payment.RefundRequest{
				OrderNumber:   order.OrderNumber,
				ProviderTxnID: providerTxnID,
				Amount:        input.Amount,
				TotalAmount:   totalAmount,
				Reason:        input.Reason,
			})
			if err != nil {
				gatewayErr = err
			} else {
				refundTxnID = refundResp.RefundTxnID
			}
		}
	} else {
		// Đơn COD: Hoàn tiền mặt nội bộ
		refundTxnID = fmt.Sprintf("REFUND_COD_%s_%d", order.OrderNumber, time.Now().Unix())
	}

	/* ============================================================
	   PHASE 3: CONFIRM OR FAIL (FINALIZE IN DB TX)
	   ============================================================ */
	txPhase3, err := db.DBPool.Begin(reqCtx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to begin phase 3 db transaction for refund")
		return v1dto.PaymentTransactionDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to begin phase 3 db transaction", err)
	}
	defer txPhase3.Rollback(reqCtx)

	if gatewayErr != nil {
		// Gateway thất bại -> Đánh dấu bản ghi là FAILED (Giữ nguyên audit trail)
		_, _ = s.paymentRepo.UpdateStatusAtomically(reqCtx, txPhase3, sqlc.UpdatePaymentStatusAtomicallyParams{
			PtUuid:              pendingRefundTxn.PtUuid,
			NewStatus:           PaymentStatusFailed,
			AllowedFromStatuses: []int32{PaymentStatusPending},
		})
		_ = txPhase3.Commit(reqCtx)
		logger.Log.Error().
			Err(gatewayErr).
			Str("provider", originalTxn.PtProvider).
			Str("order_number", order.OrderNumber).
			Msg("Gateway refund failed, marked transaction as Failed")
		return v1dto.PaymentTransactionDTO{}, utils.WrapError(utils.ErrCodeBadGateway, "Gateway refund failed", gatewayErr)
	}

	// Gateway thành công -> Chuyển sang PAID (2)
	finalRefundTxn, err := s.paymentRepo.UpdateStatusAtomically(reqCtx, txPhase3, sqlc.UpdatePaymentStatusAtomicallyParams{
		PtUuid:              pendingRefundTxn.PtUuid,
		NewStatus:           PaymentStatusPaid,
		ProviderTxnID:       &refundTxnID,
		AllowedFromStatuses: []int32{PaymentStatusPending},
	})
	if err != nil {
		logger.Log.Error().Err(err).Str("order_number", order.OrderNumber).Msg("Failed to confirm refund transaction")
		return v1dto.PaymentTransactionDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to confirm refund transaction", err)
	}

	// Cập nhật trạng thái và số tiền đã hoàn trong bảng orders
	newTotalRefunded := currentRefunded + input.Amount
	var newOrderStatus int32 = OrderPaymentStatusPartiallyRefunded
	if newTotalRefunded >= totalAmount {
		newOrderStatus = OrderPaymentStatusFullyRefunded
	}

	_, err = s.orderRepo.UpdateOrderPaymentStatus(reqCtx, txPhase3, sqlc.UpdateOrderPaymentStatusParams{
		OrderUuid:           order.OrderUuid,
		PaymentStatus:       newOrderStatus,
		RefundedAmount:      utils.FloatToNumeric(newTotalRefunded),
		AllowedFromStatuses: []int32{OrderPaymentStatusPaid, OrderPaymentStatusPartiallyRefunded},
	})
	if err != nil {
		logger.Log.Error().Err(err).Str("order_number", order.OrderNumber).Msg("Failed to update order payment status after refund")
		return v1dto.PaymentTransactionDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to update order payment status", err)
	}

	if err := txPhase3.Commit(reqCtx); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to commit phase 3 database transaction for refund")
		return v1dto.PaymentTransactionDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to commit database transaction", err)
	}

	_ = s.cache.Clear("orders:*")

	logger.Log.Info().
		Str("trace_id", logger.GetTraceId(reqCtx)).
		Str("module", "payment").
		Str("action", "refund").
		Str("order_number", order.OrderNumber).
		Str("order_uuid", order.OrderUuid.String()).
		Str("refund_txn_uuid", finalRefundTxn.PtUuid.String()).
		Float64("refund_amount", input.Amount).
		Float64("new_total_refunded", newTotalRefunded).
		Int32("new_payment_status", newOrderStatus).
		Int64("duration_ms", time.Since(startTime).Milliseconds()).
		Msg("Payment refund processed successfully")

	return v1dto.MapToPaymentTransactionDTO(finalRefundTxn, order.OrderUuid), nil
}

func (s *paymentService) ReconcilePendingPayments(ctx context.Context) error {
	startTime := time.Now()
	olderThan := time.Now().Add(-15 * time.Minute)

	// 1. Lấy danh sách tối đa 50 giao dịch Pending tạo trước 15 phút (kèm thông tin Order và User qua JOIN)
	pendingTxns, err := s.paymentRepo.GetPendingTransactionsOlderThan(ctx, nil, sqlc.GetPendingTransactionsOlderThanParams{
		OlderThan: olderThan,
		Lim:       50,
	})
	if err != nil {
		logger.Log.Error().Err(err).Msg("Reconciliation: Failed to fetch pending payment transactions")
		return utils.WrapError(utils.ErrCodeInternal, "failed to fetch pending transactions for reconciliation", err)
	}

	if len(pendingTxns) == 0 {
		return nil // Không có giao dịch treo cần xử lý
	}

	logger.Log.Info().
		Int("count", len(pendingTxns)).
		Msg("Reconciliation: Processing expired/pending payment transactions")

	successCount := 0
	expiredCount := 0

	// 2. Duyệt từng giao dịch treo để đối soát
	for _, txn := range pendingTxns {
		provider, exists := s.providers[txn.PtProvider]
		if !exists {
			logger.Log.Warn().
				Str("provider", txn.PtProvider).
				Str("pt_uuid", txn.PtUuid.String()).
				Msg("Reconciliation: Provider not supported, skipping")
			continue
		}

		queryOrderID := txn.OrderNumber

		// Ưu tiên 1: Lấy txn_ref từ metadata JSONB
		if len(txn.PtMetadata) > 0 {
			var meta map[string]interface{}
			if err := json.Unmarshal(txn.PtMetadata, &meta); err == nil {
				if ref, ok := meta["txn_ref"].(string); ok && ref != "" {
					queryOrderID = ref
				}
			}
		}

		// Ưu tiên 2: Nếu metadata chưa có nhưng pt_provider_txn_id chứa txnRef (có dấu "_")
		if queryOrderID == txn.OrderNumber && txn.PtProviderTxnID != nil && strings.Contains(*txn.PtProviderTxnID, "_") {
			queryOrderID = *txn.PtProviderTxnID
		}

		// A. Gọi API Cổng thanh toán để kiểm tra trạng thái thực tế theo txnRef/orderId
		status, err := provider.QueryTransactionStatus(ctx, queryOrderID)
		if err != nil {
			logger.Log.Warn().
				Err(err).
				Str("provider", txn.PtProvider).
				Str("query_order_id", queryOrderID).
				Msg("Reconciliation: Failed to query transaction status from gateway")
			continue
		}

		// B. Bắt đầu Database Transaction để cập nhật Atomic
		tx, err := db.DBPool.Begin(ctx)
		if err != nil {
			logger.Log.Error().Err(err).Msg("Reconciliation: Failed to begin db transaction")
			continue
		}

		switch status {
		case payment.TxnStatusSuccess:
			// TRƯỜNG HỢP 1: Khách đã trả tiền thành công nhưng Webhook bị rớt mạng -> Bù trạng thái PAID (2)
			_, err = s.paymentRepo.UpdateStatusAtomically(ctx, tx, sqlc.UpdatePaymentStatusAtomicallyParams{
				PtUuid:              txn.PtUuid,
				NewStatus:           PaymentStatusPaid,
				AllowedFromStatuses: []int32{PaymentStatusPending},
			})
			if err != nil {
				_ = tx.Rollback(ctx)
				logger.Log.Error().Err(err).Str("pt_uuid", txn.PtUuid.String()).Msg("Reconciliation: Failed to update payment status to Paid")
				continue
			}

			// Cập nhật trạng thái Order sang Paid (Hỗ trợ idempotent nếu Order đã là Paid)
			_, err = s.orderRepo.UpdateOrderPaymentStatus(ctx, tx, sqlc.UpdateOrderPaymentStatusParams{
				OrderUuid:           txn.OrderUuid,
				PaymentStatus:       OrderPaymentStatusPaid,
				RefundedAmount:      utils.FloatToNumeric(0),
				AllowedFromStatuses: []int32{OrderPaymentStatusUnpaid, OrderPaymentStatusPaid},
			})
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				_ = tx.Rollback(ctx)
				logger.Log.Error().Err(err).Str("order_number", txn.OrderNumber).Msg("Reconciliation: Failed to update order payment status to Paid")
				continue
			}

			if err := tx.Commit(ctx); err != nil {
				logger.Log.Error().Err(err).Msg("Reconciliation: Failed to commit Paid update")
				continue
			}

			successCount++
			logger.Log.Info().
				Str("order_number", txn.OrderNumber).
				Str("pt_uuid", txn.PtUuid.String()).
				Msg("Reconciliation: Successfully recovered missed paid transaction")

		case payment.TxnStatusFailed, payment.TxnStatusExpired:
			// TRƯỜNG HỢP 2: Khách bỏ ngang / Giao dịch thất bại / Quá hạn -> Cập nhật EXPIRED (4)
			_, err = s.paymentRepo.UpdateStatusAtomically(ctx, tx, sqlc.UpdatePaymentStatusAtomicallyParams{
				PtUuid:              txn.PtUuid,
				NewStatus:           PaymentStatusExpired, // 4: Expired
				AllowedFromStatuses: []int32{PaymentStatusPending},
			})
			if err != nil {
				_ = tx.Rollback(ctx)
				logger.Log.Error().Err(err).Str("pt_uuid", txn.PtUuid.String()).Msg("Reconciliation: Failed to expire pending transaction")
				continue
			}

			if err := tx.Commit(ctx); err != nil {
				logger.Log.Error().Err(err).Msg("Reconciliation: Failed to commit Expired update")
				continue
			}

			expiredCount++
			logger.Log.Info().
				Str("pt_uuid", txn.PtUuid.String()).
				Msg("Reconciliation: Marked abandoned payment transaction as Expired")

		case payment.TxnStatusPending:
			// Khách vẫn đang trong thời gian mở app (chưa hết hạn gateway) -> Giữ nguyên chờ lần quét sau
			_ = tx.Rollback(ctx)
		}
	}

	// 3. Xóa cache đơn hàng nếu có cập nhật
	if successCount > 0 {
		_ = s.cache.Clear("orders:*")
	}

	logger.Log.Info().
		Int("recovered_paid", successCount).
		Int("marked_expired", expiredCount).
		Int64("duration_ms", time.Since(startTime).Milliseconds()).
		Msg("Reconciliation: Job finished successfully")

	return nil
}

