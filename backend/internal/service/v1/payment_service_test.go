package v1service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	v1service "goshop/internal/service/v1"
	"goshop/internal/testutils"
	"goshop/internal/utils"
	"goshop/pkg/payment"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func toOrderReader(repo repository.OrderRepository) v1service.PaymentOrderReaderPort {
	if repo == nil {
		return nil
	}
	return v1service.NewOrderReaderAdapter(repo)
}

func TestPaymentService_CreatePaymentIntent(t *testing.T) {
	ctx := context.Background()
	orderUUID := uuid.New()

	t.Run("Case 1: Order Not Found -> Returns 404", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewPaymentService(nil, toOrderReader(mockOrderRepo), nil, nil, nil, nil, "", "")
		_, err := svc.CreatePaymentIntent(ctx, int32(1), "127.0.0.1", "key123", v1dto.CreatePaymentIntentInput{
			OrderUUID: orderUUID,
			Provider:  "momo",
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Forbidden When User Does Not Own Order -> Returns 403", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:     10,
					OrderUuid:   orderUUID,
					OrderUserID: 999, // Different user
				}, nil
			},
		}

		svc := v1service.NewPaymentService(nil, toOrderReader(mockOrderRepo), nil, nil, nil, nil, "", "")
		_, err := svc.CreatePaymentIntent(ctx, int32(1), "127.0.0.1", "key123", v1dto.CreatePaymentIntentInput{
			OrderUUID: orderUUID,
			Provider:  "momo",
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeForbidden)
	})

	t.Run("Case 3: Order Already Paid -> Returns 400", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:            10,
					OrderUuid:          orderUUID,
					OrderUserID:        1,
					OrderPaymentStatus: v1service.OrderPaymentStatusPaid,
				}, nil
			},
		}

		svc := v1service.NewPaymentService(nil, toOrderReader(mockOrderRepo), nil, nil, nil, nil, "", "")
		_, err := svc.CreatePaymentIntent(ctx, int32(1), "127.0.0.1", "key123", v1dto.CreatePaymentIntentInput{
			OrderUUID: orderUUID,
			Provider:  "momo",
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "already been paid")
	})

	t.Run("Case 4: Order Status Disallowed -> Returns 400", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:            10,
					OrderUuid:          orderUUID,
					OrderUserID:        1,
					OrderPaymentStatus: v1service.OrderPaymentStatusUnpaid,
					OrderStatus:        v1service.OrderStatusCancelled,
				}, nil
			},
		}

		svc := v1service.NewPaymentService(nil, toOrderReader(mockOrderRepo), nil, nil, nil, nil, "", "")
		_, err := svc.CreatePaymentIntent(ctx, int32(1), "127.0.0.1", "key123", v1dto.CreatePaymentIntentInput{
			OrderUUID: orderUUID,
			Provider:  "momo",
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "does not allow payment")
	})

	t.Run("Case 5: COD Order -> Returns 400", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:            10,
					OrderUuid:          orderUUID,
					OrderUserID:        1,
					OrderPaymentStatus: v1service.OrderPaymentStatusUnpaid,
					OrderStatus:        v1service.OrderStatusPending,
					OrderPaymentMethod: v1service.OrderPaymentMethodCOD,
				}, nil
			},
		}

		svc := v1service.NewPaymentService(nil, toOrderReader(mockOrderRepo), nil, nil, nil, nil, "", "")
		_, err := svc.CreatePaymentIntent(ctx, int32(1), "127.0.0.1", "key123", v1dto.CreatePaymentIntentInput{
			OrderUUID: orderUUID,
			Provider:  "momo",
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "COD orders")
	})

	t.Run("Case 6: Zero Amount -> Returns 400", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:            10,
					OrderUuid:          orderUUID,
					OrderUserID:        1,
					OrderPaymentStatus: v1service.OrderPaymentStatusUnpaid,
					OrderStatus:        v1service.OrderStatusPending,
					OrderPaymentMethod: v1service.OrderPaymentMethodOnline,
					OrderTotalAmount:   utils.FloatToNumeric(0),
				}, nil
			},
		}

		svc := v1service.NewPaymentService(nil, toOrderReader(mockOrderRepo), nil, nil, nil, nil, "", "")
		_, err := svc.CreatePaymentIntent(ctx, int32(1), "127.0.0.1", "key123", v1dto.CreatePaymentIntentInput{
			OrderUUID: orderUUID,
			Provider:  "momo",
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "amount is 0")
	})

	t.Run("Case 7: Missing Idempotency Key -> Returns 400", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:            10,
					OrderUuid:          orderUUID,
					OrderUserID:        1,
					OrderPaymentStatus: v1service.OrderPaymentStatusUnpaid,
					OrderStatus:        v1service.OrderStatusPending,
					OrderPaymentMethod: v1service.OrderPaymentMethodOnline,
					OrderTotalAmount:   utils.FloatToNumeric(50000),
				}, nil
			},
		}

		svc := v1service.NewPaymentService(nil, toOrderReader(mockOrderRepo), nil, nil, nil, nil, "", "")
		_, err := svc.CreatePaymentIntent(ctx, int32(1), "127.0.0.1", "   ", v1dto.CreatePaymentIntentInput{
			OrderUUID: orderUUID,
			Provider:  "momo",
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "Missing X-Idempotency-Key")
	})

	t.Run("Case 8: Unsupported Provider -> Returns 400", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:            10,
					OrderUuid:          orderUUID,
					OrderUserID:        1,
					OrderPaymentStatus: v1service.OrderPaymentStatusUnpaid,
					OrderStatus:        v1service.OrderStatusPending,
					OrderPaymentMethod: v1service.OrderPaymentMethodOnline,
					OrderTotalAmount:   utils.FloatToNumeric(50000),
				}, nil
			},
		}
		mockPaymentRepo := &testutils.MockPaymentRepository{
			GetByIdempotencyKeyFunc: func(ctx context.Context, tx pgx.Tx, idempotencyKey string) (sqlc.PaymentTransaction, error) {
				return sqlc.PaymentTransaction{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewPaymentService(mockPaymentRepo, toOrderReader(mockOrderRepo), nil, map[string]payment.PaymentProvider{}, nil, nil, "", "")
		_, err := svc.CreatePaymentIntent(ctx, int32(1), "127.0.0.1", "key123", v1dto.CreatePaymentIntentInput{
			OrderUUID: orderUUID,
			Provider:  "unknown_gateway",
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "Unsupported payment provider")
	})

	t.Run("Case 9: Happy Path -> Generates Payment URL & Returns DTO", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:            10,
					OrderNumber:        "ORD123456",
					OrderUuid:          orderUUID,
					OrderUserID:        1,
					OrderPaymentStatus: v1service.OrderPaymentStatusUnpaid,
					OrderStatus:        v1service.OrderStatusPending,
					OrderPaymentMethod: v1service.OrderPaymentMethodOnline,
					OrderTotalAmount:   utils.FloatToNumeric(50000),
				}, nil
			},
		}
		mockPaymentRepo := &testutils.MockPaymentRepository{
			GetByIdempotencyKeyFunc: func(ctx context.Context, tx pgx.Tx, idempotencyKey string) (sqlc.PaymentTransaction, error) {
				return sqlc.PaymentTransaction{}, pgx.ErrNoRows
			},
			CreateFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.CreatePaymentTransactionParams) (sqlc.PaymentTransaction, error) {
				return sqlc.PaymentTransaction{
					PtID:             1,
					PtUuid:           uuid.New(),
					PtOrderID:        params.PtOrderID,
					PtIdempotencyKey: params.PtIdempotencyKey,
					PtProvider:       params.PtProvider,
					PtTxnRef:         params.PtTxnRef,
					PtProviderUrl:    params.PtProviderUrl,
					PtAmount:         params.PtAmount,
					PtCurrency:       params.PtCurrency,
					PtStatus:         params.PtStatus,
					PtCreatedAt:      time.Now(),
				}, nil
			},
		}
		mockProvider := &testutils.MockPaymentProvider{
			CreatePaymentURLFunc: func(ctx context.Context, req payment.CreatePaymentRequest) (payment.CreatePaymentResponse, error) {
				return payment.CreatePaymentResponse{
					PaymentURL: "https://gateway.com/pay/123",
				}, nil
			},
		}

		providers := map[string]payment.PaymentProvider{
			"momo": mockProvider,
		}

		svc := v1service.NewPaymentService(mockPaymentRepo, toOrderReader(mockOrderRepo), nil, providers, nil, nil, "https://store.com/return", "https://api.com/webhooks")
		dto, err := svc.CreatePaymentIntent(ctx, int32(1), "127.0.0.1", "key123", v1dto.CreatePaymentIntentInput{
			OrderUUID: orderUUID,
			Provider:  "momo",
		})

		require.NoError(t, err)
		assert.Equal(t, orderUUID.String(), dto.OrderUUID)
		assert.Equal(t, "momo", dto.Provider)
		require.NotEmpty(t, dto.PaymentURL)
		assert.Equal(t, "https://gateway.com/pay/123", dto.PaymentURL)
	})

	t.Run("Case 10: Payment Gateway Circuit Breaker Open -> Returns 503 Service Unavailable", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:            10,
					OrderNumber:        "ORD123456",
					OrderUuid:          orderUUID,
					OrderUserID:        1,
					OrderPaymentStatus: v1service.OrderPaymentStatusUnpaid,
					OrderStatus:        v1service.OrderStatusPending,
					OrderPaymentMethod: v1service.OrderPaymentMethodOnline,
					OrderTotalAmount:   utils.FloatToNumeric(50000),
				}, nil
			},
		}
		mockPaymentRepo := &testutils.MockPaymentRepository{
			GetByIdempotencyKeyFunc: func(ctx context.Context, tx pgx.Tx, idempotencyKey string) (sqlc.PaymentTransaction, error) {
				return sqlc.PaymentTransaction{}, pgx.ErrNoRows
			},
		}

		// Raw provider that repeatedly fails
		rawProvider := &testutils.MockPaymentProvider{
			CreatePaymentURLFunc: func(ctx context.Context, req payment.CreatePaymentRequest) (payment.CreatePaymentResponse, error) {
				return payment.CreatePaymentResponse{}, errors.New("gateway connection timeout")
			},
		}

		// Wrap with Circuit Breaker configured with threshold = 2
		cbProvider := payment.NewCircuitBreakerPaymentProvider(rawProvider, "momo", payment.CBConfig{
			MaxRequests: 1,
			Interval:    0,
			Timeout:     10 * time.Second,
			MaxFailures: 2,
		})

		providers := map[string]payment.PaymentProvider{
			"momo": cbProvider,
		}

		svc := v1service.NewPaymentService(mockPaymentRepo, toOrderReader(mockOrderRepo), nil, providers, nil, nil, "https://store.com/return", "https://api.com/webhooks")

		// First 2 failures trip the breaker
		_, _ = svc.CreatePaymentIntent(ctx, int32(1), "127.0.0.1", "key1", v1dto.CreatePaymentIntentInput{OrderUUID: orderUUID, Provider: "momo"})
		_, _ = svc.CreatePaymentIntent(ctx, int32(1), "127.0.0.1", "key2", v1dto.CreatePaymentIntentInput{OrderUUID: orderUUID, Provider: "momo"})

		// 3rd call must immediately fail with ErrCodeServiceUnavailable (503)
		_, err := svc.CreatePaymentIntent(ctx, int32(1), "127.0.0.1", "key3", v1dto.CreatePaymentIntentInput{
			OrderUUID: orderUUID,
			Provider:  "momo",
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeServiceUnavailable)
		assert.Contains(t, err.Error(), "temporarily unavailable")
	})

	t.Run("Case 11: Step 1, 3, 4 Verification -> DB Record Created First, Stable TxnRef Sent To Gateway, URL Saved", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:            10,
					OrderNumber:        "ORD_STEP123",
					OrderUuid:          orderUUID,
					OrderUserID:        1,
					OrderPaymentStatus: v1service.OrderPaymentStatusUnpaid,
					OrderStatus:        v1service.OrderStatusPending,
					OrderPaymentMethod: v1service.OrderPaymentMethodOnline,
					OrderTotalAmount:   utils.FloatToNumeric(75000),
				}, nil
			},
		}

		var executionOrder []string
		ptUUID := uuid.New()

		mockPaymentRepo := &testutils.MockPaymentRepository{
			GetByIdempotencyKeyFunc: func(ctx context.Context, tx pgx.Tx, idempotencyKey string) (sqlc.PaymentTransaction, error) {
				return sqlc.PaymentTransaction{}, pgx.ErrNoRows
			},
			CreateFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.CreatePaymentTransactionParams) (sqlc.PaymentTransaction, error) {
				executionOrder = append(executionOrder, "db_create")
				assert.Equal(t, int32(v1service.PaymentStatusPending), params.PtStatus)
				assert.Nil(t, params.PtProviderUrl)
				require.NotNil(t, params.PtTxnRef)
				assert.Equal(t, "ORD_STEP123_idem-step-test", *params.PtTxnRef)

				return sqlc.PaymentTransaction{
					PtID:             99,
					PtUuid:           ptUUID,
					PtOrderID:        params.PtOrderID,
					PtIdempotencyKey: params.PtIdempotencyKey,
					PtProvider:       params.PtProvider,
					PtTxnRef:         params.PtTxnRef,
					PtAmount:         params.PtAmount,
					PtCurrency:       params.PtCurrency,
					PtStatus:         params.PtStatus,
					PtCreatedAt:      time.Now(),
				}, nil
			},
			UpdateProviderInfoFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.UpdatePaymentProviderInfoParams) (sqlc.PaymentTransaction, error) {
				executionOrder = append(executionOrder, "db_update_url")
				assert.Equal(t, ptUUID, params.PtUuid)
				require.NotNil(t, params.ProviderUrl)
				assert.Equal(t, "https://momo.vn/pay/session_step4", *params.ProviderUrl)

				return sqlc.PaymentTransaction{
					PtID:          99,
					PtUuid:        ptUUID,
					PtProvider:    "momo",
					PtProviderUrl: params.ProviderUrl,
					PtStatus:      v1service.PaymentStatusPending,
				}, nil
			},
		}

		mockProvider := &testutils.MockPaymentProvider{
			CreatePaymentURLFunc: func(ctx context.Context, req payment.CreatePaymentRequest) (payment.CreatePaymentResponse, error) {
				executionOrder = append(executionOrder, "gateway_call")
				assert.Equal(t, "ORD_STEP123_idem-step-test", req.OrderNumber)
				return payment.CreatePaymentResponse{
					PaymentURL: "https://momo.vn/pay/session_step4",
				}, nil
			},
		}

		svc := v1service.NewPaymentService(mockPaymentRepo, toOrderReader(mockOrderRepo), nil, map[string]payment.PaymentProvider{"momo": mockProvider}, nil, nil, "https://store.com/return", "https://api.com/webhooks")
		dto, err := svc.CreatePaymentIntent(ctx, int32(1), "127.0.0.1", "idem-step-test", v1dto.CreatePaymentIntentInput{
			OrderUUID: orderUUID,
			Provider:  "momo",
		})

		require.NoError(t, err)
		assert.Equal(t, "https://momo.vn/pay/session_step4", dto.PaymentURL)
		assert.Equal(t, []string{"db_create", "gateway_call", "db_update_url"}, executionOrder)
	})

	t.Run("Case 12: Step 2 Duplicate Key (23505) -> Returns Existing DTO Without Re-invoking Gateway", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:            10,
					OrderNumber:        "ORD_STEP123",
					OrderUuid:          orderUUID,
					OrderUserID:        1,
					OrderPaymentStatus: v1service.OrderPaymentStatusUnpaid,
					OrderStatus:        v1service.OrderStatusPending,
					OrderPaymentMethod: v1service.OrderPaymentMethodOnline,
					OrderTotalAmount:   utils.FloatToNumeric(75000),
				}, nil
			},
		}

		existingURL := "https://momo.vn/pay/already_created"
		gatewayCalled := false

		mockPaymentRepo := &testutils.MockPaymentRepository{
			GetByIdempotencyKeyFunc: func(ctx context.Context, tx pgx.Tx, idempotencyKey string) (sqlc.PaymentTransaction, error) {
				return sqlc.PaymentTransaction{
					PtID:             88,
					PtUuid:           uuid.New(),
					PtOrderID:        10,
					PtIdempotencyKey: idempotencyKey,
					PtProvider:       "momo",
					PtProviderUrl:    &existingURL,
					PtStatus:         v1service.PaymentStatusPending,
				}, nil
			},
			CreateFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.CreatePaymentTransactionParams) (sqlc.PaymentTransaction, error) {
				return sqlc.PaymentTransaction{}, &pgconn.PgError{Code: "23505"}
			},
		}

		mockProvider := &testutils.MockPaymentProvider{
			CreatePaymentURLFunc: func(ctx context.Context, req payment.CreatePaymentRequest) (payment.CreatePaymentResponse, error) {
				gatewayCalled = true
				return payment.CreatePaymentResponse{}, nil
			},
		}

		svc := v1service.NewPaymentService(mockPaymentRepo, toOrderReader(mockOrderRepo), nil, map[string]payment.PaymentProvider{"momo": mockProvider}, nil, nil, "https://store.com/return", "https://api.com/webhooks")
		dto, err := svc.CreatePaymentIntent(ctx, int32(1), "127.0.0.1", "idem-dup-key", v1dto.CreatePaymentIntentInput{
			OrderUUID: orderUUID,
			Provider:  "momo",
		})

		require.NoError(t, err)
		assert.Equal(t, existingURL, dto.PaymentURL)
		assert.False(t, gatewayCalled, "Gateway must not be called when another request already generated the URL")
	})

	t.Run("Case 13: Step 2 Duplicate Key (23505) -> Returns 409 Conflict When Initialization In Progress", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:            10,
					OrderNumber:        "ORD_STEP123",
					OrderUuid:          orderUUID,
					OrderUserID:        1,
					OrderPaymentStatus: v1service.OrderPaymentStatusUnpaid,
					OrderStatus:        v1service.OrderStatusPending,
					OrderPaymentMethod: v1service.OrderPaymentMethodOnline,
					OrderTotalAmount:   utils.FloatToNumeric(75000),
				}, nil
			},
		}

		mockPaymentRepo := &testutils.MockPaymentRepository{
			GetByIdempotencyKeyFunc: func(ctx context.Context, tx pgx.Tx, idempotencyKey string) (sqlc.PaymentTransaction, error) {
				// Provider URL is still nil (in flight)
				return sqlc.PaymentTransaction{
					PtID:             88,
					PtUuid:           uuid.New(),
					PtOrderID:        10,
					PtIdempotencyKey: idempotencyKey,
					PtProvider:       "momo",
					PtProviderUrl:    nil,
					PtStatus:         v1service.PaymentStatusPending,
				}, nil
			},
			CreateFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.CreatePaymentTransactionParams) (sqlc.PaymentTransaction, error) {
				return sqlc.PaymentTransaction{}, &pgconn.PgError{Code: "23505"}
			},
		}

		mockProvider := &testutils.MockPaymentProvider{
			CreatePaymentURLFunc: func(ctx context.Context, req payment.CreatePaymentRequest) (payment.CreatePaymentResponse, error) {
				return payment.CreatePaymentResponse{}, nil
			},
		}

		svc := v1service.NewPaymentService(mockPaymentRepo, toOrderReader(mockOrderRepo), nil, map[string]payment.PaymentProvider{"momo": mockProvider}, nil, nil, "https://store.com/return", "https://api.com/webhooks")
		_, err := svc.CreatePaymentIntent(ctx, int32(1), "127.0.0.1", "idem-in-progress", v1dto.CreatePaymentIntentInput{
			OrderUUID: orderUUID,
			Provider:  "momo",
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeConflict)
		assert.Contains(t, err.Error(), "initialization is currently in progress")
	})

	t.Run("Case 14: Step 5 Gateway Timeout -> Preserves Pending Status For Reconciliation Without Marking Failed", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:            10,
					OrderNumber:        "ORD_STEP123",
					OrderUuid:          orderUUID,
					OrderUserID:        1,
					OrderPaymentStatus: v1service.OrderPaymentStatusUnpaid,
					OrderStatus:        v1service.OrderStatusPending,
					OrderPaymentMethod: v1service.OrderPaymentMethodOnline,
					OrderTotalAmount:   utils.FloatToNumeric(75000),
				}, nil
			},
		}

		markedFailed := false
		ptUUID := uuid.New()

		mockPaymentRepo := &testutils.MockPaymentRepository{
			GetByIdempotencyKeyFunc: func(ctx context.Context, tx pgx.Tx, idempotencyKey string) (sqlc.PaymentTransaction, error) {
				return sqlc.PaymentTransaction{}, pgx.ErrNoRows
			},
			CreateFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.CreatePaymentTransactionParams) (sqlc.PaymentTransaction, error) {
				return sqlc.PaymentTransaction{
					PtID:             99,
					PtUuid:           ptUUID,
					PtOrderID:        params.PtOrderID,
					PtIdempotencyKey: params.PtIdempotencyKey,
					PtProvider:       params.PtProvider,
					PtTxnRef:         params.PtTxnRef,
					PtAmount:         params.PtAmount,
					PtCurrency:       params.PtCurrency,
					PtStatus:         v1service.PaymentStatusPending,
				}, nil
			},
			UpdateStatusAtomicallyFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.UpdatePaymentStatusAtomicallyParams) (sqlc.PaymentTransaction, error) {
				if params.NewStatus == v1service.PaymentStatusFailed {
					markedFailed = true
				}
				return sqlc.PaymentTransaction{}, nil
			},
		}

		mockProvider := &testutils.MockPaymentProvider{
			CreatePaymentURLFunc: func(ctx context.Context, req payment.CreatePaymentRequest) (payment.CreatePaymentResponse, error) {
				return payment.CreatePaymentResponse{}, errors.New("gateway request timeout (context deadline exceeded)")
			},
		}

		svc := v1service.NewPaymentService(mockPaymentRepo, toOrderReader(mockOrderRepo), nil, map[string]payment.PaymentProvider{"momo": mockProvider}, nil, nil, "https://store.com/return", "https://api.com/webhooks")
		_, err := svc.CreatePaymentIntent(ctx, int32(1), "127.0.0.1", "idem-timeout-test", v1dto.CreatePaymentIntentInput{
			OrderUUID: orderUUID,
			Provider:  "momo",
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeServiceUnavailable)
		assert.Contains(t, err.Error(), "pending reconciliation")
		assert.False(t, markedFailed, "Transaction must remain in Pending status for reconciliation, must not be marked Failed")
	})
}

