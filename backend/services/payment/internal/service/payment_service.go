package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/utils"
	paymentsqlc "goshop/services/payment/internal/db/sqlc"
	"goshop/services/payment/internal/dto"
	"goshop/services/payment/internal/provider"
	"goshop/services/payment/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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
	orderReader    OrderReaderClient
	webhookRepo    repository.WebhookEventRepository
	outboxRepo     repository.OutboxRepository
	providers      map[string]provider.PaymentProvider
	cache          cache.RedisCacheService
	rabbitmq       rabbitmq.RabbitMQService
	returnURL      string
	webhookBaseURL string
}

func NewPaymentService(
	paymentRepo repository.PaymentRepository,
	orderReader OrderReaderClient,
	webhookRepo repository.WebhookEventRepository,
	outboxRepo repository.OutboxRepository,
	providers map[string]provider.PaymentProvider,
	cache cache.RedisCacheService,
	rabbitmq rabbitmq.RabbitMQService,
	returnURL string,
	webhookBaseURL string,
) PaymentService {
	return &paymentService{
		paymentRepo:    paymentRepo,
		orderReader:    orderReader,
		webhookRepo:    webhookRepo,
		outboxRepo:     outboxRepo,
		providers:      providers,
		cache:          cache,
		rabbitmq:       rabbitmq,
		returnURL:      returnURL,
		webhookBaseURL: webhookBaseURL,
	}
}

