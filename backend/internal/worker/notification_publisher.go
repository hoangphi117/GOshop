package worker

import (
	"context"
	"fmt"

	v1dto "goshop/internal/dto/v1"
	"goshop/pkg/rabbitmq"

	"github.com/google/uuid"
)

type rabbitMQNotificationPublisher struct {
	rabbitMQ rabbitmq.RabbitMQService
}

func NewRabbitMQNotificationPublisher(rabbitMQ rabbitmq.RabbitMQService) OrderNotificationPort {
	return &rabbitMQNotificationPublisher{rabbitMQ: rabbitMQ}
}

func (p *rabbitMQNotificationPublisher) Send(ctx context.Context, input v1dto.SendNotificationInput) error {
	event := rabbitmq.NewCustomerNotificationEvent(
		uuid.NewString(),
		input.UserID,
		input.Type,
		"created",
		input.Title,
		input.Body,
		input.Data,
	)
	if err := p.rabbitMQ.PublishTopic(ctx, rabbitmq.RoutingKeyOrderReturnUpdated, event); err != nil {
		return fmt.Errorf("publish customer notification event: %w", err)
	}
	return nil
}

func (p *rabbitMQNotificationPublisher) NotifyAdmins(ctx context.Context, input v1dto.AdminNotifyInput) error {
	event := rabbitmq.NewAdminNotificationEvent(
		uuid.NewString(),
		input.Type,
		"created",
		input.Title,
		input.Body,
		input.Data,
	)
	if err := p.rabbitMQ.PublishTopic(ctx, rabbitmq.RoutingKeyOrderReturnUpdated, event); err != nil {
		return fmt.Errorf("publish admin notification event: %w", err)
	}
	return nil
}