func TestPaymentService_HandleWebhook(t *testing.T) {
	ctx := context.Background()

	t.Run("Case 1: Unsupported Provider -> Returns 400", func(t *testing.T) {
		svc := v1service.NewPaymentService(nil, nil, nil, map[string]payment.PaymentProvider{}, nil, nil, "", "")
		err := svc.HandleWebhook(ctx, "invalid_provider", []byte("{}"), map[string]string{})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
	})

	t.Run("Case 2: Invalid Webhook Signature -> Returns 401", func(t *testing.T) {
		mockProvider := &testutils.MockPaymentProvider{
			VerifyWebhookFunc: func(payload []byte, headers map[string]string) error {
				return errors.New("signature mismatch")
			},
		}

		providers := map[string]payment.PaymentProvider{"momo": mockProvider}
		svc := v1service.NewPaymentService(nil, nil, nil, providers, nil, nil, "", "")
		err := svc.HandleWebhook(ctx, "momo", []byte("{}"), map[string]string{})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeUnauthorized)
	})

	t.Run("Case 3: Duplicate Webhook Event -> Acknowledges with nil (no error)", func(t *testing.T) {
		mockProvider := &testutils.MockPaymentProvider{
			VerifyWebhookFunc: func(payload []byte, headers map[string]string) error {
				return nil
			},
			ParseWebhookPayloadFunc: func(payload []byte) (payment.WebhookResult, error) {
				return payment.WebhookResult{
					EventID:     "EVT_123",
					OrderNumber: "ORD_123",
					Status:      payment.TxnStatusSuccess,
				}, nil
			},
		}
		mockWebhookRepo := &testutils.MockWebhookEventRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.InsertWebhookEventParams) (sqlc.WebhookEvent, error) {
				return sqlc.WebhookEvent{}, pgx.ErrNoRows // duplicate ON CONFLICT DO NOTHING
			},
		}

		providers := map[string]payment.PaymentProvider{"momo": mockProvider}
		svc := v1service.NewPaymentService(nil, nil, mockWebhookRepo, providers, nil, nil, "", "")
		err := svc.HandleWebhook(ctx, "momo", []byte("{}"), map[string]string{})

		require.NoError(t, err)
	})

	t.Run("Case 4: Happy Path -> Inserts Event and Publishes to RabbitMQ", func(t *testing.T) {
		mockProvider := &testutils.MockPaymentProvider{
			VerifyWebhookFunc: func(payload []byte, headers map[string]string) error {
				return nil
			},
			ParseWebhookPayloadFunc: func(payload []byte) (payment.WebhookResult, error) {
				return payment.WebhookResult{
					EventID:       "EVT_456",
					EventType:     "payment.success",
					OrderNumber:   "ORD456_1700000",
					ProviderTxnID: "TRANS_999",
					Amount:        100000,
					Status:        payment.TxnStatusSuccess,
				}, nil
			},
		}
		mockWebhookRepo := &testutils.MockWebhookEventRepository{
			InsertEventFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.InsertWebhookEventParams) (sqlc.WebhookEvent, error) {
				return sqlc.WebhookEvent{
					WeID:              1,
					WeProvider:        params.WeProvider,
					WeExternalEventID: params.WeExternalEventID,
				}, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByOrderNumberFunc: func(ctx context.Context, orderNumber string) (sqlc.GetOrderByOrderNumberRow, error) {
				return sqlc.GetOrderByOrderNumberRow{
					OrderID:     1,
					OrderNumber: "ORD456",
					OrderUuid:   uuid.New(),
				}, nil
			},
		}
		published := false
		mockRabbitMQ := &testutils.MockRabbitMQService{
			PublishFunc: func(ctx context.Context, queue string, message any) error {
				published = true
				return nil
			},
		}

		providers := map[string]payment.PaymentProvider{"momo": mockProvider}
		svc := v1service.NewPaymentService(nil, toOrderReader(mockOrderRepo), mockWebhookRepo, providers, nil, mockRabbitMQ, "", "")
		err := svc.HandleWebhook(ctx, "momo", []byte(`{"event":"test"}`), map[string]string{})

		require.NoError(t, err)
		assert.True(t, published)
	})
}

