package v1service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/testutils"
	"goshop/internal/utils"
	"goshop/pkg/payment"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaymentService_CreatePaymentIntent(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	orderUUID := uuid.New()

	t.Run("Case 1: User Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewPaymentService(nil, nil, mockUserRepo, nil, nil, nil, nil, nil, "", "")
		_, err := svc.CreatePaymentIntent(ctx, userUUID, "127.0.0.1", "key123", v1dto.CreatePaymentIntentInput{
			OrderUUID: orderUUID,
			Provider:  "momo",
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Order Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewPaymentService(nil, mockOrderRepo, mockUserRepo, nil, nil, nil, nil, nil, "", "")
		_, err := svc.CreatePaymentIntent(ctx, userUUID, "127.0.0.1", "key123", v1dto.CreatePaymentIntentInput{
			OrderUUID: orderUUID,
			Provider:  "momo",
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 3: Forbidden When User Does Not Own Order -> Returns 403", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:     10,
					OrderUuid:   orderUUID,
					OrderUserID: 999, // Different user
				}, nil
			},
		}

		svc := v1service.NewPaymentService(nil, mockOrderRepo, mockUserRepo, nil, nil, nil, nil, nil, "", "")
		_, err := svc.CreatePaymentIntent(ctx, userUUID, "127.0.0.1", "key123", v1dto.CreatePaymentIntentInput{
			OrderUUID: orderUUID,
			Provider:  "momo",
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeForbidden)
	})

	t.Run("Case 4: Order Already Paid -> Returns 400", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
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

		svc := v1service.NewPaymentService(nil, mockOrderRepo, mockUserRepo, nil, nil, nil, nil, nil, "", "")
		_, err := svc.CreatePaymentIntent(ctx, userUUID, "127.0.0.1", "key123", v1dto.CreatePaymentIntentInput{
			OrderUUID: orderUUID,
			Provider:  "momo",
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "already been paid")
	})

	t.Run("Case 5: Order Status Disallowed -> Returns 400", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
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

		svc := v1service.NewPaymentService(nil, mockOrderRepo, mockUserRepo, nil, nil, nil, nil, nil, "", "")
		_, err := svc.CreatePaymentIntent(ctx, userUUID, "127.0.0.1", "key123", v1dto.CreatePaymentIntentInput{
			OrderUUID: orderUUID,
			Provider:  "momo",
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "does not allow payment")
	})

	t.Run("Case 6: COD Order -> Returns 400", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
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

		svc := v1service.NewPaymentService(nil, mockOrderRepo, mockUserRepo, nil, nil, nil, nil, nil, "", "")
		_, err := svc.CreatePaymentIntent(ctx, userUUID, "127.0.0.1", "key123", v1dto.CreatePaymentIntentInput{
			OrderUUID: orderUUID,
			Provider:  "momo",
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "COD orders")
	})

	t.Run("Case 7: Zero Amount -> Returns 400", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
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

		svc := v1service.NewPaymentService(nil, mockOrderRepo, mockUserRepo, nil, nil, nil, nil, nil, "", "")
		_, err := svc.CreatePaymentIntent(ctx, userUUID, "127.0.0.1", "key123", v1dto.CreatePaymentIntentInput{
			OrderUUID: orderUUID,
			Provider:  "momo",
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "amount is 0")
	})

	t.Run("Case 8: Missing Idempotency Key -> Returns 400", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
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

		svc := v1service.NewPaymentService(nil, mockOrderRepo, mockUserRepo, nil, nil, nil, nil, nil, "", "")
		_, err := svc.CreatePaymentIntent(ctx, userUUID, "127.0.0.1", "   ", v1dto.CreatePaymentIntentInput{
			OrderUUID: orderUUID,
			Provider:  "momo",
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "Missing X-Idempotency-Key")
	})

	t.Run("Case 9: Unsupported Provider -> Returns 400", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
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

		svc := v1service.NewPaymentService(mockPaymentRepo, mockOrderRepo, mockUserRepo, nil, map[string]payment.PaymentProvider{}, nil, nil, nil, "", "")
		_, err := svc.CreatePaymentIntent(ctx, userUUID, "127.0.0.1", "key123", v1dto.CreatePaymentIntentInput{
			OrderUUID: orderUUID,
			Provider:  "unknown_gateway",
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "Unsupported payment provider")
	})

	t.Run("Case 10: Happy Path -> Generates Payment URL & Returns DTO", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
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

		svc := v1service.NewPaymentService(mockPaymentRepo, mockOrderRepo, mockUserRepo, nil, providers, nil, nil, nil, "https://store.com/return", "https://api.com/webhooks")
		dto, err := svc.CreatePaymentIntent(ctx, userUUID, "127.0.0.1", "key123", v1dto.CreatePaymentIntentInput{
			OrderUUID: orderUUID,
			Provider:  "momo",
		})

		require.NoError(t, err)
		assert.Equal(t, orderUUID.String(), dto.OrderUUID)
		assert.Equal(t, "momo", dto.Provider)
		require.NotEmpty(t, dto.PaymentURL)
		assert.Equal(t, "https://gateway.com/pay/123", dto.PaymentURL)
	})

	t.Run("Case 11: Payment Gateway Circuit Breaker Open -> Returns 503 Service Unavailable", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
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

		svc := v1service.NewPaymentService(mockPaymentRepo, mockOrderRepo, mockUserRepo, nil, providers, nil, nil, nil, "https://store.com/return", "https://api.com/webhooks")

		// First 2 failures trip the breaker
		_, _ = svc.CreatePaymentIntent(ctx, userUUID, "127.0.0.1", "key1", v1dto.CreatePaymentIntentInput{OrderUUID: orderUUID, Provider: "momo"})
		_, _ = svc.CreatePaymentIntent(ctx, userUUID, "127.0.0.1", "key2", v1dto.CreatePaymentIntentInput{OrderUUID: orderUUID, Provider: "momo"})

		// 3rd call must immediately fail with ErrCodeServiceUnavailable (503)
		_, err := svc.CreatePaymentIntent(ctx, userUUID, "127.0.0.1", "key3", v1dto.CreatePaymentIntentInput{
			OrderUUID: orderUUID,
			Provider:  "momo",
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeServiceUnavailable)
		assert.Contains(t, err.Error(), "temporarily unavailable")
	})
}

