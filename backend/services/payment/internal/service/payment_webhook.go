package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"goshop/pkg/logger"
	"goshop/pkg/rabbitmq"
	paymentsqlc "goshop/services/payment/internal/db/sqlc"
	"goshop/services/payment/internal/dto"
	"goshop/services/payment/internal/provider"
	"goshop/services/payment/money"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// WebhookOutcome reports what a webhook achieved so the HTTP handler can answer
// the provider in its own protocol (VNPay expects RspCode/Message codes). Kept
// provider-agnostic on purpose.
type WebhookOutcome int

const (
	WebhookProcessed WebhookOutcome = iota
	WebhookAlreadyProcessed
	WebhookOrderNotFound
	WebhookAmountMismatch
	WebhookUnknown
)

// Sentinel webhook errors the HTTP handler maps to provider-specific response
// codes (e.g. VNPay RspCode 01/04/97).
var (
	ErrPaymentTxnNotFound    = errors.New("payment transaction not found for provider callback")
	ErrWebhookSignature      = errors.New("webhook signature verification failed")
	ErrWebhookAmountMismatch = errors.New("payment amount does not cover the order total")
)

func (s *paymentService) HandleWebhook(ctx context.Context, providerName string, rawPayload []byte, headers map[string]string) (WebhookOutcome, error) {
	pvd, exists := s.providers[providerName]
	if !exists {
		return WebhookUnknown, fmt.Errorf("unsupported webhook provider: %s", providerName)
	}

	if err := pvd.VerifyWebhook(rawPayload, headers); err != nil {
		return WebhookUnknown, fmt.Errorf("%w: %v", ErrWebhookSignature, err)
	}

	whResp, err := pvd.ParseWebhookPayload(rawPayload)
	if err != nil {
		return WebhookUnknown, fmt.Errorf("failed to parse webhook payload: %w", err)
	}

	eventID := whResp.EventID
	if eventID == "" {
		eventID = fmt.Sprintf("%s:%s", providerName, whResp.OrderNumber)
		if whResp.ProviderTxnID != "" {
			eventID = fmt.Sprintf("%s:%s", providerName, whResp.ProviderTxnID)
		}
	}

	webhookEvent, err := s.webhookRepo.Insert(ctx, paymentsqlc.InsertWebhookEventParams{
		WeProvider:        providerName,
		WeExternalEventID: eventID,
		WeEventType:       string(whResp.Status),
		WeRawBody:         string(rawPayload),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Log.Info().Str("event_id", eventID).Msg("Duplicate webhook event received, skipping")
			return WebhookAlreadyProcessed, nil
		}
		return WebhookUnknown, fmt.Errorf("failed to persist webhook event: %w", err)
	}

	txn, err := s.resolveTransaction(ctx, providerName, whResp.ProviderTxnID, whResp.OrderNumber, "")
	if err != nil {
		return WebhookOrderNotFound, err
	}

	outcome, err := s.finalizeTransaction(ctx, txn,
		provider.TransactionStatus(whResp.Status) == provider.TxnStatusSuccess,
		whResp.Amount, whResp.ProviderTxnID, whResp.Metadata, webhookEvent.WeID)
	if err == nil {
		if markErr := s.webhookRepo.MarkProcessed(ctx, webhookEvent.WeID); markErr != nil {
			logger.Log.Error().Err(markErr).Int32("webhook_event_id", webhookEvent.WeID).Msg("Failed to mark webhook event as processed")
			return WebhookUnknown, fmt.Errorf("failed to mark webhook event processed: %w", markErr)
		}
	}
	return outcome, err
}

// ProcessWebhook applies an already-persisted webhook message. The HTTP route
// uses HandleWebhook; this stays for programmatic callers.
func (s *paymentService) ProcessWebhook(ctx context.Context, msg dto.PaymentWebhookMessage) error {
	if msg.Amount < 0 {
		return fmt.Errorf("webhook amount must not be negative: %d", msg.Amount)
	}
	txn, err := s.resolveTransaction(ctx, msg.Provider, msg.ProviderTxnID, msg.OrderNumber, msg.OrderUUID)
	if err != nil {
		return err
	}
	_, err = s.finalizeTransaction(ctx, txn,
		msg.Status == string(provider.TxnStatusSuccess),
		money.FromMinorUnits(msg.Amount), msg.ProviderTxnID, msg.Metadata, msg.WebhookEventID)
	return err
}

// errPaymentTxnNotFound marks a callback that no stored transaction matches.
var errPaymentTxnNotFound = ErrPaymentTxnNotFound

func (s *paymentService) resolveTransaction(ctx context.Context, providerName, providerTxnID, orderNumber, orderUUID string) (paymentsqlc.PaymentTransaction, error) {
	var txn paymentsqlc.PaymentTransaction
	var err error

	if providerTxnID != "" {
		txn, err = s.paymentRepo.GetByProviderTxnID(ctx, providerName, providerTxnID)
	}
	if err != nil || txn.PtID == 0 {
		txn, err = s.paymentRepo.GetByTxnRef(ctx, providerName, orderNumber)
	}
	if (err != nil || txn.PtID == 0) && orderUUID != "" {
		if parsed, parseErr := uuid.Parse(orderUUID); parseErr == nil {
			txns, listErr := s.paymentRepo.GetByOrderUUID(ctx, parsed)
			if listErr == nil && len(txns) > 0 {
				return txns[0], nil
			}
		}
	}
	if err != nil || txn.PtID == 0 {
		return paymentsqlc.PaymentTransaction{}, fmt.Errorf("%w (provider=%s txn_ref=%s): %w", errPaymentTxnNotFound, providerName, orderNumber, err)
	}
	return txn, nil
}

