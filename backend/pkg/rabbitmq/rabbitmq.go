package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"

	"goshop/pkg/logger"
	"goshop/pkg/messaging"
	"goshop/pkg/telemetry"
	"goshop/pkg/utils"
)

const (
	defaultPrefetchCount = 10
	defaultMessageTTL    = 3600000
	defaultMaxRetries    = 3
	defaultConsumerCount = 3
)

type rabbitMQService struct {
	url               string
	conn              *amqp.Connection
	channel           *amqp.Channel
	logger            *zerolog.Logger
	mu                sync.RWMutex
	declareMu         sync.Mutex
	declaredQueues    map[string]struct{}
	declaredExchanges map[string]struct{}
	quit              chan struct{}
}

func NewRabbitMQService(amqpURL string, logger *zerolog.Logger) (RabbitMQService, error) {
	r := &rabbitMQService{
		url:               amqpURL,
		logger:            logger,
		declaredQueues:    make(map[string]struct{}),
		declaredExchanges: make(map[string]struct{}),
		quit:              make(chan struct{}),
	}

	if err := r.connect(); err != nil {
		return nil, err
	}

	go r.reconnectLoop()

	return r, nil
}

func (r *rabbitMQService) connect() error {
	conn, err := amqp.Dial(r.url)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("failed to open channel: %w", err)
	}

	prefetch := utils.GetIntEnv("RABBITMQ_PREFETCH_COUNT", defaultPrefetchCount)
	if err := ch.Qos(prefetch, 0, false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	r.mu.Lock()
	if r.conn != nil {
		_ = r.conn.Close()
	}
	r.conn = conn
	r.channel = ch
	r.declaredQueues = make(map[string]struct{})
	r.declaredExchanges = make(map[string]struct{})
	r.mu.Unlock()

	r.logger.Info().Msg("Connected to RabbitMQ")

	return nil
}

func (r *rabbitMQService) reconnectLoop() {
	for {
		select {
		case <-r.quit:
			return
		default:
		}

		// Đọc r.conn qua mutex thay vì truy cập trực tiếp, tránh race condition
		// với connect() (ghi r.conn có khóa r.mu.Lock()).
		r.mu.RLock()
		conn := r.conn
		r.mu.RUnlock()

		if conn == nil {
			// Có thể rơi vào đây nếu reconnectLoop chạy đúng lúc connection
			// vừa bị đóng (r.conn đã set nil) nhưng connect() chưa kịp gán lại.
			// Chờ một nhịp ngắn rồi kiểm tra lại, tránh busy-loop.
			select {
			case <-r.quit:
				return
			case <-time.After(500 * time.Millisecond):
			}
			continue
		}

		notifyClose := conn.NotifyClose(make(chan *amqp.Error, 1))

		err, ok := <-notifyClose
		if !ok {
			// Channel đóng bình thường (ví dụ do Close() chủ động gọi conn.Close()),
			// không phải mất kết nối ngoài ý muốn -> không cần reconnect.
			return
		}

		r.logger.Warn().Err(err).Msg("RabbitMQ connection closed, reconnecting...")

		r.mu.Lock()
		r.conn = nil
		r.channel = nil
		r.mu.Unlock()

		backoff := 1 * time.Second
		maxBackoff := 30 * time.Second

		for {
			select {
			case <-r.quit:
				return
			default:
			}

			time.Sleep(backoff)

			if err := r.connect(); err != nil {
				r.logger.Error().Err(err).Msg("RabbitMQ reconnect failed")
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			}

			r.logger.Info().Msg("RabbitMQ reconnected successfully")
			break
		}
	}
}

func (r *rabbitMQService) getChannel() (*amqp.Channel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.channel == nil {
		return nil, fmt.Errorf("rabbitmq channel is not available")
	}
	return r.channel, nil
}

func (r *rabbitMQService) IsHealthy() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.conn != nil && !r.conn.IsClosed()
}

