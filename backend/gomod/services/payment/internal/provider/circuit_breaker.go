package provider

import (
	"context"
	"fmt"
	"goshop/shared/pkg/logger"
	"goshop/shared/pkg/utils"
	"time"

	"github.com/sony/gobreaker/v2"
)

// CBConfig defines thresholds for the Circuit Breaker.
type CBConfig struct {
	MaxRequests uint32        // Max requests allowed in half-open state (probe)
	Interval    time.Duration // Cyclic period of the closed state to clear internal counts
	Timeout     time.Duration // Period of open state before switching to half-open
	MaxFailures uint32        // Consecutive failures required to trip the breaker to open
}

// DefaultCBConfig returns recommended defaults for payment gateway circuit breaker.
func DefaultCBConfig() CBConfig {
	maxFailures := uint32(utils.GetIntEnv("PAYMENT_CB_MAX_FAILURES", 5))
	timeout := time.Duration(utils.GetIntEnv("PAYMENT_CB_TIMEOUT_SECONDS", 30)) * time.Second
	maxRequests := uint32(utils.GetIntEnv("PAYMENT_CB_MAX_REQUESTS", 1))

	return CBConfig{
		MaxRequests: maxRequests,
		Interval:    0,
		Timeout:     timeout,
		MaxFailures: maxFailures,
	}
}

// CircuitBreakerReporter exposes circuit breaker state for metrics & healthchecks.
type CircuitBreakerReporter interface {
	CBState() string
	CBName() string
	CBCounts() gobreaker.Counts
}

type circuitBreakerPaymentProvider struct {
	inner        PaymentProvider
	providerName string
	cb           *gobreaker.CircuitBreaker[any]
}

// NewCircuitBreakerPaymentProvider wraps any PaymentProvider with a resilient Circuit Breaker decorator.
func NewCircuitBreakerPaymentProvider(inner PaymentProvider, providerName string, cfg CBConfig) PaymentProvider {
	settings := gobreaker.Settings{
		Name:        fmt.Sprintf("payment:%s", providerName),
		MaxRequests: cfg.MaxRequests,
		Interval:    cfg.Interval,
		Timeout:     cfg.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= cfg.MaxFailures
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			logger.Log.Warn().
				Str("circuit_breaker", name).
				Str("from_state", from.String()).
				Str("to_state", to.String()).
				Msg("Payment gateway circuit breaker state changed")
		},
	}

	return &circuitBreakerPaymentProvider{
		inner:        inner,
		providerName: providerName,
		cb:           gobreaker.NewCircuitBreaker[any](settings),
	}
}

// CreatePaymentURL invokes the gateway API protected by Circuit Breaker.
func (c *circuitBreakerPaymentProvider) CreatePaymentURL(ctx context.Context, req CreatePaymentRequest) (CreatePaymentResponse, error) {
	res, err := c.cb.Execute(func() (any, error) {
		return c.inner.CreatePaymentURL(ctx, req)
	})
	if err != nil {
		return CreatePaymentResponse{}, err
	}
	return res.(CreatePaymentResponse), nil
}

// QueryTransactionStatus queries the gateway API protected by Circuit Breaker.
func (c *circuitBreakerPaymentProvider) QueryTransactionStatus(ctx context.Context, txnRef, transactionDate string) (TransactionStatus, error) {
	res, err := c.cb.Execute(func() (any, error) {
		return c.inner.QueryTransactionStatus(ctx, txnRef, transactionDate)
	})
	if err != nil {
		return "", err
	}
	return res.(TransactionStatus), nil
}

// Refund calls the gateway API protected by Circuit Breaker.
func (c *circuitBreakerPaymentProvider) Refund(ctx context.Context, req RefundRequest) (RefundResponse, error) {
	res, err := c.cb.Execute(func() (any, error) {
		return c.inner.Refund(ctx, req)
	})
	if err != nil {
		return RefundResponse{}, err
	}
	return res.(RefundResponse), nil
}

// VerifyWebhook validates incoming webhook payload signatures locally (no outbound network call).
func (c *circuitBreakerPaymentProvider) VerifyWebhook(payload []byte, headers map[string]string) error {
	return c.inner.VerifyWebhook(payload, headers)
}

// ParseWebhookPayload parses incoming webhook payload locally (no outbound network call).
func (c *circuitBreakerPaymentProvider) ParseWebhookPayload(payload []byte) (WebhookResult, error) {
	return c.inner.ParseWebhookPayload(payload)
}

// CBState returns the current state of the circuit breaker ("closed", "half-open", "open").
func (c *circuitBreakerPaymentProvider) CBState() string {
	return c.cb.State().String()
}

// CBName returns the name identifier of the circuit breaker.
func (c *circuitBreakerPaymentProvider) CBName() string {
	return c.cb.Name()
}

// CBCounts returns current operational counts.
func (c *circuitBreakerPaymentProvider) CBCounts() gobreaker.Counts {
	return c.cb.Counts()
}
