package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const CurrentEventVersion = 1

type eventTraceIDContextKey struct{}

// WithTraceID attaches a trace ID to events created from this context.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, eventTraceIDContextKey{}, traceID)
}

func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return uuid.NewString()
	}
	if traceID, ok := ctx.Value(eventTraceIDContextKey{}).(string); ok && traceID != "" {
		return traceID
	}
	return uuid.NewString()
}

// EnrichEventPayload adds versioned metadata without changing legacy payload fields.
func EnrichEventPayload(ctx context.Context, eventType, aggregateID string, payload []byte) ([]byte, error) {
	var event map[string]json.RawMessage
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("event payload must be a JSON object: %w", err)
	}
	if event == nil {
		return nil, fmt.Errorf("event payload must be a non-null JSON object")
	}

	setString := func(key, value string) {
		if _, exists := event[key]; !exists || string(event[key]) == "null" {
			data, _ := json.Marshal(value)
			event[key] = data
		}
	}

	setInt := func(key string, value int) {
		if _, exists := event[key]; !exists || string(event[key]) == "null" {
			data, _ := json.Marshal(value)
			event[key] = data
		}
	}

	setString("event_id", uuid.NewString())
	setInt("event_version", CurrentEventVersion)
	setString("trace_id", TraceIDFromContext(ctx))
	setString("occurred_at", time.Now().UTC().Format(time.RFC3339Nano))
	setString("event_type", eventType)
	setString("aggregate_id", aggregateID)

	enriched, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event envelope: %w", err)
	}
	return enriched, nil
}
