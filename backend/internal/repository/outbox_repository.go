package repository

import (
	"context"
	"encoding/json"
	"goshop/internal/db"
	"goshop/internal/db/sqlc"

	"github.com/jackc/pgx/v5"
)

type sqlOutboxRepository struct {
	db sqlc.Querier
}

func NewSqlOutboxRepository(db sqlc.Querier) OutboxRepository {
	return &sqlOutboxRepository{db: db}
}

func (r *sqlOutboxRepository) Insert(ctx context.Context, tx pgx.Tx, aggregateType, aggregateID, eventType string, payload any) (sqlc.OutboxEvent, error) {
	var payloadBytes []byte
	var err error

	switch p := payload.(type) {
	case []byte:
		payloadBytes = p
	case json.RawMessage:
		payloadBytes = p
	case string:
		payloadBytes = []byte(p)
	default:
		payloadBytes, err = json.Marshal(payload)
		if err != nil {
			return sqlc.OutboxEvent{}, err
		}
	}

	querier := r.db
	if tx != nil {
		querier = sqlc.New(tx)
	} else if ctxTx, ok := db.GetTx(ctx); ok {
		querier = sqlc.New(ctxTx)
	}

	return querier.InsertOutboxEvent(ctx, sqlc.InsertOutboxEventParams{
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       payloadBytes,
	})
}

func (r *sqlOutboxRepository) GetPending(ctx context.Context, limit int32) ([]sqlc.OutboxEvent, error) {
	return r.db.GetPendingOutboxEvents(ctx, limit)
}

func (r *sqlOutboxRepository) MarkProcessed(ctx context.Context, eventIDs []int64) error {
	if len(eventIDs) == 0 {
		return nil
	}
	return r.db.MarkOutboxEventsProcessed(ctx, eventIDs)
}

func (r *sqlOutboxRepository) MarkFailed(ctx context.Context, eventID int64, lastErr *string) error {
	return r.db.MarkOutboxEventFailed(ctx, sqlc.MarkOutboxEventFailedParams{
		EventID:   eventID,
		LastError: lastErr,
	})
}
