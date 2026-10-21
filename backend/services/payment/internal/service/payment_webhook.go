package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"goshop/pkg/logger"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/utils"
	paymentsqlc "goshop/services/payment/internal/db/sqlc"
	"goshop/services/payment/internal/dto"
	"goshop/services/payment/internal/provider"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *paymentService) HandleWebhook(ctx context.Context, providerName string, rawPayload []byte, headers map[string]string) error {
	provider, exists := s.providers[providerName]
	if !exists {
		return fmt.Errorf("unsupported webhook provider: %s", providerName)
	}

	if err := provider.VerifyWebhook(rawPayload, headers); err != nil {
		return fmt.Errorf("webhook signature verification failed: %w", err)
	}

	whResp, err := provider.ParseWebhookPayload(rawPayload)
	if err != nil {
		return fmt.Errorf("failed to parse webhook payload: %w", err)
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
			return nil
		}
		return fmt.Errorf("failed to persist webhook event: %w", err)
	}

	msg := dto.PaymentWebhookMessage{
		WebhookEventID: webhookEvent.WeID,
		Provider:       providerName,
		EventType:      string(whResp.Status),
		ProviderTxnID:  whResp.ProviderTxnID,
		OrderNumber:    whResp.OrderNumber,
		Amount:         whResp.Amount,
		Status:         string(whResp.Status),
		Metadata:       whResp.Metadata,
	}

	if err := s.ProcessWebhook(ctx, msg); err != nil {
		return fmt.Errorf("failed to process webhook message: %w", err)
	}

	return nil
}

func (s *paymentService) ProcessWebhook(ctx context.Context, msg dto.PaymentWebhookMessage) error {
	var txn paymentsqlc.PaymentTransaction
	var err error

	// Try lookup by ProviderTxnID first, fallback to TxnRef or OrderUUID
	if msg.ProviderTxnID != "" {
		txn, err = s.paymentRepo.GetByProviderTxnID(ctx, msg.Provider, msg.ProviderTxnID)
	}
	if err != nil || txn.PtID == 0 {
		txn, err = s.paymentRepo.GetByTxnRef(ctx, msg.Provider, msg.OrderNumber)
	}

	if err != nil || txn.PtID == 0 {
		if msg.OrderUUID != "" {
			if orderUUID, parseErr := uuid.Parse(msg.OrderUUID); parseErr == nil {
				txns, listErr := s.paymentRepo.GetByOrderUUID(ctx, orderUUID)
				if listErr == nil && len(txns) > 0 {
					txn = txns[0]
					err = nil
				}
			}
		}
	}

	if err != nil || txn.PtID == 0 {
		return fmt.Errorf("could not find matching payment transaction for webhook: %w", err)
	}

	var targetStatus int32
	var routingKey string
	if msg.Status == string(provider.TxnStatusSuccess) {
		targetStatus = PaymentStatusPaid
		routingKey = rabbitmq.RoutingKeyPaymentSucceeded
	} else {
		targetStatus = PaymentStatusFailed
		routingKey = rabbitmq.RoutingKeyPaymentFailed
	}

	metadataBytes, marshalErr := json.Marshal(msg.Metadata)
	if marshalErr != nil {
		logger.Log.Warn().Err(marshalErr).Msg("Failed to serialize webhook metadata")
		metadataBytes = []byte("{}")
	}

	updatedTxn, err := s.paymentRepo.UpdateStatusAtomically(ctx, paymentsqlc.UpdatePaymentStatusAtomicallyParams{
		PtUuid:              txn.PtUuid,
		NewStatus:           targetStatus,
		ProviderTxnID:       &msg.ProviderTxnID,
		Metadata:            metadataBytes,
		AllowedFromStatuses: []int32{PaymentStatusPending},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Log.Info().Str("pt_uuid", txn.PtUuid.String()).Msg("Payment already finalized, idempotent ignore")
			if markErr := s.webhookRepo.MarkProcessed(ctx, msg.WebhookEventID); markErr != nil {
				logger.Log.Warn().Err(markErr).Msg("Failed to mark webhook event as processed")
			}
			return nil
		}
		return fmt.Errorf("failed to atomically transition payment status: %w", err)
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

	s.publishPaymentEvent(ctx, routingKey, dto.PaymentEventPayload{
		PaymentUUID:   updatedTxn.PtUuid.String(),
		OrderUUID:     orderUUIDStr,
		OrderNumber:   msg.OrderNumber,
		UserUUID:      userUUIDStr,
		Provider:      msg.Provider,
		ProviderTxnID: msg.ProviderTxnID,
		Amount:        utils.NumericToFloat(updatedTxn.PtAmount),
		Currency:      updatedTxn.PtCurrency,
		Status:        dto.MapPaymentTxnStatusToString(targetStatus),
		Timestamp:     time.Now().UTC(),
		Metadata:      metadataBytes,
	})

	if markErr := s.webhookRepo.MarkProcessed(ctx, msg.WebhookEventID); markErr != nil {
		logger.Log.Error().Err(markErr).Int32("webhook_event_id", msg.WebhookEventID).Msg("Failed to mark webhook event as processed")
		return fmt.Errorf("failed to mark webhook event processed: %w", markErr)
	}

	return nil
}
