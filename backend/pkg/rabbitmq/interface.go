package rabbitmq

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	// DefaultTopicExchange is the primary topic exchange for all business domain events
	DefaultTopicExchange = "goshop.events"
)

type RabbitMQService interface {
	// Point-to-Point (Work Queue)
	Publish(ctx context.Context, queue string, message any) error
	Consume(ctx context.Context, queue string, handler func(amqp.Table, []byte) error) error

	// Pub/Sub (Topic Exchange)
	PublishTopic(ctx context.Context, routingKey string, message any) error
	PublishToExchange(ctx context.Context, exchange, routingKey string, message any) error
	SubscribeTopic(ctx context.Context, queueName string, routingKeys []string, handler func(amqp.Table, []byte) error) error
	SubscribeToExchange(ctx context.Context, exchange, queueName string, routingKeys []string, handler func(amqp.Table, []byte) error) error

	Close() error
	IsHealthy() bool
}