func TestPaymentService_GetPaymentHistory(t *testing.T) {
	ctx := context.Background()
	orderUUID := uuid.New()

	t.Run("Case 1: Order Not Found -> Returns 404", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewPaymentService(nil, toOrderReader(mockOrderRepo), nil, nil, nil, nil, "", "")
		_, err := svc.GetPaymentHistory(ctx, int32(1), v1service.RoleMember, orderUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Forbidden for non-owner and non-admin -> Returns 403", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:     10,
					OrderUuid:   orderUUID,
					OrderUserID: 999, // Someone else's order
				}, nil
			},
		}

		svc := v1service.NewPaymentService(nil, toOrderReader(mockOrderRepo), nil, nil, nil, nil, "", "")
		_, err := svc.GetPaymentHistory(ctx, int32(1), v1service.RoleMember, orderUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeForbidden)
	})

	t.Run("Case 3: Happy Path (Owner) -> Returns Transactions", func(t *testing.T) {
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:     10,
					OrderUuid:   orderUUID,
					OrderUserID: 1,
				}, nil
			},
		}
		mockPaymentRepo := &testutils.MockPaymentRepository{
			GetByOrderIDFunc: func(ctx context.Context, tx pgx.Tx, orderID int32) ([]sqlc.PaymentTransaction, error) {
				return []sqlc.PaymentTransaction{
					{
						PtID:        1,
						PtUuid:      uuid.New(),
						PtOrderID:   10,
						PtProvider:  "momo",
						PtAmount:    utils.FloatToNumeric(50000),
						PtCurrency:  "VND",
						PtStatus:    v1service.PaymentStatusPaid,
						PtCreatedAt: time.Now(),
					},
				}, nil
			},
		}

		svc := v1service.NewPaymentService(mockPaymentRepo, toOrderReader(mockOrderRepo), nil, nil, nil, nil, "", "")
		history, err := svc.GetPaymentHistory(ctx, int32(1), v1service.RoleMember, orderUUID)

		require.NoError(t, err)
		assert.Len(t, history, 1)
		assert.Equal(t, "momo", history[0].Provider)
		assert.Equal(t, "Paid", history[0].Status)
	})
}

