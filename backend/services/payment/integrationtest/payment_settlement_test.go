//go:build integration

package integrationtest_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"goshop/pkg/db"
	paymentsqlc "goshop/services/payment/internal/db/sqlc"
	paymentmigrations "goshop/services/payment/internal/migrations"
	"goshop/services/payment/internal/provider"
	paymentrepo "goshop/services/payment/internal/repository"
	paymentservice "goshop/services/payment/internal/service"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The payment integration suite exercises the real settlement stack (real VNPay
// signature verification, real repositories, real transactions) against an
// ephemeral database. CI provides the Postgres service container; locally the
// defaults match docker-compose.dev.yml credentials.
//
// Run with: go test -tags=integration ./services/payment/integrationtest/

const (
	itDBName  = "shopping_payment_it"
	itTmnCode = "GOSHPIT0"
	itSecret  = "GOshopIntegrationSecret0000000000000000"
	itBaseDSN = "postgres://postgres:super%40password@localhost:5432/postgres?sslmode=disable"
)

type paymentIT struct {
	admin *pgxpool.Pool
	pool  *pgxpool.Pool
	svc   paymentservice.PaymentService
}

// newPaymentIT provisions a fresh database, applies the payment service's own
// migrations, and wires a real PaymentService against it. db.DBPool (the
// package-global the service's TxManager reads) is pointed at the test pool so
// settlement really runs inside transactions.
func newPaymentIT(t *testing.T) *paymentIT {
	t.Helper()

	adminDSN := envOrDefault("INTEGRATION_DATABASE_URL", itBaseDSN)
	admin, err := pgxpool.New(context.Background(), adminDSN)
	require.NoError(t, err)

	// Isolation: a dedicated database per run, recreated from scratch.
	_, err = admin.Exec(context.Background(),
		`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1 AND pid <> pg_backend_pid()`, itDBName)
	require.NoError(t, err)
	_, err = admin.Exec(context.Background(), fmt.Sprintf(`DROP DATABASE IF EXISTS %s WITH (FORCE)`, itDBName))
	require.NoError(t, err)
	_, err = admin.Exec(context.Background(), fmt.Sprintf(`CREATE DATABASE %s`, itDBName))
	require.NoError(t, err)

	dbDSN := replaceDBName(adminDSN, itDBName)

	src, err := iofs.New(paymentmigrations.FS, ".")
	require.NoError(t, err)
	migrator, err := migrate.NewWithSourceInstance("iofs", src,
		dbDSN+"&x-migrations-table=schema_migrations_payment")
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	sourceErr, dbErr := migrator.Close()
	require.NoError(t, sourceErr)
	require.NoError(t, dbErr)

	pool, err := pgxpool.New(context.Background(), dbDSN)
	require.NoError(t, err)

	saved := db.DBPool
	db.DBPool = pool
	t.Cleanup(func() {
		db.DBPool = saved
		pool.Close()
		_, _ = admin.Exec(context.Background(),
			fmt.Sprintf(`DROP DATABASE IF EXISTS %s WITH (FORCE)`, itDBName))
		admin.Close()
	})

	querier := paymentsqlc.New(pool)
	vnpay := provider.NewVNPayProvider(provider.VNPayConfig{
		TmnCode:    itTmnCode,
		HashSecret: itSecret,
	})
	svc := paymentservice.NewPaymentService(
		paymentrepo.NewSqlPaymentRepository(querier),
		nil, // order reader: settlement must tolerate its absence
		paymentrepo.NewSqlWebhookEventRepository(querier),
		paymentrepo.NewSqlOutboxRepository(querier),
		map[string]provider.PaymentProvider{"vnpay": vnpay},
		nil, nil, "", "",
	)

	return &paymentIT{admin: admin, pool: pool, svc: svc}
}

func envOrDefault(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

// replaceDBName swaps the pathname component of a postgres URL, keeping the
// query string (sslmode and friends) intact.
func replaceDBName(dsn, name string) string {
	base, query, _ := strings.Cut(dsn, "?")
	if idx := strings.LastIndex(base, "/"); idx >= 0 {
		base = base[:idx+1] + name
	}
	if query != "" {
		return base + "?" + query
	}
	return base
}

// seedTxn inserts a pending VNPAY transaction with the given whole-dong amount.
func (it *paymentIT) seedTxn(t *testing.T, txnRef string, wholeDong int64) uuid.UUID {
	t.Helper()
	var ptUUID uuid.UUID
	err := it.pool.QueryRow(context.Background(),
		`INSERT INTO payment_transactions
			 (pt_order_id, pt_idempotency_key, pt_provider, pt_txn_ref, pt_amount, pt_currency, pt_status)
		 VALUES (1, $1, 'vnpay', $2, $3::numeric, 'VND', 1)
		 RETURNING pt_uuid`,
		"it-"+txnRef, txnRef, wholeDong,
	).Scan(&ptUUID)
	require.NoError(t, err)
	return ptUUID
}

// signedVnpayPayload reproduces the provider's documented signing contract:
// HMAC-SHA512 over the alphabetically sorted, query-escaped parameters.
func signedVnpayPayload(t *testing.T, params map[string]string) []byte {
	t.Helper()

	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "vnp_SecureHash" || k == "vnp_SecureHashType" || params[k] == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, url.QueryEscape(params[k])))
	}
	mac := hmac.New(sha512.New, []byte(itSecret))
	mac.Write([]byte(strings.Join(pairs, "&")))
	signature := hex.EncodeToString(mac.Sum(nil))

	vals := url.Values{}
	for k, v := range params {
		vals.Set(k, v)
	}
	payload := vals.Encode() + "&vnp_SecureHash=" + signature
	_, err := url.ParseQuery(payload) // must stay a well-formed query
	require.NoError(t, err)
	return []byte(payload)
}

