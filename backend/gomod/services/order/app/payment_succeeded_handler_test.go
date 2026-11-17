package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goshop/shared/pkg/messaging"
)

type fakeLedger struct {
	processed map[string]bool
}

func newFakeLedger() *fakeLedger { return &fakeLedger{processed: map[string]bool{}} }

func (f *fakeLedger) IsEventProcessed(_ context.Context, eventID, consumer string) (bool, error) {
	return f.processed[consumer+"/"+eventID], nil
}

func (f *fakeLedger) MarkEventProcessed(_ context.Context, eventID, consumer string) error {
	f.processed[consumer+"/"+eventID] = true
	return nil
}

type nopLogger struct{}

func (nopLogger) Info(string, map[string]any)         {}
func (nopLogger) Error(error, string, map[string]any) {}

// The order payment consumer is only idempotent because of this wrapper: the
// consumer's Handle has no dedupe of its own, so a redelivered record applied a
// second time would re-drive the saga. This pins the wiring, not the semantics
// (eventledger.Wrap has its own tests).
func TestPaymentSucceededHandler_RedeliveryAppliesOnce(t *testing.T) {
	ledger := newFakeLedger()
	applied := 0
	handle := func(messaging.Header, []byte) error {
		applied++
		return nil
	}

	handler := paymentSucceededHandler(ledger, nopLogger{}, handle)
	body := []byte(`{"event_id":"evt-order-1","order_uuid":"x"}`)

	require.NoError(t, handler(messaging.Header{}, body))
	require.NoError(t, handler(messaging.Header{}, body))

	assert.Equal(t, 1, applied, "a redelivered payment event must not update the order twice")
	assert.True(t, ledger.processed["order.payment_succeeded/evt-order-1"],
		"the ledger row must be keyed by the order consumer name")
}
