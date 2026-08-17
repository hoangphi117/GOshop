package rabbitmq

import (
	"context"
	"encoding/json"
	"goshop/pkg/logger"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
)

type rabbitMQService struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	logger  *zerolog.Logger
}

func NewRabbitMQService(amqpURL string, logger *zerolog.Logger) (RabbitMQService, error) {
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		logger.Error().Err(err).Msg("failed to connect to RabbitMQ")
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		logger.Error().Err(err).Msg("failed to open a channel")
		return nil, err
	}

	return &rabbitMQService{
		conn:    conn,
		channel: ch,
		logger:  logger,
	}, nil
}

func (r *rabbitMQService) Publish(ctx context.Context, queue string, message any) error {
	_, err := r.channel.QueueDeclare(
		queue, // name
		true,  // durability
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		amqp.Table{
			amqp.QueueTypeArg: amqp.QueueTypeQuorum,
		},
	)
	if err != nil {
		r.logger.Error().Err(err).Msg("failed to declare a queue")
		return err
	}

	body, err := json.Marshal(message)
	if err != nil {
		r.logger.Error().Err(err).Msg("failed to marshal message")
		return err
	}

	err = r.channel.PublishWithContext(ctx,
		"",    // exchange
		queue, // routing key
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Headers:     amqp.Table{"trace_id": logger.GetTraceId(ctx)},
			Body:        []byte(body),
		})
	if err != nil {
		r.logger.Error().Err(err).Msg("failed to publish a message")
		return err
	}

	return nil
}

func (r *rabbitMQService) Consume(ctx context.Context, queue string, handler func(amqp.Table, []byte) error) error {
	_, err := r.channel.QueueDeclare(
		queue, // name
		true,  // durability
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		amqp.Table{
			amqp.QueueTypeArg: amqp.QueueTypeQuorum,
		},
	)
	if err != nil {
		r.logger.Error().Err(err).Msg("failed to declare a queue")
		return err
	}

	msgs, err := r.channel.Consume(
		queue, // queue
		"",    // consumer
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		r.logger.Error().Err(err).Msg("failed to register a consumer")
		return err
	}

	go func() {
		for {
			select {
			case msg, ok := <-msgs:
				if !ok {
					return
				}

				if err := handler(msg.Headers, msg.Body); err != nil {
					msg.Nack(false, false)
				} else {
					msg.Ack(false)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (r *rabbitMQService) Close() error {
	if r.channel != nil {
		if err := r.channel.Close(); err != nil {
			r.logger.Error().Err(err).Msg("failed to close RabbitMQ channel")
			return err
		}
	}

	if r.conn != nil {
		if err := r.conn.Close(); err != nil {
			r.logger.Error().Err(err).Msg("failed to close RabbitMQ connection")
			return err
		}
	}
	return nil
}