func (r *rabbitMQService) declareQueue(queue string, args amqp.Table) error {
	r.declareMu.Lock()
	defer r.declareMu.Unlock()

	if _, ok := r.declaredQueues[queue]; ok {
		return nil
	}

	ch, err := r.getChannel()
	if err != nil {
		return err
	}

	_, err = ch.QueueDeclare(queue, true, false, false, false, args)
	if err != nil {
		return fmt.Errorf("failed to declare queue %s: %w", queue, err)
	}

	r.declaredQueues[queue] = struct{}{}

	return nil
}

func (r *rabbitMQService) ensureQueues(queue string) error {
	ttlMs := utils.GetIntEnv("RABBITMQ_MESSAGE_TTL_MS", defaultMessageTTL)
	dlxKey := deadLetterQueueName(queue)

	err := r.declareQueue(queue, amqp.Table{
		amqp.QueueTypeArg:           amqp.QueueTypeQuorum,
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": dlxKey,
	})
	if err != nil {
		return err
	}

	err = r.declareQueue(dlxKey, amqp.Table{
		amqp.QueueTypeArg: amqp.QueueTypeQuorum,
		"x-message-ttl":   int32(ttlMs),
	})
	if err != nil {
		return err
	}

	return nil
}

func (r *rabbitMQService) Publish(ctx context.Context, queue string, message any) error {
	if err := r.ensureQueues(queue); err != nil {
		return err
	}

	var body []byte
	switch m := message.(type) {
	case []byte:
		body = m
	case json.RawMessage:
		body = m
	default:
		var err error
		body, err = json.Marshal(message)
		if err != nil {
			return fmt.Errorf("failed to marshal message: %w", err)
		}
	}

	traceID := logger.GetTraceId(ctx)
	messageID := fmt.Sprintf("%d", time.Now().UnixNano())
	if traceID == "" {
		traceID = messageID
	}

	ttlMs := utils.GetIntEnv("RABBITMQ_MESSAGE_TTL_MS", defaultMessageTTL)

	ch, err := r.getChannel()
	if err != nil {
		return err
	}

	headers := amqp.Table{
		"trace_id":      traceID,
		"x-message-id":  messageID,
		"x-retry-count": int32(0),
		"x-max-retries": int32(utils.GetIntEnv("RABBITMQ_MAX_RETRIES", defaultMaxRetries)),
	}
	injectTraceContext(ctx, headers)

	err = ch.PublishWithContext(ctx,
		"",
		queue,
		false,
		false,
		amqp.Publishing{
			ContentType:   "application/json",
			DeliveryMode:  amqp.Persistent,
			MessageId:     messageID,
			CorrelationId: traceID,
			Timestamp:     time.Now(),
			Expiration:    strconv.Itoa(ttlMs),
			Headers:       headers,
			Body:          body,
		})

	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

func (r *rabbitMQService) ensureExchange(exchange string, kind string) error {
	r.declareMu.Lock()
	defer r.declareMu.Unlock()

	if _, ok := r.declaredExchanges[exchange]; ok {
		return nil
	}

	ch, err := r.getChannel()
	if err != nil {
		return err
	}

	err = ch.ExchangeDeclare(
		exchange,
		kind,
		true,  // durable
		false, // auto-delete
		false, // internal
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("failed to declare exchange %s (%s): %w", exchange, kind, err)
	}

	r.declaredExchanges[exchange] = struct{}{}
	return nil
}

func (r *rabbitMQService) ensureTopicQueue(exchange, queue string, routingKeys []string) error {
	if err := r.ensureExchange(exchange, amqp.ExchangeTopic); err != nil {
		return err
	}

	if err := r.ensureQueues(queue); err != nil {
		return err
	}

	ch, err := r.getChannel()
	if err != nil {
		return err
	}

	r.declareMu.Lock()
	defer r.declareMu.Unlock()

	for _, key := range routingKeys {
		if err := ch.QueueBind(queue, key, exchange, false, nil); err != nil {
			return fmt.Errorf("failed to bind queue %s to exchange %s with key %s: %w", queue, exchange, key, err)
		}
	}

	return nil
}

func (r *rabbitMQService) PublishToExchange(ctx context.Context, exchange, routingKey string, message any) error {
	if err := r.ensureExchange(exchange, amqp.ExchangeTopic); err != nil {
		return err
	}

	var body []byte
	switch m := message.(type) {
	case []byte:
		body = m
	case json.RawMessage:
		body = m
	default:
		var err error
		body, err = json.Marshal(message)
		if err != nil {
			return fmt.Errorf("failed to marshal message: %w", err)
		}
	}

	traceID := logger.GetTraceId(ctx)
	messageID := fmt.Sprintf("%d", time.Now().UnixNano())
	if traceID == "" {
		traceID = messageID
	}
	ttlMs := utils.GetIntEnv("RABBITMQ_MESSAGE_TTL_MS", defaultMessageTTL)

	ch, err := r.getChannel()
	if err != nil {
		return err
	}

	headers := amqp.Table{
		"trace_id":      traceID,
		"x-message-id":  messageID,
		"x-routing-key": routingKey,
		"x-retry-count": int32(0),
		"x-max-retries": int32(utils.GetIntEnv("RABBITMQ_MAX_RETRIES", defaultMaxRetries)),
	}
	injectTraceContext(ctx, headers)

	err = ch.PublishWithContext(ctx,
		exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:   "application/json",
			DeliveryMode:  amqp.Persistent,
			MessageId:     messageID,
			CorrelationId: traceID,
			Timestamp:     time.Now(),
			Expiration:    strconv.Itoa(ttlMs),
			Headers:       headers,
			Body:          body,
		})

	if err != nil {
		return fmt.Errorf("failed to publish message to exchange %s with key %s: %w", exchange, routingKey, err)
	}

	return nil
}

