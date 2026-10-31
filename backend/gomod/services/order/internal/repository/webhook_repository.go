package repository

import (
	"context"

	ordersqlc "goshop/services/order/internal/db/sqlc"

	"github.com/jackc/pgx/v5"
)

type SqlWebhookEventRepository struct {
	db ordersqlc.Querier
}

func NewSqlWebhookEventRepository(db ordersqlc.Querier) WebhookEventRepository {
	return &SqlWebhookEventRepository{
		db: db,
	}
}

func (r *SqlWebhookEventRepository) getQuerier(tx pgx.Tx) ordersqlc.Querier {
	if tx != nil {
		return ordersqlc.New(tx)
	}
	return r.db
}

func (r *SqlWebhookEventRepository) InsertEvent(ctx context.Context, tx pgx.Tx, input ordersqlc.InsertWebhookEventParams) (ordersqlc.WebhookEvent, error) {
	return r.getQuerier(tx).InsertWebhookEvent(ctx, input)
}

func (r *SqlWebhookEventRepository) MarkProcessed(ctx context.Context, tx pgx.Tx, eventID int32) error {
	return r.getQuerier(tx).MarkWebhookEventProcessed(ctx, eventID)
}
