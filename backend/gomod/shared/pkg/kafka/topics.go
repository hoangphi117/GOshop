// Package kafka is the Kafka transport adapter behind messaging.Broker.
// Services never import this package directly; they depend on messaging.Broker
// and get it wired in app.go. Topic naming, partition keys and retry policy
// live in sibling files so this file only moves bytes.
package kafka

import (
	"errors"
	"strings"
)

// Header names carried on every Kafka record. Values stay strings so the
// same contract survives a transport swap.
const (
	headerEventID      = "event_id"
	headerEventType    = "event_type" // routing key cũ: "order.created", ...
	headerEventVersion = "event_version"
	headerTraceID      = "trace_id"
	headerAggregateID  = "aggregate_id" // đồng thời là partition key
	headerRetryCount   = "retry_count"
	headerMaxRetries   = "max_retries"
)

// defaultEventVersion mirrors contracts.CurrentEventVersion. Kept as a local
// const so the adapter does not import the contracts module.
const defaultEventVersion = "1"

// dlqSuffix appends the dead-letter topic sibling instead of relying on a
// broker-native DLX, which Kafka does not have.
const dlqSuffix = ".dlq"

// DLQName returns the dead-letter topic for a domain topic.
func DLQName(topic string) string {
	return topic + dlqSuffix
}