func (s *paymentService) CreatePaymentIntent(ctx context.Context, input dto.CreatePaymentIntentInput) (dto.PaymentIntentDTO, error) {
	if s.orderReader == nil {
		return dto.PaymentIntentDTO{}, errors.New("order reader service unavailable")
	}

	order, err := s.orderReader.GetByUUID(ctx, input.OrderUUID)
	if err != nil {
		return dto.PaymentIntentDTO{}, fmt.Errorf("failed to fetch order: %w", err)
	}
	if order == nil {
		return dto.PaymentIntentDTO{}, fmt.Errorf("order not found for uuid: %s", input.OrderUUID)
	}
	if order.OrderNumber == "" {
		return dto.PaymentIntentDTO{}, errors.New("order number is missing")
	}

	if input.UserID != 0 && order.UserID != input.UserID {
		return dto.PaymentIntentDTO{}, errors.New("unauthorized: order does not belong to user")
	}

	if order.PaymentStatus == PaymentStatusPaid {
		return dto.PaymentIntentDTO{}, errors.New("order is already paid")
	}
	if order.OrderStatus == 6 { // CANCELLED
		return dto.PaymentIntentDTO{}, errors.New("order is cancelled")
	}

	// COD Payment flow
	if input.Provider == "cod" {
		idempotencyKey := input.IdempotencyKey
		if idempotencyKey == "" {
			idempotencyKey = fmt.Sprintf("cod:%s", input.OrderUUID)
		}

		txn, err := s.paymentRepo.Create(ctx, paymentsqlc.CreatePaymentTransactionParams{
			OrderID:        order.OrderID,
			OrderUuid:      pgtype.UUID{Bytes: order.OrderUUID, Valid: true},
			IdempotencyKey: idempotencyKey,
			Provider:       "cod",
			TxnRef:         &order.OrderNumber,
			Amount:         utils.FloatToNumeric(order.TotalAmount),
			Currency:       "VND",
			Status:         PaymentStatusPending,
		})
		if err != nil {
			return dto.PaymentIntentDTO{}, fmt.Errorf("failed to create COD transaction: %w", err)
		}

		return dto.MapToPaymentIntentDTO(txn, order.OrderUUID), nil
	}

	pvd, exists := s.providers[input.Provider]
	if !exists {
		return dto.PaymentIntentDTO{}, fmt.Errorf("payment provider not supported: %s", input.Provider)
	}

	idempotencyKey := input.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("intent:%s:%s", input.Provider, input.OrderUUID)
	}

	existingTxn, err := s.paymentRepo.GetByIdempotencyKey(ctx, idempotencyKey)
	if err == nil {
		if existingTxn.PtStatus == PaymentStatusPending && existingTxn.PtProviderUrl != nil && *existingTxn.PtProviderUrl != "" {
			return dto.MapToPaymentIntentDTO(existingTxn, order.OrderUUID), nil
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		logger.Log.Warn().Err(err).Msg("Database check for idempotency returned non-standard error, proceeding to create intent")
	}

	txnRef := order.OrderNumber
	createdTxn, err := s.paymentRepo.Create(ctx, paymentsqlc.CreatePaymentTransactionParams{
		OrderID:        order.OrderID,
		OrderUuid:      pgtype.UUID{Bytes: order.OrderUUID, Valid: true},
		IdempotencyKey: idempotencyKey,
		Provider:       input.Provider,
		TxnRef:         &txnRef,
		Amount:         utils.FloatToNumeric(order.TotalAmount),
		Currency:       "VND",
		Status:         PaymentStatusPending,
	})
	if err != nil {
		return dto.PaymentIntentDTO{}, fmt.Errorf("failed to create pending transaction: %w", err)
	}

	webhookURL := fmt.Sprintf("%s/api/v1/payments/webhook/%s", s.webhookBaseURL, input.Provider)
	resp, err := pvd.CreatePaymentURL(ctx, provider.CreatePaymentRequest{
		OrderNumber: order.OrderNumber,
		Amount:      order.TotalAmount,
		Currency:    "VND",
		Description: fmt.Sprintf("Thanh toan don hang #%s", order.OrderNumber),
		ReturnURL:   s.returnURL,
		WebhookURL:  webhookURL,
		ClientIP:    input.ClientIP,
	})
	if err != nil {
		failStatus := PaymentStatusFailed
		_, updateErr := s.paymentRepo.UpdateStatusAtomically(ctx, paymentsqlc.UpdatePaymentStatusAtomicallyParams{
			PtUuid:              createdTxn.PtUuid,
			NewStatus:           failStatus,
			AllowedFromStatuses: []int32{PaymentStatusPending},
		})
		if updateErr != nil {
			logger.Log.Error().Err(updateErr).Msg("Failed to mark transaction as failed after provider error")
		}
		return dto.PaymentIntentDTO{}, fmt.Errorf("payment gateway provider returned error: %w", err)
	}

	var providerTxnIDPtr *string
	if resp.ProviderTxnID != "" {
		providerTxnIDPtr = &resp.ProviderTxnID
	}
	updatedTxn, err := s.paymentRepo.UpdateProviderInfo(ctx, paymentsqlc.UpdatePaymentProviderInfoParams{
		PtUuid:        createdTxn.PtUuid,
		ProviderUrl:   &resp.PaymentURL,
		ProviderTxnID: providerTxnIDPtr,
	})
	if err != nil {
		return dto.PaymentIntentDTO{}, fmt.Errorf("failed to update provider info on transaction: %w", err)
	}

	return dto.MapToPaymentIntentDTO(updatedTxn, order.OrderUUID), nil
}

func (s *paymentService) GetPaymentTransactionsByOrderID(ctx context.Context, orderID int32) ([]dto.PaymentTransactionDTO, error) {
	txns, err := s.paymentRepo.GetByOrderID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve transactions for order %d: %w", orderID, err)
	}

	result := make([]dto.PaymentTransactionDTO, len(txns))
	for i, t := range txns {
		var orderUUID uuid.UUID
		if t.PtOrderUuid.Valid {
			orderUUID = t.PtOrderUuid.Bytes
		}
		result[i] = dto.MapToPaymentTransactionDTO(t, orderUUID)
	}
	return result, nil
}

