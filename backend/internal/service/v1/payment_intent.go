package v1service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/utils"
	"goshop/pkg/logger"
	"goshop/pkg/payment"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sony/gobreaker/v2"
)

// CreatePaymentIntent implements the 5-step payment intent creation lifecycle:
// Step 1: Create payment transaction in DB with Pending status & unique idempotency key.
// Step 2: Ensure only one request has the right to initiate the gateway session for this intent.
// Step 3: Call external gateway with stable, deterministic merchant reference (txnRef).
// Step 4: Persist gateway URL and provider transaction info into DB & Redis cache.
// Step 5: On gateway timeout/uncertainty, preserve Pending status for background reconciliation.
func (s *paymentService) CreatePaymentIntent(ctx context.Context, userUUID uuid.UUID, clientIP, idempotencyKey string, input v1dto.CreatePaymentIntentInput) (v1dto.PaymentIntentDTO, error) {
	startTime := time.Now()

	user, err := s.getUser(ctx, userUUID)
	if err != nil {
		return v1dto.PaymentIntentDTO{}, err
	}

	order, err := s.orderReader.GetByUUID(ctx, input.OrderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return v1dto.PaymentIntentDTO{}, utils.NewError(utils.ErrCodeNotFound, "Order not found")
		}
		return v1dto.PaymentIntentDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to get order", err)
	}

	if err := s.validateOrderForPayment(order, user); err != nil {
		return v1dto.PaymentIntentDTO{}, err
	}

	orderAmount := order.TotalAmount

	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return v1dto.PaymentIntentDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Missing X-Idempotency-Key header")
	}

	redisKey := fmt.Sprintf("idem:payment:%s", idempotencyKey)

	cached, err := s.checkIdempotency(ctx, idempotencyKey, order)
	if err != nil {
		return v1dto.PaymentIntentDTO{}, err
	}
	if cached != nil {
		return *cached, nil
	}

	providerName := strings.ToLower(strings.TrimSpace(input.Provider))
	provider, exists := s.providers[providerName]
	if !exists {
		return v1dto.PaymentIntentDTO{}, utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("Unsupported payment provider: %s", providerName))
	}

	// Step 1: Create payment record in DB in Pending status with deterministic merchant reference.
	txnRef := fmt.Sprintf("%s_%s", order.OrderNumber, idempotencyKey)
	ptParams := sqlc.CreatePaymentTransactionParams{
		PtOrderID:        order.OrderID,
		PtIdempotencyKey: idempotencyKey,
		PtProvider:       providerName,
		PtTxnRef:         &txnRef,
		PtProviderTxnID:  nil,
		PtProviderUrl:    nil, // Will be set in Step 4 after gateway succeeds
		PtAmount:         utils.FloatToNumeric(order.TotalAmount),
		PtCurrency:       "VND",
		PtStatus:         PaymentStatusPending,
	}

	paymentTxn, err := s.paymentRepo.Create(ctx, nil, ptParams)
	if err != nil {
		var pgErr *pgconn.PgError
		// Step 2: Handle duplicate key (23505) - only one request has the right to initiate gateway
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			logger.Log.Warn().Str("key", idempotencyKey).Msg("Concurrent or duplicate payment intent request detected, checking existing transaction")
			existingTxn, getErr := s.paymentRepo.GetByIdempotencyKey(ctx, nil, idempotencyKey)
			if getErr != nil {
				return v1dto.PaymentIntentDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to get existing payment transaction", getErr)
			}
			if existingTxn.PtOrderID != order.OrderID {
				return v1dto.PaymentIntentDTO{}, utils.NewError(utils.ErrCodeConflict, "Idempotency key already used for a different order")
			}
			if existingTxn.PtStatus == PaymentStatusPaid {
				return v1dto.PaymentIntentDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Order has already been paid")
			}
			if existingTxn.PtProviderUrl != nil && *existingTxn.PtProviderUrl != "" {
				dto := v1dto.MapToPaymentIntentDTO(existingTxn, order.OrderUUID)
				if s.cache != nil {
					if setErr := s.cache.Set(redisKey, dto, 24*time.Hour); setErr != nil {
						logger.Log.Warn().Err(setErr).Str("key", redisKey).Msg("Failed to cache payment intent in Redis")
					}
				}
				return dto, nil
			}

			// If URL is not populated yet, another request is currently calling the gateway.
			select {
			case <-ctx.Done():
				return v1dto.PaymentIntentDTO{}, ctx.Err()
			case <-time.After(200 * time.Millisecond):
			}

			existingTxn, getErr = s.paymentRepo.GetByIdempotencyKey(ctx, nil, idempotencyKey)
			if getErr == nil && existingTxn.PtProviderUrl != nil && *existingTxn.PtProviderUrl != "" {
				dto := v1dto.MapToPaymentIntentDTO(existingTxn, order.OrderUUID)
				if s.cache != nil {
					if setErr := s.cache.Set(redisKey, dto, 24*time.Hour); setErr != nil {
						logger.Log.Warn().Err(setErr).Str("key", redisKey).Msg("Failed to cache payment intent in Redis")
					}
				}
				return dto, nil
			}
			return v1dto.PaymentIntentDTO{}, utils.NewError(utils.ErrCodeConflict, "Payment intent initialization is currently in progress. Please retry or check order status.")
		}
		return v1dto.PaymentIntentDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to create payment transaction", err)
	}

	// Step 3: Call gateway with stable, idempotent merchant reference (txnRef)
	webhookURL := s.buildWebhookURL(providerName)
	paymentReq := payment.CreatePaymentRequest{
		OrderNumber: txnRef,
		Amount:      orderAmount,
		Currency:    "VND",
		Description: fmt.Sprintf("Payment for order %s", order.OrderNumber),
		ReturnURL:   s.returnURL,
		WebhookURL:  webhookURL,
		ClientIP:    clientIP,
	}

	providerResp, err := provider.CreatePaymentURL(ctx, paymentReq)
	if err != nil {
		logger.Log.Error().
			Err(err).
			Str("trace_id", logger.GetTraceId(ctx)).
			Str("module", "payment").
			Str("action", "create_intent").
			Str("provider", providerName).
			Str("payment_uuid", paymentTxn.PtUuid.String()).
			Str("order_uuid", order.OrderUUID.String()).
			Str("txn_ref", txnRef).
			Msg("Gateway invocation failed or timed out; transaction retained in Pending for reconciliation")

		isTimeoutOrTransient := errors.Is(err, context.DeadlineExceeded) ||
			errors.Is(err, gobreaker.ErrOpenState) ||
			errors.Is(err, gobreaker.ErrTooManyRequests) ||
			strings.Contains(strings.ToLower(err.Error()), "timeout") ||
			strings.Contains(strings.ToLower(err.Error()), "deadline") ||
			strings.Contains(strings.ToLower(err.Error()), "connection refused") ||
			strings.Contains(strings.ToLower(err.Error()), "connection reset")

		// Step 5: On timeout or transient gateway error, keep Pending for reconciliation
		if isTimeoutOrTransient {
			return v1dto.PaymentIntentDTO{}, utils.WrapError(
				utils.ErrCodeServiceUnavailable,
				"Payment gateway is temporarily unavailable or timed out. Transaction is pending reconciliation. Please check payment status later.",
				err,
			)
		}

		// Permanent failure: mark status failed
		if paymentTxn.PtUuid != uuid.Nil {
			if _, updateErr := s.paymentRepo.UpdateStatusAtomically(ctx, nil, sqlc.UpdatePaymentStatusAtomicallyParams{
				PtUuid:              paymentTxn.PtUuid,
				NewStatus:           PaymentStatusFailed,
				AllowedFromStatuses: []int32{PaymentStatusPending},
			}); updateErr != nil {
				logger.Log.Error().
					Err(updateErr).
					Str("payment_uuid", paymentTxn.PtUuid.String()).
					Msg("Failed to mark payment transaction as failed after gateway rejection")
			}
		}
		return v1dto.PaymentIntentDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to connect to payment gateway. Please try again later.", err)
	}

	// Step 4: Persist payment URL and provider transaction info into DB & Redis cache
	var providerURL *string
	if providerResp.PaymentURL != "" {
		providerURL = &providerResp.PaymentURL
	}
	var providerTxnID *string
	if providerResp.ProviderTxnID != "" {
		providerTxnID = &providerResp.ProviderTxnID
	}

	paymentTxn.PtProviderUrl = providerURL
	paymentTxn.PtProviderTxnID = providerTxnID

	if paymentTxn.PtUuid != uuid.Nil {
		updatedTxn, updateErr := s.paymentRepo.UpdateProviderInfo(ctx, nil, sqlc.UpdatePaymentProviderInfoParams{
			PtUuid:        paymentTxn.PtUuid,
			ProviderUrl:   providerURL,
			ProviderTxnID: providerTxnID,
		})
		if updateErr == nil {
			if updatedTxn.PtProvider != "" {
				paymentTxn = updatedTxn
			} else {
				paymentTxn.PtProviderUrl = updatedTxn.PtProviderUrl
				paymentTxn.PtProviderTxnID = updatedTxn.PtProviderTxnID
			}
		} else {
			logger.Log.Warn().
				Err(updateErr).
				Str("payment_uuid", paymentTxn.PtUuid.String()).
				Msg("Failed to persist provider URL to database, continuing with in-memory transaction")
		}
	}

	responseDTO := v1dto.MapToPaymentIntentDTO(paymentTxn, order.OrderUUID)
	if s.cache != nil {
		if setErr := s.cache.Set(redisKey, responseDTO, 24*time.Hour); setErr != nil {
			logger.Log.Warn().Err(setErr).Str("key", redisKey).Msg("Failed to cache payment intent in Redis")
		}
	}

	logger.Log.Info().
		Str("trace_id", logger.GetTraceId(ctx)).
		Str("module", "payment").
		Str("action", "create_intent").
		Str("order_number", order.OrderNumber).
		Str("order_uuid", order.OrderUUID.String()).
		Str("payment_uuid", paymentTxn.PtUuid.String()).
		Str("txn_ref", txnRef).
		Str("provider", providerName).
		Str("status", "Pending").
		Float64("amount", orderAmount).
		Int64("duration_ms", time.Since(startTime).Milliseconds()).
		Msg("Payment intent created successfully")

	return responseDTO, nil
}

