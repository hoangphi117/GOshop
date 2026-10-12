package db

import (
	"context"
	"goshop/internal/db/sqlc"
	pkgpgx "goshop/pkg/pgx"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TxManager = pkgpgx.TxManager

func NewTxManager(pool *pgxpool.Pool) TxManager {
	return pkgpgx.NewTxManager(pool)
}

func GetQuerier(ctx context.Context, defaultQuerier sqlc.Querier) sqlc.Querier {
	if tx, ok := pkgpgx.GetTx(ctx); ok {
		return sqlc.New(tx)
	}
	return defaultQuerier
}

func GetTx(ctx context.Context) (pgx.Tx, bool) {
	return pkgpgx.GetTx(ctx)
}

func InjectTx(ctx context.Context, tx pgx.Tx) context.Context {
	return pkgpgx.InjectTx(ctx, tx)
}


