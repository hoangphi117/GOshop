package rabbitmq

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnrichEventPayloadPreservesLegacyFields(t *testing.T) {
	ctx := WithTraceID(context.Background(), "trace-123")

	payload, err := EnrichEventPayload(ctx, "order.created", "order-123", []byte(`{"event_id":"legacy-id","order_id":42}`))
	require.NoError(t, err)

	var event map[string]any
	require.NoError(t, json.Unmarshal(payload, &event))
	require.Equal(t, "legacy-id", event["event_id"])
	require.Equal(t, float64(42), event["order_id"])
	require.Equal(t, float64(CurrentEventVersion), event["event_version"])
	require.Equal(t, "trace-123", event["trace_id"])
	require.Equal(t, "order.created", event["event_type"])
	require.Equal(t, "order-123", event["aggregate_id"])
	require.NotEmpty(t, event["occurred_at"])
}

func TestEnrichEventPayloadRejectsInvalidJSON(t *testing.T) {
	_, err := EnrichEventPayload(context.Background(), "order.created", "order-123", []byte(`not-json`))
	require.Error(t, err)
}
