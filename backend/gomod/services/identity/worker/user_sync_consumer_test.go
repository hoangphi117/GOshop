package identityworker_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"goshop/contracts/events"
	identityworker "goshop/services/identity/worker"
	"goshop/shared/pkg/messaging"
)

func TestUserSyncConsumer_Handle(t *testing.T) {
	consumer := identityworker.NewUserSyncConsumer(nil)

	t.Run("Valid user updated event", func(t *testing.T) {
		event := events.UserSyncEvent{
			EventID:      "evt-123",
			EventType:    events.QueueUserUpdated,
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

		err = consumer.Handle(messaging.Header{}, body)
		assert.NoError(t, err)
	})

	t.Run("Invalid JSON payload returns error without swallowing", func(t *testing.T) {
		invalidBody := []byte("{invalid-json}")
		err := consumer.Handle(messaging.Header{}, invalidBody)
		assert.Error(t, err)
	})
}