func TestPaymentService_RefundPayment_Validation(t *testing.T) {
	ctx := context.Background()

	t.Run("Case 1: Invalid Order UUID -> Returns 400", func(t *testing.T) {
		svc := v1service.NewPaymentService(nil, nil, nil, nil, nil, nil, "", "")
		_, err := svc.RefundPayment(ctx, "key123", v1dto.RefundPaymentInput{
			OrderUUID: "not-a-valid-uuid",
			Amount:    50000,
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "Invalid order UUID")
	})
}

func TestPaymentService_CreatePaymentIntent_OrderExpired(t *testing.T) {
	ctx := context.Background()
	orderUUID := uuid.New()

	mockOrderRepo := &testutils.MockOrderRepository{
		GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
			return sqlc.GetOrderByUUIDRow{
				OrderID:            100,
				OrderUuid:          orderUUID,
				OrderUserID:        10,
				OrderStatus:        v1service.OrderStatusPending,
				OrderPaymentStatus: v1service.OrderPaymentStatusUnpaid,
				OrderPaymentMethod: v1service.OrderPaymentMethodOnline,
				OrderTotalAmount:   utils.FloatToNumeric(100000),
				OrderCreatedAt:     time.Now().Add(-20 * time.Minute), // Expired > 15m
			}, nil
		},
	}

	svc := v1service.NewPaymentService(nil, toOrderReader(mockOrderRepo), nil, nil, nil, nil, "", "")
	_, err := svc.CreatePaymentIntent(ctx, int32(10), "127.0.0.1", "key-123", v1dto.CreatePaymentIntentInput{
		OrderUUID: orderUUID,
		Provider:  "momo",
	})

	require.Error(t, err)
	assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
	assert.Contains(t, err.Error(), "Order payment window (15 minutes) has expired. Please place a new order.")
}

