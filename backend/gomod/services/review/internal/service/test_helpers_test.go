package service_test

import (
	"context"
	"errors"
	"testing"

	"goshop/shared/pkg/utils"

	"github.com/stretchr/testify/assert"
)

func assertAppErrorCode(t *testing.T, err error, expectedCode utils.ErrorCode) {
	t.Helper()
	var appErr *utils.AppError
	if errors.As(err, &appErr) {
		assert.Equal(t, expectedCode, appErr.Code)
	} else {
		t.Fatalf("expected *utils.AppError, got %T: %v", err, err)
	}
}

// noopTxManager runs the callback inline (no real transaction) so unit tests
// exercise service logic without a database.
type noopTxManager struct{}

func (noopTxManager) WithTx(ctx context.Context, fn func(txCtx context.Context) error) error {
	return fn(ctx)
}

var _ = context.Background
