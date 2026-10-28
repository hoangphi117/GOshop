package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"goshop/pkg/db"
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

	txn, err := s.resolveTransaction(ctx, providerName, whResp.ProviderTxnID, whResp.OrderNumber, "")
	if err != nil {
		return WebhookOrderNotFound, err
	}

	return s.settlePayment(ctx, settleRequest{
		txn:           txn,
		success:       provider.TransactionStatus(whResp.Status) == provider.TxnStatusSuccess,
		amount:        whResp.Amount,
		providerTxnID: whResp.ProviderTxnID,
		metadata:      whResp.Metadata,
		insertEvent: &paymentsqlc.InsertWebhookEventParams{
			WeProvider:        providerName,
			WeExternalEventID: eventID,
			WeEventType:       string(whResp.Status),
			WeRawBody:         string(rawPayload),
		},
	})
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
	_, err = s.settlePayment(ctx, settleRequest{
		txn:             txn,
		success:         msg.Status == string(provider.TxnStatusSuccess),
		amount:          money.FromMinorUnits(msg.Amount),
		providerTxnID:   msg.ProviderTxnID,
		metadata:        msg.Metadata,
		existingEventID: msg.WebhookEventID,
	})
	return err
}

// settleRequest carries everything the settlement needs. The target transaction
// is resolved before the transaction opens; the CAS (AllowedFromStatuses) makes
// a concurrent settle between the read and the write a no-op, not a corruption.
type settleRequest struct {
	txn             paymentsqlc.PaymentTransaction
	success         bool
	amount          money.Amount
	providerTxnID   string
	metadata        map[string]string
	insertEvent     *paymentsqlc.InsertWebhookEventParams // nil = no audit row (reconciliation)
	existingEventID int32                                 // set when the event row already exists
}

// settlePayment performs the settlement of a payment callback inside ONE
// database transaction: the webhook audit row, the payment transition and the
// outbox message commit or roll back together. RabbitMQ publishes only after
// commit, so a failed commit can never emit an event for a payment that did
// not settle.
func (s *paymentService) settlePayment(ctx context.Context, req settleRequest) (WebhookOutcome, error) {
	var (
		outcome    WebhookOutcome
		routingKey string
		evt        *dto.PaymentEventPayload
	)
	exec := func(txCtx context.Context) error {
		o, r, e, err := s.settleInTx(txCtx, req)
		outcome, routingKey, evt = o, r, e
		return err
	}

	if s.txManager != nil && db.DBPool != nil {
		if err := s.txManager.WithTx(ctx, exec); err != nil {
			return WebhookUnknown, err
		}
	} else if err := exec(ctx); err != nil {
		return WebhookUnknown, err
	}

	if evt != nil {
		s.publishRabbitEvent(ctx, routingKey, *evt)
	}
	return outcome, nil
}

// settleInTx runs the transactional half of settlement. Every repository call
// resolves the tx from ctx (see repository.txQuerier). A returned error rolls
// the whole settlement back, including the audit row: the next provider retry
// then replays the full flow instead of being mistaken for a duplicate.
func (s *paymentService) settleInTx(ctx context.Context, req settleRequest) (WebhookOutcome, string, *dto.PaymentEventPayload, error) {
	webhookEventID := req.existingEventID
	if req.insertEvent != nil {
		we, err := s.webhookRepo.Insert(ctx, *req.insertEvent)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// Same (provider, external_event_id) already recorded: never
				// settle twice off one callback.
				logger.Log.Info().Str("event_id", req.insertEvent.WeExternalEventID).Msg("Duplicate webhook event received, skipping")
				return WebhookAlreadyProcessed, "", nil, nil
			}
			return WebhookUnknown, "", nil, fmt.Errorf("failed to persist webhook event: %w", err)
		}
		webhookEventID = we.WeID
	}

	outcome, routingKey, evt, err := s.finalizeSettlement(ctx, req, webhookEventID)
	if err != nil {
		return WebhookUnknown, "", nil, err
	}

	// The audit row is written before the transition (so a collision can be
	// detected), which leaves we_processed = false. Mark it here, inside the
	// same transaction: a settled delivery must be recorded as processed, and
	// a failed settlement rolls the mark back with everything else. The
	// already-finalized branch marks its own row before returning nil evt.
	if evt != nil && webhookEventID != 0 && s.webhookRepo != nil {
		if markErr := s.webhookRepo.MarkProcessed(ctx, webhookEventID); markErr != nil {
			return WebhookUnknown, "", nil, fmt.Errorf("failed to mark webhook event as processed: %w", markErr)
		}
	}

	if evt != nil && s.outboxRepo != nil {
		bytes, marshalErr := json.Marshal(evt)
		if marshalErr != nil {
			return WebhookUnknown, "", nil, fmt.Errorf("failed to serialize payment settlement event: %w", marshalErr)
		}
		if _, err := s.outboxRepo.Create(ctx, "Payment", evt.PaymentUUID, routingKey, bytes); err != nil {
			return WebhookUnknown, "", nil, fmt.Errorf("failed to write settlement to outbox: %w", err)
		}
	}
	return outcome, routingKey, evt, nil
}

