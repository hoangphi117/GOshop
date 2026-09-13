package db

import (
	"context"
	"fmt"
	"goshop/internal/db/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type txKey struct{}

type TxManager interface {
	// WithTx executes the given function "fn" within a database transaction.
	// If the function returns an error, the transaction is rolled back;
	// otherwise, it is committed (err == nil).
	WithTx(ctx context.Context, fn func(txCtx context.Context) error) error
}

// pgxTxManager is an implementation of the TxManager interface using pgxpool.Pool.
type pgxTxManager struct {
	pool *pgxpool.Pool
}

func NewTxManager(pool *pgxpool.Pool) TxManager {
	return &pgxTxManager{pool: pool}
}

func (m *pgxTxManager) WithTx(ctx context.Context, fn func(txCtx context.Context) error) error {

	// If a transaction already exists in the context, use it.
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}

	// Start a new transaction.
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Inject tx into the context
	txCtx := context.WithValue(ctx, txKey{}, tx)

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p) // re-throw panic after Rollback
		}
	}()

	// Execute the function
	if err := fn(txCtx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil && rbErr != pgx.ErrTxClosed {
			return fmt.Errorf("tx err: %v, rollback err: %w", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetQuerier returns a sqlc.Querier that uses the transaction from the context if it exists;
// otherwise, it returns the defaultQuerier.
func GetQuerier(ctx context.Context, defaultQuerier sqlc.Querier) sqlc.Querier {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return sqlc.New(tx)
	}
	return defaultQuerier
}

// GetTx returns pgx.Tx if low-level operations are required (e.g., batch or copy)
func GetTx(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	return tx, ok
}