// itSuccessParams builds a gateway-success IPN for the given txn ref.
func itSuccessParams(txnRef string, minorUnits int64) map[string]string {
	return map[string]string{
		"vnp_Amount":            fmt.Sprintf("%d", minorUnits),
		"vnp_BankCode":          "NCB",
		"vnp_BankTranNo":        "IT" + txnRef,
		"vnp_CardType":          "ATM",
		"vnp_OrderInfo":         "IT settlement test",
		"vnp_PayDate":           time.Now().Format("20060102150405"),
		"vnp_ResponseCode":      "00",
		"vnp_TmnCode":           itTmnCode,
		"vnp_TransactionNo":     "15700001",
		"vnp_TransactionStatus": "00",
		"vnp_TxnRef":            txnRef,
	}
}

// itEventID is the VNPay provider's external event id format:
// "vnpay:<order number (our txn ref)>:<provider txn id>".
func itEventID(ref string) string { return "vnpay:" + ref + ":15700001" }

func countQuery(t *testing.T, it *paymentIT, query string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, it.pool.QueryRow(context.Background(), query, args...).Scan(&n))
	return n
}

func txnStatus(t *testing.T, it *paymentIT, txnRef string) (int32, *string, *time.Time) {
	t.Helper()
	var status int32
	var providerTxnID *string
	var paidAt *time.Time
	require.NoError(t, it.pool.QueryRow(context.Background(),
		`SELECT pt_status, pt_provider_txn_id, pt_paid_at FROM payment_transactions WHERE pt_txn_ref = $1`,
		txnRef).Scan(&status, &providerTxnID, &paidAt))
	return status, providerTxnID, paidAt
}

func TestPaymentSettlement_HappyPath(t *testing.T) {
	it := newPaymentIT(t)
	const ref = "IT-HAPPY-01"
	it.seedTxn(t, ref, 150000)

	outcome, err := it.svc.HandleWebhook(context.Background(), "vnpay",
		signedVnpayPayload(t, itSuccessParams(ref, 15_000_000)), nil)
	require.NoError(t, err)
	assert.Equal(t, paymentservice.WebhookProcessed, outcome)

	status, providerTxnID, paidAt := txnStatus(t, it, ref)
	assert.Equal(t, int32(2), status, "transaction must be paid")
	require.NotNil(t, providerTxnID)
	assert.Equal(t, "15700001", *providerTxnID)
	require.NotNil(t, paidAt, "paid_at must be stamped")

	assert.Equal(t, 1, countQuery(t, it,
		`SELECT count(*) FROM webhook_events WHERE we_external_event_id = $1 AND we_processed = true`,
		itEventID(ref)),
		"exactly one processed audit row")

	assert.Equal(t, 1, countQuery(t, it,
		`SELECT count(*) FROM outbox_events WHERE oe_event_type = 'payment.succeeded'`),
		"settlement must write the payment.succeeded outbox event")
}

func TestPaymentSettlement_DuplicateCallbackStopsAtAlreadyProcessed(t *testing.T) {
	it := newPaymentIT(t)
	const ref = "IT-DUP-01"
	it.seedTxn(t, ref, 150000)

	payload := signedVnpayPayload(t, itSuccessParams(ref, 15_000_000))

	outcome, err := it.svc.HandleWebhook(context.Background(), "vnpay", payload, nil)
	require.NoError(t, err)
	require.Equal(t, paymentservice.WebhookProcessed, outcome)

	second, err := it.svc.HandleWebhook(context.Background(), "vnpay", payload, nil)
	require.NoError(t, err)
	assert.Equal(t, paymentservice.WebhookAlreadyProcessed, second)

	status, _, _ := txnStatus(t, it, ref)
	assert.Equal(t, int32(2), status)
	assert.Equal(t, 1, countQuery(t, it,
		`SELECT count(*) FROM webhook_events WHERE we_external_event_id = $1`, itEventID(ref)))
	assert.Equal(t, 1, countQuery(t, it, `SELECT count(*) FROM outbox_events`),
		"exactly one settlement event despite retries")
}