type standaloneOrderReader struct {
	order *v1service.PaymentOrderSummary
	err   error
}

func (s *standaloneOrderReader) GetByUUID(ctx context.Context, orderUUID uuid.UUID) (*v1service.PaymentOrderSummary, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.order, nil
}

func (s *standaloneOrderReader) GetByOrderNumber(ctx context.Context, orderNumber string) (*v1service.PaymentOrderSummary, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.order, nil
}

func TestPaymentService_DecoupledOrderReaderPort(t *testing.T) {
	ctx := context.Background()
	orderUUID := uuid.New()

	reader := &standaloneOrderReader{
		order: &v1service.PaymentOrderSummary{
			OrderID:       100,
			OrderUUID:     orderUUID,
			OrderNumber:   "ORD-DECOUPLE-001",
			UserID:        10,
			TotalAmount:   250000,
			PaymentStatus: v1service.OrderPaymentStatusUnpaid,
			OrderStatus:   v1service.OrderStatusPending,
			PaymentMethod: v1service.OrderPaymentMethodOnline,
			CreatedAt:     time.Now(),
		},
	}

	mockPaymentRepo := &testutils.MockPaymentRepository{
		GetByIdempotencyKeyFunc: func(ctx context.Context, tx pgx.Tx, idempotencyKey string) (sqlc.PaymentTransaction, error) {
			return sqlc.PaymentTransaction{}, pgx.ErrNoRows
		},
		CreateFunc: func(ctx context.Context, tx pgx.Tx, params sqlc.CreatePaymentTransactionParams) (sqlc.PaymentTransaction, error) {
			return sqlc.PaymentTransaction{
				PtID:             1,
				PtUuid:           uuid.New(),
				PtOrderID:        params.PtOrderID,
				PtIdempotencyKey: params.PtIdempotencyKey,
				PtProvider:       params.PtProvider,
				PtTxnRef:         params.PtTxnRef,
				PtProviderUrl:    params.PtProviderUrl,
				PtStatus:         params.PtStatus,
				PtAmount:         params.PtAmount,
				PtCreatedAt:      time.Now(),
			}, nil
		},
	}

	mockProvider := &testutils.MockPaymentProvider{
		CreatePaymentURLFunc: func(ctx context.Context, req payment.CreatePaymentRequest) (payment.CreatePaymentResponse, error) {
			return payment.CreatePaymentResponse{PaymentURL: "https://pay.momo.vn/123"}, nil
		},
	}

	// Note: reader is a pure PaymentOrderReaderPort, NOT an OrderRepository
	svc := v1service.NewPaymentService(mockPaymentRepo, reader, nil, map[string]payment.PaymentProvider{"momo": mockProvider}, nil, nil, "", "")

	intent, err := svc.CreatePaymentIntent(ctx, int32(10), "127.0.0.1", "idem-decouple-1", v1dto.CreatePaymentIntentInput{
		OrderUUID: orderUUID,
		Provider:  "momo",
	})

	require.NoError(t, err)
	assert.Equal(t, orderUUID.String(), intent.OrderUUID)
	assert.Equal(t, "https://pay.momo.vn/123", intent.PaymentURL)
}

