package repository

import (
	"context"

	coresqlc "goshop/internal/db/sqlc"
	coreRepository "goshop/internal/repository"
	identitysqlc "goshop/services/identity/internal/db/sqlc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type workerOutboxRepository struct {
	repository OutboxRepository
}

func NewWorkerOutboxRepository(repository OutboxRepository) coreRepository.OutboxRepository {
	return &workerOutboxRepository{repository: repository}
}

func (r *workerOutboxRepository) Insert(ctx context.Context, tx pgx.Tx, aggregateType, aggregateID, eventType string, payload any) (coresqlc.OutboxEvent, error) {
	event, err := r.repository.Insert(ctx, tx, aggregateType, aggregateID, eventType, payload)
	if err != nil {
		return coresqlc.OutboxEvent{}, err
	}
	return toCoreOutboxEvent(event), nil
}

func (r *workerOutboxRepository) GetPending(ctx context.Context, limit int32) ([]coresqlc.OutboxEvent, error) {
	events, err := r.repository.GetPending(ctx, limit)
	if err != nil {
		return nil, err
	}
	return toCoreOutboxEvents(events), nil
}

func (r *workerOutboxRepository) GetPendingByAggregateTypes(ctx context.Context, aggregateTypes []string, limit int32) ([]coresqlc.OutboxEvent, error) {
	events, err := r.repository.GetPendingByAggregateTypes(ctx, aggregateTypes, limit)
	if err != nil {
		return nil, err
	}
	return toCoreOutboxEvents(events), nil
}

func (r *workerOutboxRepository) MarkProcessed(ctx context.Context, eventIDs []int64) error {
	return r.repository.MarkProcessed(ctx, eventIDs)
}

func (r *workerOutboxRepository) MarkFailed(ctx context.Context, eventID int64, lastErr *string) error {
	return r.repository.MarkFailed(ctx, eventID, lastErr)
}

func toCoreOutboxEvents(events []identitysqlc.OutboxEvent) []coresqlc.OutboxEvent {
	result := make([]coresqlc.OutboxEvent, len(events))
	for i, event := range events {
		result[i] = toCoreOutboxEvent(event)
	}
	return result
}

func toCoreOutboxEvent(event identitysqlc.OutboxEvent) coresqlc.OutboxEvent {
	return coresqlc.OutboxEvent{
		EventID:       event.EventID,
		EventUuid:     uuid.Nil,
		AggregateType: event.AggregateType,
		AggregateID:   event.AggregateID,
		EventType:     event.EventType,
		Payload:       event.Payload,
		Status:        event.Status,
		RetryCount:    event.RetryCount,
		LastError:     event.LastError,
		CreatedAt:     event.CreatedAt,
		ProcessedAt:   event.ProcessedAt,
	}
}
