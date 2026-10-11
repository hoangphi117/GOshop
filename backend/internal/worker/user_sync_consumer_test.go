package worker_test

import (
	"encoding/json"
	"testing"
	"time"

	"goshop/internal/worker"
	"goshop/pkg/rabbitmq"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
)

func TestUserSyncConsumer_Handle(t *testing.T) {
	consumer := worker.NewUserSyncConsumer(nil)

	t.Run("Valid user updated event", func(t *testing.T) {
		event := rabbitmq.UserSyncEvent{
			EventID:      "evt-123",
			EventType:    rabbitmq.QueueUserUpdated,
			UserID:       10,
			UserUUID:     "29701be4-9c8e-43bb-b250-8e88d6754ffe",
			UserFullname: "Updated Name",
			UserEmail:    "updated@example.com",
			UserStatus:   1,
			UserLevel:    3,
			Timestamp:    time.Now().UTC(),
		}

		body, err := json.Marshal(event)
		assert.NoError(t, err)

		err = consumer.Handle(amqp.Table{}, body)
		assert.NoError(t, err)
	})

	t.Run("Invalid JSON payload returns error without swallowing", func(t *testing.T) {
		invalidBody := []byte("{invalid-json}")
		err := consumer.Handle(amqp.Table{}, invalidBody)
		assert.Error(t, err)
	})
}
