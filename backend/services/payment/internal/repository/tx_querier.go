package repository

import (
	"context"

	"goshop/pkg/db"
	paymentsqlc "goshop/services/payment/internal/db/sqlc"
)

// txQuerier returns a querier bound to the transaction carried by ctx, or the
// plain pool-backed querier. Webhook settlement, reconciliation and refunds run
// their writes inside db.WithTx; without this hop the repositories would use
// the pool directly and escape the transaction.
func txQuerier(ctx context.Context, q paymentsqlc.Querier) paymentsqlc.Querier {
	if tx, ok := db.GetTx(ctx); ok {
		return paymentsqlc.New(tx)
	}
	return q
}
