package rabbitmq

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQService interface {
	Publish(ctx context.Context, queue string, message any) error
	Consume(ctx context.Context, queue string, handler func(amqp.Table, []byte) error) error
	Close() error
}