func (r *rabbitMQService) PublishTopic(ctx context.Context, routingKey string, message any) error {
	exchange := utils.GetEnv("RABBITMQ_TOPIC_EXCHANGE", DefaultTopicExchange)
	return r.PublishToExchange(ctx, exchange, routingKey, message)
}

func (r *rabbitMQService) republish(ctx context.Context, queue string, body []byte, headers amqp.Table) error {
	traceID, _ := headers["trace_id"].(string)
	if traceID == "" {
		traceID = logger.GetTraceId(ctx)
	}
	messageID := fmt.Sprintf("%d", time.Now().UnixNano())
	if traceID == "" {
		traceID = messageID
	}
	if headers == nil {
		headers = amqp.Table{}
	}
	headers["trace_id"] = traceID

	ttlMs := utils.GetIntEnv("RABBITMQ_MESSAGE_TTL_MS", defaultMessageTTL)

	ch, err := r.getChannel()
	if err != nil {
		return err
	}

	err = ch.PublishWithContext(ctx,
		"",
		queue,
		false,
		false,
		amqp.Publishing{
			ContentType:   "application/json",
			DeliveryMode:  amqp.Persistent,
			MessageId:     messageID,
			CorrelationId: traceID,
			Timestamp:     time.Now(),
			Expiration:    strconv.Itoa(ttlMs),
			Headers:       headers,
			Body:          body,
		})

	if err != nil {
		return fmt.Errorf("failed to republish message: %w", err)
	}

	return nil
}

func (r *rabbitMQService) Consume(ctx context.Context, queue string, handler messaging.Handler) error {
	if err := r.ensureQueues(queue); err != nil {
		return err
	}

	ch, err := r.getChannel()
	if err != nil {
		return err
	}

	consumerCount := utils.GetIntEnv("RABBITMQ_CONSUMER_COUNT", defaultConsumerCount)
	if consumerCount < 1 {
		consumerCount = 1
	}

	for i := 0; i < consumerCount; i++ {
		tag := fmt.Sprintf("%s-c%d", queue, i)
		msgs, err := ch.Consume(queue, tag, false, false, false, false, nil)
		if err != nil {
			return fmt.Errorf("failed to register consumer %s: %w", tag, err)
		}
		go r.startConsumer(ctx, queue, tag, msgs, handler)
		r.logger.Info().Str("queue", queue).Str("consumer", tag).Msg("Consumer registered")
	}

	return nil
}