// finalizeTransaction flips a pending transaction to a terminal status and
// publishes the payment event. Webhook handling and reconciliation both funnel
// through here, so a payment confirmed by either route produces identical side
// effects and identical idempotency. providerTxnID and metadata are optional.
func (s *paymentService) finalizeTransaction(
	ctx context.Context,
	txn paymentsqlc.PaymentTransaction,
	success bool,
	amount money.Amount,
	providerTxnID string,
	metadata map[string]string,
	webhookEventID int32,
) (WebhookOutcome, error) {
	var targetStatus int32
	var routingKey string

	if success {
		// Never confirm a payment that does not cover the order: a tampered or
		// partial callback must not settle the transaction. A zero amount means
		// "not reported by the source", which only reconciliation uses. The
		// comparison is exact integer arithmetic (the old float+0.01 epsilon is
		// gone): no rounding can slide an under-payment through.
		expected, err := money.NumericToAmount(txn.PtAmount)
		if err != nil {
			return WebhookUnknown, fmt.Errorf("transaction %s has an unreadable amount: %w", txn.PtUuid, err)
		}
		if amount > 0 && !amount.Covers(expected) {
			logger.Log.Warn().
				Str("pt_uuid", txn.PtUuid.String()).
				Str("expected", expected.String()).
				Str("received", amount.String()).
				Msg("Rejecting payment confirmation with an uncovered amount")
			if webhookEventID != 0 {
				if markErr := s.webhookRepo.MarkProcessed(ctx, webhookEventID); markErr != nil {
					logger.Log.Warn().Err(markErr).Msg("Failed to mark webhook event as processed")
				}
			}
			return WebhookAmountMismatch, fmt.Errorf("%w: %s does not cover %s", ErrWebhookAmountMismatch, amount, expected)
		}
		targetStatus = PaymentStatusPaid
		routingKey = rabbitmq.RoutingKeyPaymentSucceeded
	} else {
		targetStatus = PaymentStatusFailed
		routingKey = rabbitmq.RoutingKeyPaymentFailed
	}

	metadataBytes := []byte("{}")
	if metadata != nil {
		b, marshalErr := json.Marshal(metadata)
		if marshalErr != nil {
			logger.Log.Warn().Err(marshalErr).Msg("Failed to serialize webhook metadata")
		} else {
			metadataBytes = b
		}
	}

	updatedTxn, err := s.paymentRepo.UpdateStatusAtomically(ctx, paymentsqlc.UpdatePaymentStatusAtomicallyParams{
		PtUuid:              txn.PtUuid,
		NewStatus:           targetStatus,
		ProviderTxnID:       &providerTxnID,
		Metadata:            metadataBytes,
		AllowedFromStatuses: []int32{PaymentStatusPending},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Another writer settled it between our read and write.
			logger.Log.Info().Str("pt_uuid", txn.PtUuid.String()).Msg("Payment already finalized, idempotent ignore")
			if webhookEventID != 0 {
				if markErr := s.webhookRepo.MarkProcessed(ctx, webhookEventID); markErr != nil {
					logger.Log.Warn().Err(markErr).Msg("Failed to mark webhook event as processed")
				}
			}
			return WebhookAlreadyProcessed, nil
		}
		return WebhookUnknown, fmt.Errorf("failed to atomically transition payment status: %w", err)
	}

	var orderUUIDStr string
	var userUUIDStr string
	if updatedTxn.PtOrderUuid.Valid {
		orderUUIDStr = uuid.UUID(updatedTxn.PtOrderUuid.Bytes).String()
	} else if txn.PtOrderUuid.Valid {
		orderUUIDStr = uuid.UUID(txn.PtOrderUuid.Bytes).String()
	}

	if s.orderReader != nil {
		var ord *dto.PaymentOrderSummary
		if orderUUIDStr != "" {
			if parsedUUID, err := uuid.Parse(orderUUIDStr); err == nil {
				ord, _ = s.orderReader.GetByUUID(ctx, parsedUUID)
			}
		}
		if ord != nil {
			if orderUUIDStr == "" {
				orderUUIDStr = ord.OrderUUID.String()
			}
			userUUIDStr = ord.UserUUID.String()
		}
	}

	var orderNumberStr string
	if txn.PtTxnRef != nil {
		orderNumberStr = *txn.PtTxnRef
	}

	eventAmount, err := money.NumericToAmount(updatedTxn.PtAmount)
	if err != nil {
		return WebhookUnknown, fmt.Errorf("settled transaction %s carries an unreadable amount: %w", updatedTxn.PtUuid, err)
	}

	s.publishPaymentEvent(ctx, routingKey, dto.PaymentEventPayload{
		PaymentUUID:   updatedTxn.PtUuid.String(),
		OrderUUID:     orderUUIDStr,
		OrderNumber:   orderNumberStr,
		UserUUID:      userUUIDStr,
		Provider:      txn.PtProvider,
		ProviderTxnID: providerTxnID,
		Amount:        eventAmount.ToFloat(),
		Currency:      updatedTxn.PtCurrency,
		Status:        dto.MapPaymentTxnStatusToString(targetStatus),
		Timestamp:     time.Now().UTC(),
		Metadata:      metadataBytes,
	})

	return WebhookProcessed, nil
}