func TestPaymentSettlement_UnderpaymentRejectedAndLeavesNoTrace(t *testing.T) {
	it := newPaymentIT(t)
	const ref = "IT-UNDER-01"
	it.seedTxn(t, ref, 150000)

	// 1,000,000 xu = 10,000 dong, far below the 150,000 dong charged.
	_, err := it.svc.HandleWebhook(context.Background(), "vnpay",
		signedVnpayPayload(t, itSuccessParams(ref, 1_000_000)), nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, paymentservice.ErrWebhookAmountMismatch),
		"error must map to the amount-mismatch sentinel, got: %v", err)

	status, _, _ := txnStatus(t, it, ref)
	assert.Equal(t, int32(1), status, "transaction must remain pending")

	assert.Equal(t, 0, countQuery(t, it, `SELECT count(*) FROM webhook_events`),
		"rejected settlement must roll its audit row back, not poison retries with a duplicate marker")
	assert.Equal(t, 0, countQuery(t, it, `SELECT count(*) FROM outbox_events`),
		"no settlement event may be written for a rejected payment")
}

func TestPaymentSettlement_ConcurrentDuplicateDeliversOnce(t *testing.T) {
	it := newPaymentIT(t)
	const ref = "IT-RACE-01"
	it.seedTxn(t, ref, 150000)

	payload := signedVnpayPayload(t, itSuccessParams(ref, 15_000_000))

	const racers = 4
	var wg sync.WaitGroup
	outcomes := make([]paymentservice.WebhookOutcome, racers)
	errs := make([]error, racers)
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			outcomes[i], errs[i] = it.svc.HandleWebhook(context.Background(), "vnpay", payload, nil)
		}(i)
	}
	wg.Wait()

	processed := 0
	for i := range outcomes {
		require.NoError(t, errs[i], "racer %d", i)
		if outcomes[i] == paymentservice.WebhookProcessed {
			processed++
		}
	}
	assert.Equal(t, 1, processed, "exactly one racer may settle the payment")

	status, _, _ := txnStatus(t, it, ref)
	assert.Equal(t, int32(2), status)
	assert.Equal(t, 1, countQuery(t, it, `SELECT count(*) FROM outbox_events`))
}

func TestPaymentSettlement_TransientFailureDoesNotPoisonRetry(t *testing.T) {
	it := newPaymentIT(t)
	const ref = "IT-RETRY-01"
	it.seedTxn(t, ref, 150000)

	// Block the status transition to emulate a transient constraint/DB fault.
	_, err := it.pool.Exec(context.Background(), `
		CREATE OR REPLACE FUNCTION it_block_txn_update() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'IT: payment_transactions update blocked';
		END;
		$$ LANGUAGE plpgsql`)
	require.NoError(t, err)
	_, err = it.pool.Exec(context.Background(), `
		CREATE TRIGGER it_block_txn_update_trg
		BEFORE UPDATE ON payment_transactions
		FOR EACH ROW EXECUTE FUNCTION it_block_txn_update()`)
	require.NoError(t, err)

	eventID := itEventID(ref)
	_, firstErr := it.svc.HandleWebhook(context.Background(), "vnpay",
		signedVnpayPayload(t, itSuccessParams(ref, 15_000_000)), nil)
	require.Error(t, firstErr, "settlement must fail while the update is blocked")

	// THE invariant: the failed attempt leaves NO audit row behind, so the
	// provider's retry replays the full flow instead of being answered as a
	// duplicate while the payment is still pending.
	assert.Equal(t, 0, countQuery(t, it,
		`SELECT count(*) FROM webhook_events WHERE we_external_event_id = $1`, eventID))
	status, _, _ := txnStatus(t, it, ref)
	assert.Equal(t, int32(1), status, "transaction must stay pending after the failed attempt")

	// Heal the fault and replay the exact same delivery.
	_, err = it.pool.Exec(context.Background(), `DROP TRIGGER it_block_txn_update_trg ON payment_transactions`)
	require.NoError(t, err)

	outcome, retryErr := it.svc.HandleWebhook(context.Background(), "vnpay",
		signedVnpayPayload(t, itSuccessParams(ref, 15_000_000)), nil)
	require.NoError(t, retryErr)
	assert.Equal(t, paymentservice.WebhookProcessed, outcome)

	status, _, _ = txnStatus(t, it, ref)
	assert.Equal(t, int32(2), status, "retry must settle after the transient fault heals")
}

// Guard: resolveTransaction must not be satisfiable by forged callbacks for
// unknown orders, so the handler keeps answering RspCode 01.
func TestPaymentSettlement_UnknownOrderRejects(t *testing.T) {
	it := newPaymentIT(t)

	_, err := it.svc.HandleWebhook(context.Background(), "vnpay",
		signedVnpayPayload(t, itSuccessParams("IT-GHOST-01", 15_000_000)), nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, paymentservice.ErrPaymentTxnNotFound))
}
