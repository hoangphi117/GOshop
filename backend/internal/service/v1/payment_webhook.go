package v1service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/utils"
	"goshop/pkg/logger"
	"goshop/pkg/rabbitmq"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	amqp "github.com/rabbitmq/amqp091-go"
)

// HandleWebhook verifies gateway webhook signatures, logs the event idempotently, and enqueues to RabbitMQ.
func (s *paymentService) HandleWebhook(ctx context.Context, providerName string, rawPayload []byte, headers map[string]string) error {
	startTime := time.Now()

	providerName = strings.ToLower(strings.TrimSpace(providerName))
	provider, exists := s.providers[providerName]
	if !exists {
		return utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("Unsupported payment provider: %s", providerName))
	}

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

	eventParams := sqlc.InsertWebhookEventParams{
		WeProvider:        providerName,
		WeExternalEventID: result.EventID,
		WeEventType:       result.EventType,
		WeRawBody:         string(rawPayload),
	}

	event, err := s.webhookRepo.InsertEvent(ctx, nil, eventParams)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Log.Info().
				Str("provider", providerName).
				Str("event_id", result.EventID).
				Msg("Duplicate webhook event received (ON CONFLICT), acknowledging gateway")
			return nil
		}
		logger.Log.Error().
			Err(err).
			Str("provider", providerName).
			Str("event_id", result.EventID).
			Msg("Database failure while inserting webhook event")
		return utils.WrapError(utils.ErrCodeInternal, "failed to persist webhook event", err)
	}

	if event.WeID == 0 {
		logger.Log.Info().
			Str("provider", providerName).
			Str("event_id", result.EventID).
			Msg("Duplicate webhook event received (WeID=0), acknowledging gateway")
		return nil
	}

	realOrderNumber, _, _ := strings.Cut(result.OrderNumber, "_")

	var orderUUID string
	order, err := s.orderReader.GetByOrderNumber(ctx, realOrderNumber)
	if err == nil {
		orderUUID = order.OrderUUID.String()
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

	if err := s.rabbitmq.Publish(ctx, "payment.webhook", msgPayload); err != nil {
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

// ProcessWebhookMessage consumes incoming webhook messages from the payment.webhook RabbitMQ queue.
func (s *paymentService) ProcessWebhookMessage(headers amqp.Table, body []byte) error {
	var msg v1dto.PaymentWebhookMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to unmarshal payment webhook message")
		return err
	}
	return s.ProcessWebhook(context.Background(), msg)
}

// ProcessWebhook applies the transaction status transition and enqueues transactional outbox events atomically.
func (s *paymentService) ProcessWebhook(ctx context.Context, msg v1dto.PaymentWebhookMessage) error {
	if ctx == nil {
		ctx = context.Background()
	}
	startTime := time.Now()

	var newPaymentStatus int32 = PaymentStatusFailed
	if msg.Status == "success" {
		newPaymentStatus = PaymentStatusPaid
	}

	tx, err := db.BeginTx(ctx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to begin database transaction")
		return err
	}
	defer tx.Rollback(ctx)

	pt, err := s.paymentRepo.GetByTxnRef(ctx, tx, msg.Provider, msg.OrderNumber)
	if err != nil {
		pt, err = s.paymentRepo.GetByProviderTxnID(ctx, tx, msg.Provider, msg.OrderNumber)
	}
	if err != nil {
		logger.Log.Error().
			Err(err).
			Str("provider", msg.Provider).
			Str("txn_ref", msg.OrderNumber).
			Msg("Payment transaction not found by txn_ref")
		return err
	}

	if pt.PtStatus == PaymentStatusPaid {
		logger.Log.Info().
			Str("module", "payment").
			Str("action", "webhook_idempotent_skip").
			Str("pt_uuid", pt.PtUuid.String()).
			Str("order_number", msg.OrderNumber).
			Msg("Payment transaction is already Paid, acknowledging redelivered message as idempotent success")

		if markErr := s.webhookRepo.MarkProcessed(ctx, tx, msg.WebhookEventID); markErr != nil {
			logger.Log.Warn().Err(markErr).Int32("event_id", msg.WebhookEventID).Msg("Failed to mark duplicate webhook event as processed")
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			logger.Log.Error().Err(commitErr).Msg("Failed to commit transaction on idempotent webhook skip")
			return commitErr
		}
		return nil
	}

	metaBytes, err := json.Marshal(map[string]interface{}{
		"txn_ref":  msg.OrderNumber,
		"trans_id": msg.ProviderTxnID,
	})
	if err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to marshal payment metadata, using empty json object")
		metaBytes = []byte("{}")
	}
	_, err = s.paymentRepo.UpdateStatusAtomically(ctx, tx, sqlc.UpdatePaymentStatusAtomicallyParams{
		PtUuid:              pt.PtUuid,
		NewStatus:           newPaymentStatus,
		ProviderTxnID:       &msg.ProviderTxnID,
		Metadata:            metaBytes,
		AllowedFromStatuses: []int32{PaymentStatusPending},
	})
	if err != nil {
		logger.Log.Error().Err(err).Str("pt_uuid", pt.PtUuid.String()).Msg("Failed to update payment transaction")
		return err
	}

	var paidOrder *PaymentOrderSummary
	if msg.Status == "success" {
		realOrderNumber, _, _ := strings.Cut(msg.OrderNumber, "_")

		order, err := s.orderReader.GetByOrderNumber(ctx, realOrderNumber)
		if err == nil {
			paidOrder = order
		}
	}

	if err := s.webhookRepo.MarkProcessed(ctx, tx, msg.WebhookEventID); err != nil {
		logger.Log.Error().
			Err(err).
			Int32("webhook_event_id", msg.WebhookEventID).
			Msg("Failed to mark webhook event as processed")
		return utils.WrapError(utils.ErrCodeInternal, "failed to mark webhook event as processed", err)
	}

	// Enqueue payment event to transactional outbox atomically
	if s.outboxRepo != nil {
		if msg.Status == "success" && paidOrder != nil {
			event := rabbitmq.NewPaymentSucceededEvent(
				uuid.New().String(),
				pt.PtUuid.String(),
				paidOrder.OrderUUID.String(),
				paidOrder.OrderNumber,
				paidOrder.UserUUID.String(),
				msg.Provider,
				msg.ProviderTxnID,
				msg.Amount,
			)
			if _, err := s.outboxRepo.Insert(ctx, tx, "payment", pt.PtUuid.String(), rabbitmq.QueuePaymentSucceeded, event); err != nil {
				logger.Log.Error().Err(err).Msg("Failed to enqueue payment.succeeded outbox event")
				return err
			}
		} else if msg.Status != "success" {
			event := rabbitmq.NewPaymentFailedEvent(
				uuid.New().String(),
				pt.PtUuid.String(),
				msg.OrderUUID,
				msg.OrderNumber,
				"",
				msg.Provider,
				msg.Status,
			)
			if _, err := s.outboxRepo.Insert(ctx, tx, "payment", pt.PtUuid.String(), rabbitmq.QueuePaymentFailed, event); err != nil {
				logger.Log.Error().Err(err).Msg("Failed to enqueue payment.failed outbox event")
				return err
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to commit database transaction")
		return err
	}

	if s.cache != nil {
		if clearErr := s.cache.Clear("orders:*"); clearErr != nil {
			logger.Log.Warn().Err(clearErr).Msg("Failed to clear orders cache")
		}
	}

	if msg.Status == "success" {
		if paidOrder != nil {
			s.dispatchSuccessNotification(*paidOrder, msg.ProviderTxnID)
			if s.outboxRepo == nil {
				s.publishPaymentSucceeded(ctx, pt.PtUuid.String(), paidOrder.OrderUUID.String(), paidOrder.OrderNumber, paidOrder.UserUUID.String(), msg.Provider, msg.ProviderTxnID, msg.Amount)
			}
		}
	} else {
		if s.outboxRepo == nil {
			s.publishPaymentFailed(ctx, pt.PtUuid.String(), msg.OrderUUID, msg.OrderNumber, "", msg.Provider, msg.Status)
		}
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

func (s *paymentService) dispatchSuccessNotification(paidOrder PaymentOrderSummary, providerTxnID string) {
	if s.notificationService != nil {
		go func() {
			if notifErr := s.notificationService.Send(context.Background(), v1dto.SendNotificationInput{
				UserID: paidOrder.UserID,
				Type:   "payment_success",
				Title:  "Payment Received",
				Body:   fmt.Sprintf("Payment for order #%s was successfully received.", paidOrder.OrderNumber),
				Data: map[string]any{
					"order_uuid":      paidOrder.OrderUUID.String(),
					"provider_txn_id": providerTxnID,
				},
			}); notifErr != nil {
				logger.Log.Warn().Err(notifErr).Str("order_number", paidOrder.OrderNumber).Msg("Failed to dispatch payment success notification")
			}
		}()
	}
}