func (s *paymentService) GetPaymentTransactionsByOrderUUID(ctx context.Context, orderUUID uuid.UUID) ([]dto.PaymentTransactionDTO, error) {
	txns, err := s.paymentRepo.GetByOrderUUID(ctx, orderUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve transactions for order uuid %s: %w", orderUUID, err)
	}

	result := make([]dto.PaymentTransactionDTO, len(txns))
	for i, t := range txns {
		result[i] = dto.MapToPaymentTransactionDTO(t, orderUUID)
	}
	return result, nil
}

func (s *paymentService) ConfirmCODPayment(ctx context.Context, orderUUID uuid.UUID) error {
	txns, err := s.paymentRepo.GetByOrderUUID(ctx, orderUUID)
	if err != nil {
		return fmt.Errorf("failed to find transactions for order: %w", err)
	}

	var codTxn *paymentsqlc.PaymentTransaction
	for _, t := range txns {
		if t.PtProvider == "cod" && t.PtStatus == PaymentStatusPending {
			txnCopy := t
			codTxn = &txnCopy
			break
		}
	}

	if codTxn == nil {
		return errors.New("no pending COD transaction found for order")
	}

	updated, err := s.paymentRepo.UpdateStatusAtomically(ctx, paymentsqlc.UpdatePaymentStatusAtomicallyParams{
		PtUuid:              codTxn.PtUuid,
		NewStatus:           PaymentStatusPaid,
		AllowedFromStatuses: []int32{PaymentStatusPending},
	})
	if err != nil {
		return fmt.Errorf("failed to atomically update COD transaction to paid: %w", err)
	}

	var userUUIDStr string
	var orderNumberStr string
	if s.orderReader != nil {
		if ord, _ := s.orderReader.GetByUUID(ctx, orderUUID); ord != nil {
			userUUIDStr = ord.UserUUID.String()
			orderNumberStr = ord.OrderNumber
		}
	}

	s.publishPaymentEvent(ctx, rabbitmq.RoutingKeyPaymentSucceeded, dto.PaymentEventPayload{
		PaymentUUID: updated.PtUuid.String(),
		OrderUUID:   orderUUID.String(),
		OrderNumber: orderNumberStr,
		UserUUID:    userUUIDStr,
		Provider:    "cod",
		Amount:      utils.NumericToFloat(updated.PtAmount),
		Currency:    updated.PtCurrency,
		Status:      "Paid",
		Timestamp:   time.Now().UTC(),
	})

	return nil
}

