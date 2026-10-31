package rabbitmq

import (
	"context"
	"testing"

	"goshop/contracts/events"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
)

func TestTopicConstantsAndPatterns(t *testing.T) {
	assert.Equal(t, "goshop.events", DefaultTopicExchange)
	assert.Equal(t, "order.created", events.RoutingKeyOrderCreated)
	assert.Equal(t, "user.created", events.RoutingKeyUserCreated)
	assert.Equal(t, "payment.succeeded", events.RoutingKeyPaymentSucceeded)

	assert.Equal(t, "order.*", events.PatternOrderAll)
	assert.Equal(t, "payment.*", events.PatternPaymentAll)
	assert.Equal(t, "user.*", events.PatternUserAll)
	assert.Equal(t, "*.*", events.PatternAllEvents)
}

func TestDeadLetterQueueName(t *testing.T) {
	queue := "order.created"
	dlq := deadLetterQueueName(queue)
	assert.Equal(t, "order.created.dlq", dlq)
}

func TestRetryHeadersHelper(t *testing.T) {
	headers := amqp.Table{
		"x-retry-count": int32(2),
		"x-max-retries": int32(5),
	}

	retryCount := getRetryCount(headers)
	maxRetries := getMaxRetries(headers)

	assert.Equal(t, 2, retryCount)
	assert.Equal(t, 5, maxRetries)

	// Clone headers
	cloned := cloneHeaders(headers)
	assert.Equal(t, headers, cloned)

	// Missing headers default fallback
	emptyHeaders := amqp.Table{}
	assert.Equal(t, 0, getRetryCount(emptyHeaders))
	assert.Equal(t, defaultMaxRetries, getMaxRetries(emptyHeaders))
}

type dummyRabbitMQService struct {
	RabbitMQService
	publishedExchanges []string
	publishedKeys      []string
	subscribedQueues   []string
}

func (d *dummyRabbitMQService) PublishToExchange(ctx context.Context, exchange, routingKey string, message any) error {
	d.publishedExchanges = append(d.publishedExchanges, exchange)
	d.publishedKeys = append(d.publishedKeys, routingKey)
	return nil
}

func (d *dummyRabbitMQService) PublishTopic(ctx context.Context, routingKey string, message any) error {
	return d.PublishToExchange(ctx, DefaultTopicExchange, routingKey, message)
}

func (d *dummyRabbitMQService) SubscribeTopic(ctx context.Context, queueName string, routingKeys []string, handler func(amqp.Table, []byte) error) error {
	d.subscribedQueues = append(d.subscribedQueues, queueName)
	return nil
}

func TestDummyRabbitMQPubSubInterface(t *testing.T) {
	dummy := &dummyRabbitMQService{}

	err := dummy.PublishTopic(context.Background(), events.RoutingKeyOrderCreated, map[string]string{"order_id": "123"})
	assert.NoError(t, err)
	assert.Len(t, dummy.publishedExchanges, 1)
	assert.Equal(t, DefaultTopicExchange, dummy.publishedExchanges[0])
	assert.Equal(t, events.RoutingKeyOrderCreated, dummy.publishedKeys[0])

	err = dummy.SubscribeTopic(context.Background(), "notification.orders", []string{events.PatternOrderAll}, nil)
	assert.NoError(t, err)
	assert.Contains(t, dummy.subscribedQueues, "notification.orders")
}
