package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	v1dto "goshop/internal/dto/v1"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestHTTPIdentityClient_GetUserByID(t *testing.T) {
	testUUID := uuid.New()
	secret := "test-internal-secret"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, secret, r.Header.Get("X-Internal-Token"))

		if r.URL.Path == "/internal/v1/users/42" {
			user := v1dto.InternalUserDTO{
				UserID:       42,
				UserUUID:     testUUID,
				UserFullname: "John Doe",
				UserEmail:    "john@example.com",
				UserStatus:   1,
				UserLevel:    3,
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(user)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewHTTPIdentityClient(server.URL, secret, nil)

	t.Run("success", func(t *testing.T) {
		user, err := client.GetUserByID(context.Background(), 42)
		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, int32(42), user.UserID)
		assert.Equal(t, "John Doe", user.UserFullname)
		assert.Equal(t, "john@example.com", user.UserEmail)
	})

	t.Run("not found", func(t *testing.T) {
		user, err := client.GetUserByID(context.Background(), 999)
		assert.Error(t, err)
		assert.Nil(t, user)
	})

	t.Run("invalid id", func(t *testing.T) {
		user, err := client.GetUserByID(context.Background(), 0)
		assert.Error(t, err)
		assert.Nil(t, user)
	})
}

func TestHTTPIdentityClient_GetUsersBatch(t *testing.T) {
	testUUID1 := uuid.New()
	testUUID2 := uuid.New()
	secret := "test-internal-secret"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, secret, r.Header.Get("X-Internal-Token"))
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/internal/v1/users/batch", r.URL.Path)

		var req v1dto.InternalBatchUsersRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)

		resp := v1dto.InternalBatchUsersResponse{
			Users: map[int32]v1dto.InternalUserDTO{
				1: {
					UserID:       1,
					UserUUID:     testUUID1,
					UserFullname: "Alice",
					UserEmail:    "alice@example.com",
					UserStatus:   1,
					UserLevel:    1,
				},
				2: {
					UserID:       2,
					UserUUID:     testUUID2,
					UserFullname: "Bob",
					UserEmail:    "bob@example.com",
					UserStatus:   1,
					UserLevel:    3,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewHTTPIdentityClient(server.URL, secret, nil)

	t.Run("success batch", func(t *testing.T) {
		users, err := client.GetUsersBatch(context.Background(), []int32{1, 2})
		assert.NoError(t, err)
		assert.Len(t, users, 2)
		assert.Equal(t, "Alice", users[1].UserFullname)
		assert.Equal(t, "Bob", users[2].UserFullname)
	})

	t.Run("empty ids", func(t *testing.T) {
		users, err := client.GetUsersBatch(context.Background(), []int32{})
		assert.NoError(t, err)
		assert.Empty(t, users)
	})
}