func (s *paymentService) RefundPayment(ctx context.Context, input dto.RefundPaymentInput) (dto.PaymentTransactionDTO, error) {
	orderUUID, err := uuid.Parse(input.OrderUUID)
	if err != nil {
		return dto.PaymentTransactionDTO{}, fmt.Errorf("invalid order uuid: %w", err)
	}
	if input.Amount <= 0 {
		return dto.PaymentTransactionDTO{}, errors.New("refund amount must be greater than zero")
	}

	txns, err := s.paymentRepo.GetByOrderUUID(ctx, orderUUID)
	if err != nil {
		return dto.PaymentTransactionDTO{}, fmt.Errorf("failed to lookup transactions for refund: %w", err)
	}

	var paidTxn *paymentsqlc.PaymentTransaction
	for _, t := range txns {
		if t.PtStatus == PaymentStatusPaid {
			txnCopy := t
			paidTxn = &txnCopy
			break
		}
	}

	if paidTxn == nil {
		return dto.PaymentTransactionDTO{}, errors.New("cannot refund: no paid transaction found for order")
	}

	paidAmount := utils.NumericToFloat(paidTxn.PtAmount)
	if input.Amount > paidAmount {
		return dto.PaymentTransactionDTO{}, fmt.Errorf("refund amount (%.2f) exceeds original transaction amount (%.2f)", input.Amount, paidAmount)
	}

	idempotencyKey := input.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("refund:%s:%.2f", input.OrderUUID, input.Amount)
	}

	if existing, lookupErr := s.paymentRepo.GetByIdempotencyKey(ctx, idempotencyKey); lookupErr == nil {
		return dto.MapToPaymentTransactionDTO(existing, orderUUID), nil
	} else if !errors.Is(lookupErr, pgx.ErrNoRows) {
		return dto.PaymentTransactionDTO{}, fmt.Errorf("failed to check refund idempotency key: %w", lookupErr)
	}

	var refundedAmount float64
	for _, txn := range txns {
		if txn.PtRefundOf != nil && *txn.PtRefundOf == paidTxn.PtID && txn.PtStatus == PaymentStatusRefunded {
			refundedAmount += utils.NumericToFloat(txn.PtRefundAmount)
		}
	}
	if refundedAmount+input.Amount > paidAmount {
		return dto.PaymentTransactionDTO{}, fmt.Errorf(
			"total refund amount (%.2f) exceeds original transaction amount (%.2f)",
			refundedAmount+input.Amount,
			paidAmount,
		)
	}

	refundTxn, err := s.paymentRepo.CreateRefund(ctx, paymentsqlc.CreateRefundTransactionParams{
		OrderID:        paidTxn.PtOrderID,
		OrderUuid:      pgtype.UUID{Bytes: orderUUID, Valid: true},
		IdempotencyKey: idempotencyKey,
		Provider:       paidTxn.PtProvider,
		ProviderTxnID:  paidTxn.PtProviderTxnID,
		Amount:         utils.FloatToNumeric(-input.Amount),
		Currency:       paidTxn.PtCurrency,
		Status:         PaymentStatusRefunded,
		RefundOf:       &paidTxn.PtID,
		RefundAmount:   utils.FloatToNumeric(input.Amount),
	})
	if err != nil {
		return dto.PaymentTransactionDTO{}, fmt.Errorf("failed to insert refund transaction: %w", err)
	}

	s.publishPaymentEvent(ctx, "payment.refunded", dto.PaymentEventPayload{
		PaymentUUID: refundTxn.PtUuid.String(),
		OrderUUID:   input.OrderUUID,
		Provider:    paidTxn.PtProvider,
		Amount:      input.Amount,
		Currency:    paidTxn.PtCurrency,
		Status:      "Refunded",
		Reason:      input.Reason,
		Timestamp:   time.Now().UTC(),
	})

	return dto.MapToPaymentTransactionDTO(refundTxn, orderUUID), nil
}

func (s *paymentService) publishPaymentEvent(ctx context.Context, routingKey string, payload dto.PaymentEventPayload) {
	bytes, err := json.Marshal(payload)
	if err != nil {
		logger.Log.Error().Err(err).Str("routing_key", routingKey).Msg("Failed to serialize payment event payload")
		return
	}

	// 1. Transactional Outbox
	if s.outboxRepo != nil {
		_, err := s.outboxRepo.Create(ctx, "Payment", payload.PaymentUUID, routingKey, bytes)
		if err != nil {
			logger.Log.Error().Err(err).Str("routing_key", routingKey).Msg("Failed to write event to transactional outbox")
		}
	}

	// 2. RabbitMQ Topic Exchange & Direct Queue
	if s.rabbitmq != nil {
		if err := s.rabbitmq.PublishTopic(ctx, routingKey, payload); err != nil {
			logger.Log.Error().Err(err).Str("routing_key", routingKey).Msg("Failed to publish event to RabbitMQ topic exchange")
		}
		if err := s.rabbitmq.Publish(ctx, routingKey, payload); err != nil {
			logger.Log.Error().Err(err).Str("queue", routingKey).Msg("Failed to publish event to RabbitMQ direct queue")
		}
	}
}