func TestPaymentService_OrderReaderAdapter(t *testing.T) {
	ctx := context.Background()
	orderUUID := uuid.New()
	userUUID := uuid.New()

	mockOrderRepo := &testutils.MockOrderRepository{
		GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
			return sqlc.GetOrderByUUIDRow{
				OrderID:             99,
				OrderUuid:           orderUUID,
				OrderNumber:         "ORD-ADAPT-001",
				OrderUserID:         12,
				UserUuid:            userUUID,
				OrderTotalAmount:    utils.FloatToNumeric(350000),
				OrderPaymentStatus:  v1service.OrderPaymentStatusUnpaid,
				OrderStatus:         v1service.OrderStatusPending,
				OrderPaymentMethod:  v1service.OrderPaymentMethodOnline,
				OrderRefundedAmount: utils.FloatToNumeric(0),
				OrderCreatedAt:      time.Now(),
			}, nil
		},
		GetByOrderNumberFunc: func(ctx context.Context, orderNumber string) (sqlc.GetOrderByOrderNumberRow, error) {
			return sqlc.GetOrderByOrderNumberRow{
				OrderID:             99,
				OrderUuid:           orderUUID,
				OrderNumber:         orderNumber,
				OrderUserID:         12,
				UserUuid:            userUUID,
				OrderTotalAmount:    utils.FloatToNumeric(350000),
				OrderPaymentStatus:  v1service.OrderPaymentStatusUnpaid,
				OrderStatus:         v1service.OrderStatusPending,
				OrderPaymentMethod:  v1service.OrderPaymentMethodOnline,
				OrderRefundedAmount: utils.FloatToNumeric(0),
				OrderCreatedAt:      time.Now(),
			}, nil
		},
	}

	adapter := v1service.NewOrderReaderAdapter(mockOrderRepo)
	require.NotNil(t, adapter)

	// Verify GetByUUID
	summary, err := adapter.GetByUUID(ctx, orderUUID)
	require.NoError(t, err)
	assert.Equal(t, int32(99), summary.OrderID)
	assert.Equal(t, orderUUID, summary.OrderUUID)
	assert.Equal(t, "ORD-ADAPT-001", summary.OrderNumber)
	assert.Equal(t, float64(350000), summary.TotalAmount)
	assert.Equal(t, v1service.OrderPaymentStatusUnpaid, summary.PaymentStatus)

	// Verify GetByOrderNumber
	summaryByNum, err := adapter.GetByOrderNumber(ctx, "ORD-ADAPT-001")
	require.NoError(t, err)
	assert.Equal(t, int32(99), summaryByNum.OrderID)
	assert.Equal(t, "ORD-ADAPT-001", summaryByNum.OrderNumber)
	assert.Equal(t, float64(350000), summaryByNum.TotalAmount)

	// Verify underlying repo accessor for backward-compatible modules
	if provider, ok := adapter.(interface{ OrderRepository() repository.OrderRepository }); ok {
		assert.Equal(t, mockOrderRepo, provider.OrderRepository())
	}
}

