package testutils

import "context"

// NoopTxManager is a mock TxManager that executes fn directly without opening a DB transaction.
type NoopTxManager struct{}

func NewNoopTxManager() *NoopTxManager {
	return &NoopTxManager{}
}

func (m *NoopTxManager) WithTx(ctx context.Context, fn func(txCtx context.Context) error) error {
	return fn(ctx)
}
