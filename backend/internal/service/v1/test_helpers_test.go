package v1service_test

import (
	"errors"
	"testing"

	"goshop/internal/utils"

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