// finalizeSettlement flips a pending transaction to a terminal status and
// builds the payment event payload. Must run inside the settlement transaction.
func (s *paymentService) finalizeSettlement(
	ctx context.Context,
	req settleRequest,
	webhookEventID int32,
) (WebhookOutcome, string, *dto.PaymentEventPayload, error) {
	var targetStatus int32
	var routingKey string

	if req.success {
		// Never confirm a payment that does not cover the order: a tampered or
		// partial callback must not settle the transaction. A zero amount means
		// "not reported by the source", which only reconciliation uses. The
		// comparison is exact integer arithmetic (the old float+0.01 epsilon is
		// gone): no rounding can slide an under-payment through.
		expected, err := money.NumericToAmount(req.txn.PtAmount)
		if err != nil {
			return WebhookUnknown, "", nil, fmt.Errorf("transaction %s has an unreadable amount: %w", req.txn.PtUuid, err)
		}
		if req.amount > 0 && !req.amount.Covers(expected) {
			logger.Log.Warn().
				Str("pt_uuid", req.txn.PtUuid.String()).
				Str("expected", expected.String()).
				Str("received", req.amount.String()).
				Msg("Rejecting payment confirmation with an uncovered amount")
			return WebhookAmountMismatch, "", nil, fmt.Errorf("%w: %s does not cover %s", ErrWebhookAmountMismatch, req.amount, expected)
		}
		targetStatus = PaymentStatusPaid
		routingKey = rabbitmq.RoutingKeyPaymentSucceeded
	} else {
		targetStatus = PaymentStatusFailed
		routingKey = rabbitmq.RoutingKeyPaymentFailed
	}

	metadataBytes := []byte("{}")
	if req.metadata != nil {
		b, marshalErr := json.Marshal(req.metadata)
		if marshalErr != nil {
			logger.Log.Warn().Err(marshalErr).Msg("Failed to serialize webhook metadata")
		} else {
			metadataBytes = b
		}
	}

	updatedTxn, err := s.paymentRepo.UpdateStatusAtomically(ctx, paymentsqlc.UpdatePaymentStatusAtomicallyParams{
		PtUuid:              req.txn.PtUuid,
		NewStatus:           targetStatus,
		ProviderTxnID:       &req.providerTxnID,
		Metadata:            metadataBytes,
		AllowedFromStatuses: []int32{PaymentStatusPending},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Another writer settled it between our read and write. Keep the
			// audit row (marked) and answer the provider "already confirmed".
			if webhookEventID != 0 && s.webhookRepo != nil {
				if markErr := s.webhookRepo.MarkProcessed(ctx, webhookEventID); markErr != nil {
					logger.Log.Warn().Err(markErr).Msg("Failed to mark webhook event as processed")
				}
			}
			logger.Log.Info().Str("pt_uuid", req.txn.PtUuid.String()).Msg("Payment already finalized, idempotent ignore")
			return WebhookAlreadyProcessed, "", nil, nil
		}
		return WebhookUnknown, "", nil, fmt.Errorf("failed to atomically transition payment status: %w", err)
	}

	orderUUIDStr, userUUIDStr := s.resolveOrderAndUser(ctx, req.txn)

	var orderNumberStr string
	if req.txn.PtTxnRef != nil {
		orderNumberStr = *req.txn.PtTxnRef
	}

	eventAmount, err := money.NumericToAmount(updatedTxn.PtAmount)
	if err != nil {
		return WebhookUnknown, "", nil, fmt.Errorf("settled transaction %s carries an unreadable amount: %w", updatedTxn.PtUuid, err)
	}

	return WebhookProcessed, routingKey, &dto.PaymentEventPayload{
		PaymentUUID:   updatedTxn.PtUuid.String(),
		OrderUUID:     orderUUIDStr,
		OrderNumber:   orderNumberStr,
		UserUUID:      userUUIDStr,
		Provider:      req.txn.PtProvider,
		ProviderTxnID: req.providerTxnID,
		Amount:        eventAmount.ToFloat(),
		Currency:      updatedTxn.PtCurrency,
		Status:        dto.MapPaymentTxnStatusToString(targetStatus),
		Timestamp:     time.Now().UTC(),
		Metadata:      metadataBytes,
	}, nil
}

// resolveOrderAndUser resolves the order/user UUIDs for the event payload via
// the order reader. Called before the settlement transaction opens; a failure
// degrades to an under-filled payload, never blocks settlement.
func (s *paymentService) resolveOrderAndUser(ctx context.Context, txn paymentsqlc.PaymentTransaction) (orderUUIDStr, userUUIDStr string) {
	if txn.PtOrderUuid.Valid {
		orderUUIDStr = uuid.UUID(txn.PtOrderUuid.Bytes).String()
	}
	if s.orderReader != nil && orderUUIDStr != "" {
		if parsed, err := uuid.Parse(orderUUIDStr); err == nil {
			if ord, _ := s.orderReader.GetByUUID(ctx, parsed); ord != nil {
				userUUIDStr = ord.UserUUID.String()
			}
		}
	}
	return orderUUIDStr, userUUIDStr
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