func (r *rabbitMQService) SubscribeToExchange(ctx context.Context, exchange, queue string, routingKeys []string, handler messaging.Handler) error {
	if err := r.ensureTopicQueue(exchange, queue, routingKeys); err != nil {
		return err
	}

	ch, err := r.getChannel()
	if err != nil {
		return err
	}

	consumerCount := utils.GetIntEnv("RABBITMQ_CONSUMER_COUNT", defaultConsumerCount)
	if consumerCount < 1 {
		consumerCount = 1
	}

	for i := 0; i < consumerCount; i++ {
		tag := fmt.Sprintf("%s-c%d", queue, i)
		msgs, err := ch.Consume(queue, tag, false, false, false, false, nil)
		if err != nil {
			return fmt.Errorf("failed to register topic consumer %s: %w", tag, err)
		}
		go r.startConsumer(ctx, queue, tag, msgs, handler)
		r.logger.Info().Str("exchange", exchange).Str("queue", queue).Str("consumer", tag).Msg("Topic consumer registered")
	}

	return nil
}

func (r *rabbitMQService) SubscribeTopic(ctx context.Context, queue string, routingKeys []string, handler messaging.Handler) error {
	exchange := utils.GetEnv("RABBITMQ_TOPIC_EXCHANGE", DefaultTopicExchange)
	return r.SubscribeToExchange(ctx, exchange, queue, routingKeys, handler)
}

func (r *rabbitMQService) startConsumer(ctx context.Context, queue, consumerTag string, msgs <-chan amqp.Delivery, handler messaging.Handler) {
	currentMsgs := msgs

	for {
		stopped := r.consumeLoop(ctx, queue, currentMsgs, handler)
		if stopped {
			return
		}

		r.logger.Warn().Str("queue", queue).Str("consumer", consumerTag).Msg("Consumer channel closed, attempting to re-register")

		for {
			select {
			case <-ctx.Done():
				return
			case <-r.quit:
				return
			default:
			}

			ch, err := r.getChannel()
			if err != nil {
				time.Sleep(time.Second)
				continue
			}

			newMsgs, err := ch.Consume(queue, consumerTag, false, false, false, false, nil)
			if err != nil {
				r.logger.Error().Err(err).Str("queue", queue).Str("consumer", consumerTag).Msg("Failed to re-register consumer, retrying")
				time.Sleep(time.Second)
				continue
			}

			r.logger.Info().Str("queue", queue).Str("consumer", consumerTag).Msg("Consumer re-registered successfully")
			currentMsgs = newMsgs
			break
		}
	}
}

