package db

import (
	"context"

	pkgpgx "goshop/pkg/pgx"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Thin re-exports so services can share one transaction manager without
// depending on the monolith sqlc package. The monolith sqlc package is gone.

type TxManager = pkgpgx.TxManager

func NewTxManager(pool *pgxpool.Pool) TxManager {
	return pkgpgx.NewTxManager(pool)
}

func GetTx(ctx context.Context) (pgx.Tx, bool) {
	return pkgpgx.GetTx(ctx)
}

func InjectTx(ctx context.Context, tx pgx.Tx) context.Context {
	return pkgpgx.InjectTx(ctx, tx)
}
