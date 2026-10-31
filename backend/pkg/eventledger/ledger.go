// Package eventledger provides at-least-once event consumption with
// exactly-once side effects, backed by a processed_events table.
//
// Wrap turns any apply function into a broker handler that applies an event at
// most once per consumer: the ledger row is written only after every side
// effect succeeded, so a failure leaves the event retryable (the returned error
// makes the broker redeliver it).
package eventledger

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"goshop/pkg/messaging"
)

// Repository persists which events a consumer already applied.
type Repository interface {
	IsEventProcessed(ctx context.Context, eventID, consumerName string) (bool, error)
	MarkEventProcessed(ctx context.Context, eventID, consumerName string) error
}

type sqlRepository struct {
	db *pgxpool.Pool
}

// NewRepository returns a Postgres-backed ledger writing to the
// processed_events table (see migrations/ in this package).
func NewRepository(db *pgxpool.Pool) Repository {
	return &sqlRepository{db: db}
}

func (r *sqlRepository) IsEventProcessed(ctx context.Context, eventID, consumerName string) (bool, error) {
	var processed bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM processed_events
			WHERE pe_event_id = $1 AND pe_consumer_name = $2
		)`,
		eventID, consumerName,
	).Scan(&processed)
	if err != nil {
		return false, fmt.Errorf("ledger lookup failed for event %s: %w", eventID, err)
	}
	return processed, nil
}

func (r *sqlRepository) MarkEventProcessed(ctx context.Context, eventID, consumerName string) error {
	if _, err := r.db.Exec(ctx,
		`INSERT INTO processed_events (pe_event_id, pe_consumer_name)
		 VALUES ($1, $2)
		 ON CONFLICT ON CONSTRAINT uq_processed_events_event_consumer DO NOTHING`,
		eventID, consumerName,
	); err != nil {
		return fmt.Errorf("ledger insert failed for event %s: %w", eventID, err)
	}
	return nil
}

// EventID returns the durable identity of an event: the envelope's event_id
// when present, otherwise a content hash. Legacy payloads without an event_id
// therefore still deduplicate on redelivery (identical bytes → identical id).
func EventID(body []byte) string {
	var envelope struct {
		EventID string `json:"event_id"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && strings.TrimSpace(envelope.EventID) != "" {
		return strings.TrimSpace(envelope.EventID)
	}
	digest := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(digest[:])
}

// Logger is the minimal logging surface Wrap needs.
type Logger interface {
	Info(msg string, fields map[string]any)
	Error(err error, msg string, fields map[string]any)
}

// ApplyFunc performs the side effects of one event.
type ApplyFunc func(ctx context.Context, body []byte) error

// Wrap builds a broker handler that applies each event at most once for the
// named consumer. Event IDs are present on every log line.
func Wrap(repository Repository, consumerName string, apply ApplyFunc, log Logger) messaging.Handler {
	return func(headers messaging.Header, body []byte) error {
		eventID := EventID(body)
		ctx := context.Background()

		processed, err := repository.IsEventProcessed(ctx, eventID, consumerName)
		if err != nil {
			log.Error(err, "Failed to check event ledger", map[string]any{"event_id": eventID})
			return err
		}
		if processed {
			log.Info("Duplicate event delivery ignored; already processed", map[string]any{"event_id": eventID})
			return nil
		}

		if err := apply(ctx, body); err != nil {
			log.Error(err, "Event processing failed; leaving event retryable", map[string]any{"event_id": eventID})
			return err
		}

		if err := repository.MarkEventProcessed(ctx, eventID, consumerName); err != nil {
			log.Error(err, "Failed to record processed event", map[string]any{"event_id": eventID})
			return err
		}

		log.Info("Event processed and recorded", map[string]any{"event_id": eventID, "consumer": consumerName})
		return nil
	}
}
