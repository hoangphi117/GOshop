package worker

import (
	"context"
	"encoding/json"
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/pkg/rabbitmq"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
)

type NotificationConsumer struct {
	notificationService v1service.NotificationService
	logger              *zerolog.Logger
}

func NewNotificationConsumer(notificationService v1service.NotificationService, logger *zerolog.Logger) *NotificationConsumer {
	return &NotificationConsumer{
		notificationService: notificationService,
		logger:              logger,
	}
}

func (nc *NotificationConsumer) Handle(headers amqp.Table, body []byte) error {
	var event rabbitmq.NotificationEvent
	if err := json.Unmarshal(body, &event); err != nil {
		nc.logger.Error().Err(err).Msg("Failed to unmarshal notification event")
		return err
	}

	// 1. Customer Unicast Notification
	if event.UserID != nil {
		nc.logger.Info().
			Str("event_id", event.EventID).
			Int32("user_id", *event.UserID).
			Str("type", event.Type).
			Msg("Processing customer notification event")

		customerInput := v1dto.SendNotificationInput{
			UserID: *event.UserID,
			Type:   event.Type,
			Title:  event.Title,
			Body:   event.Body,
			Data:   event.Data,
		}

		if err := nc.notificationService.Send(context.Background(), customerInput); err != nil {
			nc.logger.Error().Err(err).Str("event_id", event.EventID).Int32("user_id", *event.UserID).Msg("Failed to send customer notification")
			return err
		}
		return nil
	}

	// 2. Admin Multi-Cast Notification
	nc.logger.Info().
		Str("event_id", event.EventID).
		Str("type", event.Type).
		Str("action", event.Action).
		Msg("Processing admin notification event")

	adminInput := v1dto.AdminNotifyInput{
		Type:  event.Type,
		Title: event.Title,
		Body:  event.Body,
		Data:  event.Data,
	}

	if err := nc.notificationService.NotifyAdmins(context.Background(), adminInput); err != nil {
		nc.logger.Error().Err(err).Str("event_id", event.EventID).Msg("Failed to notify admins")
		return err
	}
	return nil
}
