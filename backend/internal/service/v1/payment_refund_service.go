package v1service

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sony/gobreaker/v2"

	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/payment"
)

type paymentRefundService struct {
	paymentRepo         repository.PaymentRepository
	orderRepo           repository.OrderRepository
	providers           map[string]payment.PaymentProvider
	notificationService NotificationService
	cache               cache.RedisCacheService
}

// NewPaymentRefundService constructs a new instance of PaymentRefundService.
func NewPaymentRefundService(
	paymentRepo repository.PaymentRepository,
	orderRepo repository.OrderRepository,
	providers map[string]payment.PaymentProvider,
	notificationService NotificationService,
	cache cache.RedisCacheService,
) PaymentRefundService {
	return &paymentRefundService{
		paymentRepo:         paymentRepo,
		orderRepo:           orderRepo,
		providers:           providers,
		notificationService: notificationService,
		cache:               cache,
	}
}

func (s *paymentRefundService) RefundPayment(ctx context.Context, idempotencyKey string, input v1dto.RefundPaymentInput) (v1dto.PaymentTransactionDTO, error) {
	startTime := time.Now()

	orderUUID, err := uuid.Parse(input.OrderUUID)
	if err != nil {
		return v1dto.PaymentTransactionDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Invalid order UUID format")
	}

	/* =================== PHASE 1: RESERVE PENDING RECORD =================== */
	txPhase1, err := db.BeginTx(ctx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to begin phase 1 db transaction for refund")
		return v1dto.PaymentTransactionDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to begin db transaction", err)
	}
	defer txPhase1.Rollback(ctx)

	order, err := s.orderRepo.GetByUUIDForUpdate(ctx, txPhase1, orderUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return v1dto.PaymentTransactionDTO{}, utils.NewError(utils.ErrCodeNotFound, "Order not found")
		}
		return v1dto.PaymentTransactionDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to get order", err)
	}

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

	if idempotencyKey == "" {
		raw := fmt.Sprintf("%s:%.2f:%s:%.2f", order.OrderNumber, input.Amount, strings.TrimSpace(input.Reason), currentRefunded)
		hash := sha256.Sum256([]byte(raw))
		idempotencyKey = fmt.Sprintf("refund:%s:%x", order.OrderNumber, hash[:8])
	}

	txns, err := s.paymentRepo.GetByOrderID(ctx, txPhase1, order.OrderID)
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

	refundNumeric := utils.FloatToNumeric(input.Amount)
	pendingRefundTxn, err := s.paymentRepo.CreateRefund(ctx, txPhase1, sqlc.CreateRefundTransactionParams{
		OrderID:        order.OrderID,
		IdempotencyKey: idempotencyKey,
		Provider:       originalTxn.PtProvider,
		ProviderTxnID:  nil,
		Amount:         refundNumeric,
		Currency:       "VND",
		Status:         PaymentStatusPending,
		RefundOf:       &originalTxn.PtID,
		RefundAmount:   refundNumeric,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			_ = txPhase1.Rollback(ctx)

			existingTxn, getErr := s.paymentRepo.GetByIdempotencyKey(ctx, nil, idempotencyKey)
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

	if err := txPhase1.Commit(ctx); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to commit phase 1 refund transaction")
		return v1dto.PaymentTransactionDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to commit phase 1", err)
	}

	/* =================== PHASE 2: CALL EXTERNAL GATEWAY =================== */
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

			refundResp, err := provider.Refund(ctx, payment.RefundRequest{
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
		refundTxnID = fmt.Sprintf("REFUND_COD_%s_%d", order.OrderNumber, time.Now().Unix())
	}

	/* =================== PHASE 3: FINALIZE IN DB =================== */
	txPhase3, err := db.BeginTx(ctx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to begin phase 3 db transaction for refund")
		return v1dto.PaymentTransactionDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to begin phase 3 db transaction", err)
	}
	defer txPhase3.Rollback(ctx)

	if gatewayErr != nil {
		if _, updateErr := s.paymentRepo.UpdateStatusAtomically(ctx, txPhase3, sqlc.UpdatePaymentStatusAtomicallyParams{
			PtUuid:              pendingRefundTxn.PtUuid,
			NewStatus:           PaymentStatusFailed,
			AllowedFromStatuses: []int32{PaymentStatusPending},
		}); updateErr != nil {
			logger.Log.Error().Err(updateErr).Msg("Failed to mark refund transaction as Failed in db")
		}
		if commitErr := txPhase3.Commit(ctx); commitErr != nil {
			logger.Log.Error().Err(commitErr).Msg("Failed to commit failed refund status")
		}
		logger.Log.Error().
			Err(gatewayErr).
			Str("provider", originalTxn.PtProvider).
			Str("order_number", order.OrderNumber).
			Msg("Gateway refund failed, marked transaction as Failed")
		if errors.Is(gatewayErr, gobreaker.ErrOpenState) || errors.Is(gatewayErr, gobreaker.ErrTooManyRequests) {
			return v1dto.PaymentTransactionDTO{}, utils.WrapError(utils.ErrCodeServiceUnavailable, "Payment gateway is temporarily unavailable for refund. Please try again later.", gatewayErr)
		}
		return v1dto.PaymentTransactionDTO{}, utils.WrapError(utils.ErrCodeBadGateway, "Gateway refund failed", gatewayErr)
	}

	finalRefundTxn, err := s.paymentRepo.UpdateStatusAtomically(ctx, txPhase3, sqlc.UpdatePaymentStatusAtomicallyParams{
		PtUuid:              pendingRefundTxn.PtUuid,
		NewStatus:           PaymentStatusPaid,
		ProviderTxnID:       &refundTxnID,
		AllowedFromStatuses: []int32{PaymentStatusPending},
	})
	if err != nil {
		logger.Log.Error().Err(err).Str("order_number", order.OrderNumber).Msg("Failed to confirm refund transaction")
		return v1dto.PaymentTransactionDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to confirm refund transaction", err)
	}

	newTotalRefunded := currentRefunded + input.Amount
	var newOrderStatus int32 = OrderPaymentStatusPartiallyRefunded
	if newTotalRefunded >= totalAmount {
		newOrderStatus = OrderPaymentStatusFullyRefunded
	}

	_, err = s.orderRepo.UpdateOrderPaymentStatus(ctx, txPhase3, sqlc.UpdateOrderPaymentStatusParams{
		OrderUuid:           order.OrderUuid,
		PaymentStatus:       newOrderStatus,
		RefundedAmount:      utils.FloatToNumeric(newTotalRefunded),
		AllowedFromStatuses: []int32{OrderPaymentStatusPaid, OrderPaymentStatusPartiallyRefunded},
	})
	if err != nil {
		logger.Log.Error().Err(err).Str("order_number", order.OrderNumber).Msg("Failed to update order payment status after refund")
		return v1dto.PaymentTransactionDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to update order payment status", err)
	}

	if err := txPhase3.Commit(ctx); err != nil {
		logger.Log.Error().Err(err).Msg("Failed to commit phase 3 database transaction for refund")
		return v1dto.PaymentTransactionDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to commit database transaction", err)
	}

	if s.cache != nil {
		if clearErr := s.cache.Clear("orders:*"); clearErr != nil {
			logger.Log.Warn().Err(clearErr).Msg("Failed to clear orders cache after refund")
		}
	}

	s.dispatchRefundNotification(order, finalRefundTxn, input.Amount)

	logger.Log.Info().
		Str("trace_id", logger.GetTraceId(ctx)).
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

func (s *paymentRefundService) dispatchRefundNotification(order sqlc.Order, finalRefundTxn sqlc.PaymentTransaction, amount float64) {
	if s.notificationService != nil {
		go func() {
			if notifErr := s.notificationService.Send(context.Background(), v1dto.SendNotificationInput{
				UserID: order.OrderUserID,
				Type:   "payment_refunded",
				Title:  "Payment Refunded",
				Body:   fmt.Sprintf("A refund of %.0f VND for order #%s has been successfully processed.", amount, order.OrderNumber),
				Data: map[string]any{
					"order_uuid":      order.OrderUuid.String(),
					"refund_txn_uuid": finalRefundTxn.PtUuid.String(),
				},
			}); notifErr != nil {
				logger.Log.Warn().Err(notifErr).Str("order_number", order.OrderNumber).Msg("Failed to dispatch refund notification")
			}
		}()
	}
}
