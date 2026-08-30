package repository

import (
	"context"
	"goshop/internal/db/sqlc"

	"github.com/jackc/pgx/v5"
)

type SqlWebhookEventRepository struct {
	db sqlc.Querier
}

func NewSqlWebhookEventRepository(db sqlc.Querier) WebhookEventRepository {
	return &SqlWebhookEventRepository{
		db: db,
	}
}

func (r *SqlWebhookEventRepository) getQuerier(tx pgx.Tx) sqlc.Querier {
	if tx != nil {
		return sqlc.New(tx)
	}
	return r.db
}

func (r *SqlWebhookEventRepository) InsertEvent(ctx context.Context, tx pgx.Tx, input sqlc.InsertWebhookEventParams) (sqlc.WebhookEvent, error) {
	return r.getQuerier(tx).InsertWebhookEvent(ctx, input)
}

func (r *SqlWebhookEventRepository) MarkProcessed(ctx context.Context, tx pgx.Tx, eventID int32) error {
	return r.getQuerier(tx).MarkWebhookEventProcessed(ctx, eventID)
}
