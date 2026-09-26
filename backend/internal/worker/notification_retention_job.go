package worker

import (
	"context"
	"time"

	v1service "goshop/internal/service/v1"

	"github.com/rs/zerolog"
)

type NotificationRetentionJob struct {
	service       v1service.NotificationService
	logger        *zerolog.Logger
	interval      time.Duration
	retentionDays int
}

func NewNotificationRetentionJob(
	service v1service.NotificationService,
	logger *zerolog.Logger,
	interval time.Duration,
	retentionDays int,
) *NotificationRetentionJob {
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	if retentionDays <= 0 {
		retentionDays = 30
	}
	return &NotificationRetentionJob{
		service:       service,
		logger:        logger,
		interval:      interval,
		retentionDays: retentionDays,
	}
}

// RunOnce executes a single retention purge cycle.
func (j *NotificationRetentionJob) RunOnce(ctx context.Context) (int64, error) {
	if j.service == nil {
		return 0, nil
	}

	j.logger.Info().
		Int("retention_days", j.retentionDays).
		Msg("Running scheduled notification retention purge")

	deleted, err := j.service.PurgeStaleNotifications(ctx, j.retentionDays)
	if err != nil {
		j.logger.Error().
			Err(err).
			Int("retention_days", j.retentionDays).
			Msg("Failed to purge stale read notifications")
		return 0, err
	}

	j.logger.Info().
		Int64("deleted_count", deleted).
		Int("retention_days", j.retentionDays).
		Msg("Notification retention purge completed successfully")

	return deleted, nil
}

// Start begins the recurring retention loop until context cancellation.
func (j *NotificationRetentionJob) Start(ctx context.Context) {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	// Initial run after brief startup delay
	select {
	case <-ctx.Done():
		return
	case <-time.After(15 * time.Second):
		_, _ = j.RunOnce(ctx)
	}

	for {
		select {
		case <-ctx.Done():
			j.logger.Info().Msg("Notification retention job stopped")
			return
		case <-ticker.C:
			_, _ = j.RunOnce(ctx)
		}
	}
}