// KnownDLQTopics lists the DLQ sibling for every configured domain topic.
// Used by the replay CLI and by kafka-init checks so a new domain cannot ship
// without its dead-letter topic.
func KnownDLQTopics() []string {
	seen := make(map[string]struct{}, len(domainTopics))
	out := make([]string, 0, len(domainTopics))
	for _, spec := range domainTopics {
		name := DLQName(spec.topic)
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

// topicSpec maps one routing key to its Kafka home: domain topic plus the
// aggregate field used as partition key. One topic per aggregate (never one
// topic per event): mở kafka-ui nhìn là hiểu, thêm domain là thêm 1 dòng.
type topicSpec struct {
	topic string
	// keyField is the JSON field in the payload holding the aggregate ID.
	// "" means no usable key (RoundRobin-style distribution).
	keyField string
}

// domainTopics is the single source of truth for topology. Routing keys come
// from contracts/events; the mapping 1:1 mirrors the old AMQP wildcards
// (order.* → goshop.order.v1).
var domainTopics = map[string]topicSpec{
	// order.*
	"order.created":        {topic: "goshop.order.v1", keyField: "order_uuid"},
	"order.cancelled":      {topic: "goshop.order.v1", keyField: "order_uuid"},
	"order.status_changed": {topic: "goshop.order.v1", keyField: "order_uuid"},
	"order.shipped":        {topic: "goshop.order.v1", keyField: "order_uuid"},
	"order.delivered":      {topic: "goshop.order.v1", keyField: "order_uuid"},
	// order_return.*
	"order_return.requested": {topic: "goshop.order-return.v1", keyField: "order_uuid"},
	"order_return.updated":   {topic: "goshop.order-return.v1", keyField: "order_uuid"},
	"order_return.completed": {topic: "goshop.order-return.v1", keyField: "order_uuid"},
	// payment.*
	"payment.expired":   {topic: "goshop.payment.v1", keyField: "order_uuid"},
	"payment.succeeded": {topic: "goshop.payment.v1", keyField: "order_uuid"},
	"payment.failed":    {topic: "goshop.payment.v1", keyField: "order_uuid"},
	"payment.refunded":  {topic: "goshop.payment.v1", keyField: "order_uuid"},
	// user.*
	"user.created": {topic: "goshop.user.v1", keyField: "user_uuid"},
	"user.updated": {topic: "goshop.user.v1", keyField: "user_uuid"},
	"user.deleted": {topic: "goshop.user.v1", keyField: "user_uuid"},
	// inventory.*
	"inventory.stock_low": {topic: "goshop.inventory.v1", keyField: "product_id"},
	// review.*
	"review.reported":     {topic: "goshop.review.v1", keyField: "product_id"},
	"review_rating":       {topic: "goshop.review.v1", keyField: "product_id"},
	"notification_events": {topic: "goshop.notification.v1", keyField: ""},
	// work-queue style (point-to-point commands)
	"auth_email_queue": {topic: "goshop.email.v1", keyField: "user_uuid"},
	"media.uploaded":   {topic: "goshop.media.v1", keyField: "file_key"},
	"shipping.create":  {topic: "goshop.shipping.v1", keyField: "order_uuid"},
	// order service constant (services/order QueueShippingCreate); same topic.
	"shipping.order.create": {topic: "goshop.shipping.v1", keyField: "order_uuid"},
}

// fallbackTopics maps names the publishers use that are NOT routing keys
// (queue names passed to Publish) to their domain topic. This covers the
// Queue* constants in contracts/events.
var fallbackTopics = map[string]topicSpec{
	// work queues
	"auth_email_queue":      {topic: "goshop.email.v1", keyField: "user_uuid"},
	"media.uploaded":        {topic: "goshop.media.v1", keyField: "file_key"},
	"shipping.create":       {topic: "goshop.shipping.v1", keyField: "order_uuid"},
	"shipping.order.create": {topic: "goshop.shipping.v1", keyField: "order_uuid"},
	"review.rating_updated": {topic: "goshop.review.v1", keyField: "product_id"},
}

// TopicForEvent resolves a routing key or queue name to its domain topic and
// key field. Unknown names fall back to a same-named v1 topic so new events
// fail loudly in kafka-init (topic missing) instead of silently landing
// nowhere. Blank names are a wiring bug and return errNameEmpty.
func TopicForEvent(name string) (topic string, keyField string, err error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", "", errNameEmpty
	}
	if spec, ok := domainTopics[name]; ok {
		return spec.topic, spec.keyField, nil
	}
	if spec, ok := fallbackTopics[name]; ok {
		return spec.topic, spec.keyField, nil
	}
	return "goshop." + name + ".v1", "", nil
}

// matchPattern reports whether an AMQP-style routing pattern ("order.*",
// "*.*", or an exact key) matches a concrete routing key. This replaces the
// topic exchange wildcard Kafka does not have.
func matchPattern(pattern, key string) bool {
	p, k := strings.Split(pattern, "."), strings.Split(key, ".")
	if len(p) != len(k) && !(len(p) > 0 && p[len(p)-1] == "#") {
		return false
	}
	for i := 0; i < len(p) && i < len(k); i++ {
		if p[i] == "#" || p[i] == "*" {
			continue
		}
		if p[i] != k[i] {
			return false
		}
	}
	return true
}

// topicsForPattern expands a routing pattern (or exact key) into the set of
// domain topics it can match. "*.*" expands to every topic in the map.
//
// ponytail: a coin "*" wildcard scans the whole map per subscription; the map
// has ~20 entries, revisit only if it grows past a few hundred.
func topicsForPattern(pattern string) ([]string, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil, errNameEmpty
	}
	if !strings.ContainsAny(pattern, "*#") {
		topic, _, err := TopicForEvent(pattern)
		if err != nil {
			return nil, err
		}
		return []string{topic}, nil
	}

	seen := make(map[string]struct{}, len(domainTopics))
	topics := make([]string, 0, 4)
	for key, spec := range domainTopics {
		if !matchPattern(pattern, key) {
			continue
		}
		if _, dup := seen[spec.topic]; dup {
			continue
		}
		seen[spec.topic] = struct{}{}
		topics = append(topics, spec.topic)
	}
	if len(topics) > 0 {
		return topics, nil
	}

	base := strings.TrimSuffix(strings.TrimSuffix(pattern, ".*"), ".#")
	topic, _, err := TopicForEvent(base)
	if err != nil {
		return nil, err
	}
	return []string{topic}, nil
}

// errNameEmpty guards programming errors early instead of publishing
// unroutable records.
var errNameEmpty = errors.New("kafka: empty queue/routing key")