// consumeLoop xử lý message tới khi channel msgs đóng (trả về false, do mất
// kết nối) hoặc ctx bị hủy (trả về true, dừng hẳn theo yêu cầu người gọi).
func (r *rabbitMQService) consumeLoop(ctx context.Context, queue string, msgs <-chan amqp.Delivery, handler messaging.Handler) (stoppedByContext bool) {
	for {
		select {
		case msg, ok := <-msgs:
			if !ok {
				return false
			}

			headers := cloneHeaders(msg.Headers)
			traceID, _ := headers["trace_id"].(string)
			if traceID == "" {
				traceID = msg.CorrelationId
			}
			if traceID == "" {
				traceID = msg.MessageId
			}
			// Continue the publisher's distributed trace when the message
			// carries W3C headers; the plain trace_id stays for log correlation.
			if spanTraceID := telemetry.TraceIDFromContext(extractTraceContext(msg.Headers)); spanTraceID != "" {
				traceID = spanTraceID
			}
			if headers == nil {
				headers = amqp.Table{}
			}
			headers["trace_id"] = traceID
			if _, ok := headers["x-routing-key"]; !ok || headers["x-routing-key"] == "" {
				headers["x-routing-key"] = msg.RoutingKey
			}

			if err := handler(toNeutralHeaders(headers), msg.Body); err != nil {
				retryCount := getRetryCount(headers)
				maxRetries := getMaxRetries(headers)

				r.logger.Warn().
					Err(err).
					Str("trace_id", traceID).
					Int("retry_count", retryCount).
					Int("max_retries", maxRetries).
					Str("queue", queue).
					Msg("Message handler failed")

				if retryCount < maxRetries {
					headers["x-retry-count"] = int32(retryCount + 1)

					if repErr := r.republish(ctx, queue, msg.Body, headers); repErr != nil {
						// Republish thất bại -> KHÔNG Ack message gốc, nếu không
						// message sẽ mất vĩnh viễn (bản gốc bị xóa, bản retry
						// chưa từng tồn tại). Nack + requeue để giữ lại message,
						// broker sẽ đẩy lại cho consumer xử lý tiếp.
						r.logger.Error().Err(repErr).Str("trace_id", traceID).Msg("Failed to republish message, requeueing original")
						_ = msg.Nack(false, true)
						continue
					}

					// Chỉ Ack bản gốc sau khi chắc chắn bản retry đã được
					// broker chấp nhận (publish thành công).
					_ = msg.Ack(false)
					continue
				}

				// Đã vượt quá số lần retry cho phép -> xóa khỏi queue chính,
				// broker sẽ tự động chuyển vào DLQ nhờ x-dead-letter-exchange.
				_ = msg.Nack(false, false)
				continue
			}

			r.logger.Info().
				Str("trace_id", traceID).
				Str("queue", queue).
				Msg("Message processed successfully")

			_ = msg.Ack(false)

		case <-ctx.Done():
			return true
		}
	}
}

func (r *rabbitMQService) Close() error {
	close(r.quit)

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.channel != nil {
		if err := r.channel.Close(); err != nil {
			r.logger.Error().Err(err).Msg("Failed to close RabbitMQ channel")
		}
	}

	if r.conn != nil {
		if err := r.conn.Close(); err != nil {
			r.logger.Error().Err(err).Msg("Failed to close RabbitMQ connection")
			return err
		}
	}

	return nil
}

func deadLetterQueueName(queue string) string {
	return queue + ".dlq"
}

// toNeutralHeaders converts wire headers (AMQP allows arbitrary value types,
// e.g. int32 retry counters) into the broker-neutral string map handlers see.
func toNeutralHeaders(src amqp.Table) messaging.Header {
	dst := make(messaging.Header, len(src))
	for k, v := range src {
		s, ok := v.(string)
		if !ok {
			s = fmt.Sprintf("%v", v)
		}
		dst[k] = s
	}
	return dst
}

func getRetryCount(headers amqp.Table) int {
	v, ok := headers["x-retry-count"]
	if !ok {
		return 0
	}

	switch val := v.(type) {
	case int32:
		return int(val)
	case int:
		return val
	default:
		n, _ := fmt.Sprintf("%v", val), 0
		n2, _ := strconv.Atoi(n)
		return n2
	}
}

func getMaxRetries(headers amqp.Table) int {
	v, ok := headers["x-max-retries"]
	if !ok {
		return defaultMaxRetries
	}

	switch val := v.(type) {
	case int32:
		return int(val)
	case int:
		return val
	default:
		n, _ := fmt.Sprintf("%v", val), 0
		n2, _ := strconv.Atoi(n)
		return n2
	}
}

func cloneHeaders(src amqp.Table) amqp.Table {
	if src == nil {
		return amqp.Table{}
	}

	dst := make(amqp.Table, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