func (s *paymentService) validateOrderForPayment(order *PaymentOrderSummary, user sqlc.User) error {
	if order.UserID != user.UserID {
		return utils.NewError(utils.ErrCodeForbidden, "You are not authorized to pay for this order")
	}

	if order.PaymentStatus == OrderPaymentStatusPaid {
		return utils.NewError(utils.ErrCodeBadRequest, "Order has already been paid")
	}

	if order.OrderStatus != OrderStatusPending && order.OrderStatus != OrderStatusConfirmed {
		return utils.NewError(utils.ErrCodeBadRequest, "Order status does not allow payment")
	}

	if order.PaymentMethod == OrderPaymentMethodCOD {
		return utils.NewError(utils.ErrCodeBadRequest, "COD orders cannot create online payment intent")
	}

	if order.TotalAmount <= 0 {
		return utils.NewError(utils.ErrCodeBadRequest, "Order amount is 0, online payment is not required")
	}

	if !order.CreatedAt.IsZero() && time.Since(order.CreatedAt) > 15*time.Minute {
		return utils.NewError(utils.ErrCodeBadRequest, "Order payment window (15 minutes) has expired. Please place a new order.")
	}

	return nil
}

func (s *paymentService) checkIdempotency(ctx context.Context, idempotencyKey string, order *PaymentOrderSummary) (*v1dto.PaymentIntentDTO, error) {
	redisKey := fmt.Sprintf("idem:payment:%s", idempotencyKey)
	var cachedDTO v1dto.PaymentIntentDTO
	if s.cache != nil {
		if err := s.cache.Get(redisKey, &cachedDTO); err == nil {
			if cachedDTO.OrderUUID != order.OrderUUID.String() {
				return nil, utils.NewError(utils.ErrCodeConflict, "Idempotency key already used for a different order")
			}
			return &cachedDTO, nil
		}
	}

	existingTxn, err := s.paymentRepo.GetByIdempotencyKey(ctx, nil, idempotencyKey)
	if err == nil {
		if existingTxn.PtOrderID != order.OrderID {
			return nil, utils.NewError(utils.ErrCodeConflict, "Idempotency key already used for a different order")
		}
		if existingTxn.PtStatus == PaymentStatusPaid {
			return nil, utils.NewError(utils.ErrCodeBadRequest, "Order has already been paid")
		}
		if existingTxn.PtProviderUrl != nil && *existingTxn.PtProviderUrl != "" {
			dto := v1dto.MapToPaymentIntentDTO(existingTxn, order.OrderUUID)
			if s.cache != nil {
				if setErr := s.cache.Set(redisKey, dto, 24*time.Hour); setErr != nil {
					logger.Log.Warn().Err(setErr).Str("key", redisKey).Msg("Failed to cache payment intent in Redis")
				}
			}
			return &dto, nil
		}
		return nil, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to check idempotency key", err)
	}

	return nil, nil
}

func (s *paymentService) buildWebhookURL(providerName string) string {
	webhookBase := strings.TrimRight(s.webhookBaseURL, "/")
	if !strings.Contains(webhookBase, "/api/v1/webhooks/payment") {
		webhookBase = fmt.Sprintf("%s/api/v1/webhooks/payment", webhookBase)
	}
	return fmt.Sprintf("%s/%s", webhookBase, providerName)
}
