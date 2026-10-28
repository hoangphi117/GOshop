package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"goshop/pkg/cache"
	"goshop/pkg/db"
	"goshop/pkg/logger"
	"goshop/pkg/rabbitmq"
	paymentsqlc "goshop/services/payment/internal/db/sqlc"
	"goshop/services/payment/internal/dto"
	"goshop/services/payment/internal/provider"
	"goshop/services/payment/internal/repository"
	"goshop/services/payment/money"

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
	txManager      db.TxManager
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
		txManager:      db.NewTxManager(db.DBPool),
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

	// The order service still speaks its total as a gRPC double. Convert once,
	// here, and use integer minor units for everything downstream.
	totalAmount, err := money.FromFloat(order.TotalAmount)
	if err != nil {
		return dto.PaymentIntentDTO{}, fmt.Errorf("order %s has an amount that cannot be charged exactly: %w", input.OrderUUID, err)
	}
	if totalAmount <= 0 {
		return dto.PaymentIntentDTO{}, fmt.Errorf("order %s has a non-positive total amount", input.OrderUUID)
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
			Amount:         totalAmount.Numeric(),
			Currency:       "VND",
			Status:         PaymentStatusPending,
		})
		if err != nil {
			return dto.PaymentIntentDTO{}, fmt.Errorf("failed to create COD transaction: %w", err)
		}

		return dto.MapToPaymentIntentDTO(txn, order.OrderUUID)
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
			return dto.MapToPaymentIntentDTO(existingTxn, order.OrderUUID)
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
		Amount:         totalAmount.Numeric(),
		Currency:       "VND",
		Status:         PaymentStatusPending,
	})
	if err != nil {
		return dto.PaymentIntentDTO{}, fmt.Errorf("failed to create pending transaction: %w", err)
	}

	webhookURL := fmt.Sprintf("%s/api/v1/payments/webhook/%s", s.webhookBaseURL, input.Provider)
	resp, err := pvd.CreatePaymentURL(ctx, provider.CreatePaymentRequest{
		OrderNumber: order.OrderNumber,
		Amount:      totalAmount,
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

	return dto.MapToPaymentIntentDTO(updatedTxn, order.OrderUUID)
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
		mapped, mapErr := dto.MapToPaymentTransactionDTO(t, orderUUID)
		if mapErr != nil {
			return nil, fmt.Errorf("failed to map transaction %s: %w", t.PtUuid, mapErr)
		}
		result[i] = mapped
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
		mapped, mapErr := dto.MapToPaymentTransactionDTO(t, orderUUID)
		if mapErr != nil {
			return nil, fmt.Errorf("failed to map transaction %s: %w", t.PtUuid, mapErr)
		}
		result[i] = mapped
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

	codAmount, err := money.NumericToAmount(updated.PtAmount)
	if err != nil {
		return fmt.Errorf("COD transaction %s has an unreadable amount: %w", updated.PtUuid, err)
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
		Amount:      codAmount.ToFloat(),
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
	// The API speaks minor units; anything non-positive is rejected before it
	// can reach a comparison.
	refundAmount := money.FromMinorUnits(input.Amount)
	if input.Amount <= 0 {
		return dto.PaymentTransactionDTO{}, errors.New("refund amount must be greater than zero")
	}
	if input.Amount > int64(money.MaxAmount) {
		return dto.PaymentTransactionDTO{}, fmt.Errorf("refund amount %s exceeds the supported maximum %s", refundAmount, money.MaxAmount)
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

	paidAmount, err := money.NumericToAmount(paidTxn.PtAmount)
	if err != nil {
		return dto.PaymentTransactionDTO{}, fmt.Errorf("paid transaction %s has an unreadable amount: %w", paidTxn.PtUuid, err)
	}
	if refundAmount.Exceeds(paidAmount) {
		return dto.PaymentTransactionDTO{}, fmt.Errorf("refund amount (%s) exceeds original transaction amount (%s)", refundAmount, paidAmount)
	}

	idempotencyKey := input.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("refund:%s:%d", input.OrderUUID, input.Amount)
	}

	if existing, lookupErr := s.paymentRepo.GetByIdempotencyKey(ctx, idempotencyKey); lookupErr == nil {
		return dto.MapToPaymentTransactionDTO(existing, orderUUID)
	} else if !errors.Is(lookupErr, pgx.ErrNoRows) {
		return dto.PaymentTransactionDTO{}, fmt.Errorf("failed to check refund idempotency key: %w", lookupErr)
	}

	// Cumulative refund ceiling: integer addition of already-issued refunds.
	var refundedAmount money.Amount
	for _, txn := range txns {
		if txn.PtRefundOf != nil && *txn.PtRefundOf == paidTxn.PtID && txn.PtStatus == PaymentStatusRefunded {
			alreadyRefunded, readErr := money.NumericToAmount(txn.PtRefundAmount)
			if readErr != nil {
				return dto.PaymentTransactionDTO{}, fmt.Errorf("refund %s of order %s has an unreadable amount: %w", txn.PtUuid, input.OrderUUID, readErr)
			}
			refundedAmount, err = refundedAmount.Add(alreadyRefunded)
			if err != nil {
				return dto.PaymentTransactionDTO{}, fmt.Errorf("failed to total refunded amount for order %s: %w", input.OrderUUID, err)
			}
		}
	}
	claimed, err := refundedAmount.Add(refundAmount)
	if err != nil {
		return dto.PaymentTransactionDTO{}, fmt.Errorf("failed to total refunded amount for order %s: %w", input.OrderUUID, err)
	}
	if claimed.Exceeds(paidAmount) {
		return dto.PaymentTransactionDTO{}, fmt.Errorf(
			"total refund amount (%s) exceeds original transaction amount (%s)",
			claimed,
			paidAmount,
		)
	}

	// Ask the gateway BEFORE recording anything. Writing a refund row for money
	// that never moved silently corrupts the ledger, so a gateway failure (or a
	// provider without refund support) aborts here and the caller must fall back
	// to an explicit manual bank transfer. COD never goes through a gateway.
	pvd, hasGateway := s.providers[paidTxn.PtProvider]
	if paidTxn.PtProvider == "cod" || !hasGateway {
		return dto.PaymentTransactionDTO{}, fmt.Errorf("refund not supported for provider %q: use a manual bank transfer", paidTxn.PtProvider)
	}

	orderNumber := ""
	if paidTxn.PtTxnRef != nil {
		orderNumber = *paidTxn.PtTxnRef
	}
	providerTxnID := ""
	if paidTxn.PtProviderTxnID != nil {
		providerTxnID = *paidTxn.PtProviderTxnID
	}
	if providerTxnID == "" {
		return dto.PaymentTransactionDTO{}, fmt.Errorf("cannot refund: payment has no provider transaction id: use a manual bank transfer")
	}

	reason := input.Reason
	if reason == "" {
		reason = "Refund requested by admin"
	}
	transactionDate := paidTxn.PtCreatedAt.In(time.FixedZone("Asia/Ho_Chi_Minh", 7*3600)).Format("20060102150405")
	refundResp, refundErr := pvd.Refund(ctx, provider.RefundRequest{
		OrderNumber:     orderNumber,
		ProviderTxnID:   providerTxnID,
		Amount:          refundAmount,
		TotalAmount:     paidAmount,
		Reason:          reason,
		TransactionDate: transactionDate,
		CreatedBy:       "goshop-admin",
	})
	if refundErr != nil {
		return dto.PaymentTransactionDTO{}, fmt.Errorf("gateway refund failed: %w", refundErr)
	}
	if refundResp.Status != provider.TxnStatusSuccess {
		return dto.PaymentTransactionDTO{}, fmt.Errorf("gateway refund rejected: %s", refundResp.Message)
	}

	refundTxn, err := s.paymentRepo.CreateRefund(ctx, paymentsqlc.CreateRefundTransactionParams{
		OrderID:        paidTxn.PtOrderID,
		OrderUuid:      pgtype.UUID{Bytes: orderUUID, Valid: true},
		IdempotencyKey: idempotencyKey,
		Provider:       paidTxn.PtProvider,
		ProviderTxnID:  &refundResp.RefundTxnID,
		Amount:         refundAmount.Neg().Numeric(),
		Currency:       paidTxn.PtCurrency,
		Status:         PaymentStatusRefunded,
		RefundOf:       &paidTxn.PtID,
		RefundAmount:   refundAmount.Numeric(),
	})
	if err != nil {
		return dto.PaymentTransactionDTO{}, fmt.Errorf("failed to insert refund transaction: %w", err)
	}

	s.publishPaymentEvent(ctx, "payment.refunded", dto.PaymentEventPayload{
		PaymentUUID: refundTxn.PtUuid.String(),
		OrderUUID:   input.OrderUUID,
		Provider:    paidTxn.PtProvider,
		Amount:      refundAmount.ToFloat(),
		Currency:    paidTxn.PtCurrency,
		Status:      "Refunded",
		Reason:      input.Reason,
		Timestamp:   time.Now().UTC(),
	})

	return dto.MapToPaymentTransactionDTO(refundTxn, orderUUID)
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
	s.publishRabbitEvent(ctx, routingKey, payload)
}

// publishRabbitEvent is the broker half of publishPaymentEvent. Settlements call
// it directly AFTER their transaction commits; the outbox row written inside the
// transaction already guarantees redelivery.
func (s *paymentService) publishRabbitEvent(ctx context.Context, routingKey string, payload dto.PaymentEventPayload) {
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

		expiredAmount, amountErr := money.NumericToAmount(txn.PtAmount)
		if amountErr != nil {
			logger.Log.Error().Err(amountErr).Str("pt_uuid", txn.PtUuid.String()).
				Msg("Expired transaction carries an unreadable amount; publishing expiry without its amount")
			expiredAmount = money.FromMinorUnits(0)
		}

		s.publishPaymentEvent(ctx, rabbitmq.RoutingKeyPaymentExpired, dto.PaymentEventPayload{
			PaymentUUID: txn.PtUuid.String(),
			OrderUUID:   orderUUIDStr,
			Provider:    txn.PtProvider,
			Amount:      expiredAmount.ToFloat(),
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

	outcome, finalizeErr := s.settlePayment(ctx, settleRequest{
		txn:     txn,
		success: true,
		metadata: map[string]string{
			"source": "reconciliation",
			"status": string(status),
		},
	})
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