func (s *paymentService) ReconcilePendingPayments(ctx context.Context) error {
	cutoff := time.Now().UTC().Add(-15 * time.Minute)
	pendingTxns, err := s.paymentRepo.GetPendingOlderThan(ctx, cutoff, 50)
	if err != nil {
		return fmt.Errorf("failed to fetch pending transactions for reconciliation: %w", err)
	}

	for _, txn := range pendingTxns {
		// Ask the provider first. An IPN that never reached us (tunnel down,
		// portal not configured, network hiccup) still leaves the money taken,
		// so expiring without a status query silently loses paid orders.
		if s.providerConfirmsPayment(ctx, txn) {
			continue
		}

		failStatus := PaymentStatusExpired
		_, err := s.paymentRepo.UpdateStatusAtomically(ctx, paymentsqlc.UpdatePaymentStatusAtomicallyParams{
			PtUuid:              txn.PtUuid,
			NewStatus:           failStatus,
			AllowedFromStatuses: []int32{PaymentStatusPending},
		})
		if err != nil {
			logger.Log.Warn().Err(err).Str("pt_uuid", txn.PtUuid.String()).Msg("Failed to expire pending transaction")
			continue
		}

		var orderUUIDStr string
		if txn.PtOrderUuid.Valid {
			orderUUIDStr = uuid.UUID(txn.PtOrderUuid.Bytes).String()
		}

		s.publishPaymentEvent(ctx, rabbitmq.RoutingKeyPaymentExpired, dto.PaymentEventPayload{
			PaymentUUID: txn.PtUuid.String(),
			OrderUUID:   orderUUIDStr,
			Provider:    txn.PtProvider,
			Amount:      utils.NumericToFloat(txn.PtAmount),
			Currency:    txn.PtCurrency,
			Status:      "Expired",
			Reason:      "Transaction timed out after 15 minutes",
			Timestamp:   time.Now().UTC(),
		})
	}

	return nil
}

// providerConfirmsPayment queries the payment gateway for the authoritative
// status of a pending transaction and finalizes it when the gateway reports a
// successful payment. Returns true when the transaction was settled (so the
// caller must not expire it). Query failures count as "not settled": the
// gateway is the source of truth but an inconclusive answer must not block
// expiry of genuinely abandoned checkouts.
func (s *paymentService) providerConfirmsPayment(ctx context.Context, txn paymentsqlc.PaymentTransaction) bool {
	pvd, exists := s.providers[txn.PtProvider]
	if !exists || txn.PtTxnRef == nil || *txn.PtTxnRef == "" {
		return false
	}

	transactionDate := ""
	if !txn.PtCreatedAt.IsZero() {
		transactionDate = txn.PtCreatedAt.In(time.FixedZone("Asia/Ho_Chi_Minh", 7*3600)).Format("20060102150405")
	}
	status, err := pvd.QueryTransactionStatus(ctx, *txn.PtTxnRef, transactionDate)
	if err != nil {
		logger.Log.Warn().Err(err).
			Str("pt_uuid", txn.PtUuid.String()).
			Str("provider", txn.PtProvider).
			Msg("Payment status query failed during reconciliation; leaving transaction pending for the next run")
		// Inconclusive gateway answer: keep the transaction pending and retry
		// on the next tick rather than expiring a payment we cannot verify.
		// Only transactions whose lookup is impossible (below) are expired.
		return true
	}
	if status != provider.TxnStatusSuccess {
		return false
	}

	outcome, finalizeErr := s.finalizeTransaction(ctx, txn, true, 0, "", map[string]string{
		"source": "reconciliation",
		"status": string(status),
	}, 0)
	if finalizeErr != nil {
		logger.Log.Error().Err(finalizeErr).
			Str("pt_uuid", txn.PtUuid.String()).
			Int("outcome", int(outcome)).
			Msg("Failed to finalize gateway-confirmed payment during reconciliation")
		return false
	}
	logger.Log.Info().
		Str("pt_uuid", txn.PtUuid.String()).
		Str("provider", txn.PtProvider).
		Msg("Reconciliation confirmed a paid transaction the webhook never delivered")
	return true
}
