package repository

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"

	"goshop/contracts/events"
	ordersqlc "goshop/services/order/internal/db/sqlc"
	"goshop/shared/pkg/db"
)

type sqlOutboxRepository struct {
	db ordersqlc.Querier
}

func NewSqlOutboxRepository(db ordersqlc.Querier) OutboxRepository {
	return &sqlOutboxRepository{db: db}
}

func (r *sqlOutboxRepository) getQuerier(tx pgx.Tx) ordersqlc.Querier {
	if tx != nil {
		return ordersqlc.New(tx)
	}
	return r.db
}

func (r *sqlOutboxRepository) Insert(ctx context.Context, tx pgx.Tx, aggregateType, aggregateID, eventType string, payload any) (ordersqlc.OutboxEvent, error) {
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
			return ordersqlc.OutboxEvent{}, err
		}
	}
	payloadBytes, err = events.EnrichEventPayload(ctx, eventType, aggregateID, payloadBytes)
	if err != nil {
		return ordersqlc.OutboxEvent{}, err
	}

	querier := r.db
	if tx != nil {
		querier = ordersqlc.New(tx)
	} else if ctxTx, ok := db.GetTx(ctx); ok {
		querier = ordersqlc.New(ctxTx)
	}

	return querier.InsertOutboxEvent(ctx, ordersqlc.InsertOutboxEventParams{
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       payloadBytes,
	})
}

func (r *sqlOutboxRepository) Create(ctx context.Context, tx pgx.Tx, aggregateType, aggregateID, eventType string, payload []byte) (ordersqlc.OutboxEvent, error) {
	enrichedPayload, err := events.EnrichEventPayload(ctx, eventType, aggregateID, payload)
	if err != nil {
		return ordersqlc.OutboxEvent{}, err
	}
	return r.getQuerier(tx).InsertOutboxEvent(ctx, ordersqlc.InsertOutboxEventParams{
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       enrichedPayload,
	})
}

func (r *sqlOutboxRepository) GetPending(ctx context.Context, limit int32) ([]ordersqlc.OutboxEvent, error) {
	return r.db.GetPendingOutboxEvents(ctx, limit)
}

func (r *sqlOutboxRepository) GetPendingByAggregateTypes(ctx context.Context, limit int32, aggregateTypes []string) ([]ordersqlc.OutboxEvent, error) {
	return r.db.GetPendingOutboxEventsByAggregateTypes(ctx, ordersqlc.GetPendingOutboxEventsByAggregateTypesParams{
		Limit:          limit,
		AggregateTypes: aggregateTypes,
	})
}

func (r *sqlOutboxRepository) MarkProcessed(ctx context.Context, eventIDs []int64) error {
	return r.db.MarkOutboxEventsProcessed(ctx, eventIDs)
}

func (r *sqlOutboxRepository) MarkFailed(ctx context.Context, eventID int64, lastErr string) error {
	return r.db.MarkOutboxEventFailed(ctx, ordersqlc.MarkOutboxEventFailedParams{
		EventID:   eventID,
		LastError: &lastErr,
	})
}
