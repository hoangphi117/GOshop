package contract

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestHTTPIdentityClient(t *testing.T) {
	testUUID := uuid.New()
	secret := "test-secret"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, secret, r.Header.Get("X-Internal-Token"))

		switch r.URL.Path {
		case "/internal/v1/users/1":
			user := InternalUserDTO{
				UserID:       1,
				UserUUID:     testUUID,
				UserFullname: "John Doe",
				UserEmail:    "john@example.com",
				UserStatus:   1,
				UserLevel:    1,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(user)
		case "/internal/v1/users/uuid/" + testUUID.String():
			user := InternalUserDTO{
				UserID:       1,
				UserUUID:     testUUID,
				UserFullname: "John Doe",
				UserEmail:    "john@example.com",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(user)
		case "/internal/v1/users/admins":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"admin_ids": []int32{1, 2}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewHTTPIdentityClient(server.URL, secret, nil)

	t.Run("get user by id", func(t *testing.T) {
		user, err := client.GetUserByID(context.Background(), 1)
		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, int32(1), user.UserID)
		assert.Equal(t, "John Doe", user.UserFullname)
	})

	t.Run("get user by uuid", func(t *testing.T) {
		user, err := client.GetUserByUUID(context.Background(), testUUID)
		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, "John Doe", user.UserFullname)
	})

	t.Run("get active admins", func(t *testing.T) {
		admins, err := client.GetActiveAdminUserIDs(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, []int32{1, 2}, admins)
	})
}
