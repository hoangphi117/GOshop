package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EventLedgerRepository persists which events a consumer already applied.
//
// Ordering contract: a row must only be written AFTER every side effect of the
// event succeeded. Any event without a ledger row stays eligible for redelivery.
type EventLedgerRepository interface {
	IsEventProcessed(ctx context.Context, eventID, consumerName string) (bool, error)
	MarkEventProcessed(ctx context.Context, eventID, consumerName string) error
}

type sqlEventLedgerRepository struct {
	db *pgxpool.Pool
}

func NewSqlEventLedgerRepository(db *pgxpool.Pool) EventLedgerRepository {
	return &sqlEventLedgerRepository{db: db}
}

func (r *sqlEventLedgerRepository) IsEventProcessed(ctx context.Context, eventID, consumerName string) (bool, error) {
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

func (r *sqlEventLedgerRepository) MarkEventProcessed(ctx context.Context, eventID, consumerName string) error {
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
