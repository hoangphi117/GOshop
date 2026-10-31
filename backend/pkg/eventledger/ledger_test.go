package eventledger_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goshop/pkg/eventledger"
	"goshop/pkg/messaging"
)

type fakeRepo struct {
	processed map[string]bool
	failCheck error
	failMark  error
	marks     int
}

func newFakeRepo() *fakeRepo { return &fakeRepo{processed: map[string]bool{}} }

func (f *fakeRepo) IsEventProcessed(_ context.Context, eventID, consumer string) (bool, error) {
	if f.failCheck != nil {
		return false, f.failCheck
	}
	return f.processed[consumer+"/"+eventID], nil
}

func (f *fakeRepo) MarkEventProcessed(_ context.Context, eventID, consumer string) error {
	if f.failMark != nil {
		return f.failMark
	}
	f.marks++
	f.processed[consumer+"/"+eventID] = true
	return nil
}

type fakeLogger struct{ errors int }

func (l *fakeLogger) Info(string, map[string]any)         {}
func (l *fakeLogger) Error(error, string, map[string]any) { l.errors++ }

func envelope(t *testing.T, eventID string) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{"event_id": eventID, "payload": "x"})
	require.NoError(t, err)
	return body
}

// The same event delivered twice applies its side effects exactly once.
func TestWrap_RedeliveryAppliesOnce(t *testing.T) {
	repo := newFakeRepo()
	applied := 0
	handler := eventledger.Wrap(repo, "svc.consumer", func(context.Context, []byte) error {
		applied++
		return nil
	}, &fakeLogger{})

	body := envelope(t, "evt-1")
	require.NoError(t, handler(messaging.Header{}, body))
	require.NoError(t, handler(messaging.Header{}, body))

	assert.Equal(t, 1, applied)
	assert.Equal(t, 1, repo.marks)
}

// A failing side effect must not be recorded: the error propagates so the
// broker redelivers, and the retry applies the effect.
func TestWrap_FailureStaysRetryable(t *testing.T) {
	repo := newFakeRepo()
	var log fakeLogger
	calls := 0
	handler := eventledger.Wrap(repo, "svc.consumer", func(context.Context, []byte) error {
		calls++
		if calls == 1 {
			return errors.New("downstream down")
		}
		return nil
	}, &log)

	body := envelope(t, "evt-2")
	assert.ErrorContains(t, handler(messaging.Header{}, body), "downstream down")
	assert.Equal(t, 0, repo.marks)
	assert.Equal(t, 1, log.errors)

	require.NoError(t, handler(messaging.Header{}, body))
	assert.Equal(t, 1, repo.marks)
}

// A ledger write failure must surface, never acknowledge the event silently.
func TestWrap_MarkFailurePropagates(t *testing.T) {
	repo := newFakeRepo()
	repo.failMark = errors.New("ledger write failed")
	handler := eventledger.Wrap(repo, "svc.consumer", func(context.Context, []byte) error { return nil }, &fakeLogger{})

	assert.ErrorContains(t, handler(messaging.Header{}, envelope(t, "evt-3")), "ledger write failed")
}

// A ledger read failure must stop processing: without the check there is no
// idempotency guarantee.
func TestWrap_LedgerReadFailurePropagates(t *testing.T) {
	repo := newFakeRepo()
	repo.failCheck = errors.New("ledger unavailable")
	applied := false
	handler := eventledger.Wrap(repo, "svc.consumer", func(context.Context, []byte) error {
		applied = true
		return nil
	}, &fakeLogger{})

	assert.ErrorContains(t, handler(messaging.Header{}, envelope(t, "evt-4")), "ledger unavailable")
	assert.False(t, applied)
}

// Legacy payloads without event_id deduplicate by content hash.
func TestEventID_FallsBackToContentHash(t *testing.T) {
	body := []byte(`{"payload":"no-envelope"}`)

	assert.Equal(t, eventledger.EventID(body), eventledger.EventID(body))
	assert.Equal(t, "sha256:", eventledger.EventID(body)[:7])
	assert.Equal(t, "evt-5", eventledger.EventID([]byte(`{"event_id":"evt-5"}`)))
}
