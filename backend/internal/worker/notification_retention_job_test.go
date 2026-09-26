package worker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"goshop/internal/testutils"
	"goshop/internal/worker"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestNotificationRetentionJob_RunOnce(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.Nop()

	t.Run("successfully purges stale notifications", func(t *testing.T) {
		mockService := &testutils.MockNotificationService{
			PurgeStaleNotificationsFunc: func(ctx context.Context, retentionDays int) (int64, error) {
				assert.Equal(t, 30, retentionDays)
				return 25, nil
			},
		}

		job := worker.NewNotificationRetentionJob(mockService, &logger, 24*time.Hour, 30)
		count, err := job.RunOnce(ctx)

		assert.NoError(t, err)
		assert.Equal(t, int64(25), count)
	})

	t.Run("returns error when purge service fails", func(t *testing.T) {
		mockService := &testutils.MockNotificationService{
			PurgeStaleNotificationsFunc: func(ctx context.Context, retentionDays int) (int64, error) {
				return 0, errors.New("database connection failed")
			},
		}

		job := worker.NewNotificationRetentionJob(mockService, &logger, 24*time.Hour, 30)
		count, err := job.RunOnce(ctx)

		assert.Error(t, err)
		assert.Equal(t, int64(0), count)
	})
}

func TestNotificationRetentionJob_Start_Cancellation(t *testing.T) {
	logger := zerolog.Nop()
	mockService := &testutils.MockNotificationService{
		PurgeStaleNotificationsFunc: func(ctx context.Context, retentionDays int) (int64, error) {
			return 0, nil
		},
	}

	job := worker.NewNotificationRetentionJob(mockService, &logger, 50*time.Millisecond, 30)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		job.Start(ctx)
		close(done)
	}()

	// Cancel context immediately to verify clean shutdown
	cancel()

	select {
	case <-done:
		// Succeeded in shutting down cleanly
	case <-time.After(1 * time.Second):
		t.Fatal("Job did not stop within expected time after context cancellation")
	}
}