func TestPaymentService_HandleWebhook(t *testing.T) {
	ctx := context.Background()

	t.Run("Case 1: Unsupported Provider -> Returns 400", func(t *testing.T) {
		svc := v1service.NewPaymentService(nil, nil, nil, nil, map[string]payment.PaymentProvider{}, nil, nil, nil, "", "")
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
		svc := v1service.NewPaymentService(nil, nil, nil, nil, providers, nil, nil, nil, "", "")
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
		svc := v1service.NewPaymentService(nil, nil, nil, mockWebhookRepo, providers, nil, nil, nil, "", "")
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
		svc := v1service.NewPaymentService(nil, mockOrderRepo, nil, mockWebhookRepo, providers, nil, nil, mockRabbitMQ, "", "")
		err := svc.HandleWebhook(ctx, "momo", []byte(`{"event":"test"}`), map[string]string{})

		require.NoError(t, err)
		assert.True(t, published)
	})
}

func TestPaymentService_GetPaymentHistory(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	orderUUID := uuid.New()

	t.Run("Case 1: User Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewPaymentService(nil, nil, mockUserRepo, nil, nil, nil, nil, nil, "", "")
		_, err := svc.GetPaymentHistory(ctx, userUUID, orderUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Order Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewPaymentService(nil, mockOrderRepo, mockUserRepo, nil, nil, nil, nil, nil, "", "")
		_, err := svc.GetPaymentHistory(ctx, userUUID, orderUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 3: Forbidden for non-owner and non-admin -> Returns 403", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID, UserLevel: v1service.RoleMember}, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{
					OrderID:     10,
					OrderUuid:   orderUUID,
					OrderUserID: 999, // Someone else's order
				}, nil
			},
		}

		svc := v1service.NewPaymentService(nil, mockOrderRepo, mockUserRepo, nil, nil, nil, nil, nil, "", "")
		_, err := svc.GetPaymentHistory(ctx, userUUID, orderUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeForbidden)
	})

	t.Run("Case 4: Happy Path (Owner) -> Returns Transactions", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID, UserLevel: v1service.RoleMember}, nil
			},
		}
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

		svc := v1service.NewPaymentService(mockPaymentRepo, mockOrderRepo, mockUserRepo, nil, nil, nil, nil, nil, "", "")
		history, err := svc.GetPaymentHistory(ctx, userUUID, orderUUID)

		require.NoError(t, err)
		assert.Len(t, history, 1)
		assert.Equal(t, "momo", history[0].Provider)
		assert.Equal(t, "Paid", history[0].Status)
	})
}

func TestPaymentService_RefundPayment_Validation(t *testing.T) {
	ctx := context.Background()

	t.Run("Case 1: Invalid Order UUID -> Returns 400", func(t *testing.T) {
		svc := v1service.NewPaymentService(nil, nil, nil, nil, nil, nil, nil, nil, "", "")
		_, err := svc.RefundPayment(ctx, "key123", v1dto.RefundPaymentInput{
			OrderUUID: "not-a-valid-uuid",
			Amount:    50000,
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "Invalid order UUID")
	})
}
