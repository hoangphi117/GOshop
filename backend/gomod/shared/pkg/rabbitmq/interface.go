package rabbitmq

import (
	"context"

	"goshop/shared/pkg/messaging"
)

const (
	// DefaultTopicExchange is the primary topic exchange for all business domain events
	DefaultTopicExchange = "goshop.events"
)

// RabbitMQService is the transport contract of this adapter. Services depend on
// messaging.Broker instead; this interface only exists so the concrete type
// keeps a local compile-time contract. Handlers are broker-neutral:
// amqp types must never leak to callers.
type RabbitMQService interface {
	// Point-to-Point (Work Queue)
	Publish(ctx context.Context, queue string, message any) error
	Consume(ctx context.Context, queue string, handler messaging.Handler) error

	// Pub/Sub (Topic Exchange)
	PublishTopic(ctx context.Context, routingKey string, message any) error
	PublishToExchange(ctx context.Context, exchange, routingKey string, message any) error
	SubscribeTopic(ctx context.Context, queueName string, routingKeys []string, handler messaging.Handler) error
	SubscribeToExchange(ctx context.Context, exchange, queueName string, routingKeys []string, handler messaging.Handler) error

	Close() error
	IsHealthy() bool
}
