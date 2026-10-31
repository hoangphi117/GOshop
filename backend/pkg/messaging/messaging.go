// Package messaging is the broker-neutral seam every service publishes and
// consumes through. Handlers and payloads never mention a broker type, so
// swapping the transport (RabbitMQ today, Kafka or NATS later) is an adapter
// change, not a service-wide rewrite.
//
// ponytail: only the surface the current services use is modelled (point-to-point
// queues plus topic exchange). Add fan-out/reply-to when a service needs them.
package messaging

import "context"

// Header carries broker-independent message metadata (retry counters, trace
// context, content type). Values stay strings so the same type survives a
// transport swap.
type Header map[string]string

// Get returns the header value, or "" when absent.
func (h Header) Get(key string) string { return h[key] }

// Set stores a header value, allocating the map when nil.
func (h *Header) Set(key, value string) {
	if *h == nil {
		*h = make(Header, 4)
	}
	(*h)[key] = value
}

// Handler applies one delivered message. Returning an error asks the broker to
// redeliver it. The payload is raw JSON; the header is transport-neutral.
type Handler func(headers Header, payload []byte) error

// Broker is what services depend on.
type Broker interface {
	// Point-to-point (work queue): one consumer group, every message applied once.
	Publish(ctx context.Context, queue string, message any) error
	Consume(ctx context.Context, queue string, handler Handler) error

	// Publish/subscribe through a topic exchange.
	PublishTopic(ctx context.Context, routingKey string, message any) error
	PublishToExchange(ctx context.Context, exchange, routingKey string, message any) error
	SubscribeTopic(ctx context.Context, queueName string, routingKeys []string, handler Handler) error
	SubscribeToExchange(ctx context.Context, exchange, queueName string, routingKeys []string, handler Handler) error

	Close() error
	IsHealthy() bool
}
