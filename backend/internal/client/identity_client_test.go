package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestHTTPIdentityClient_GetUserByID(t *testing.T) {
	testUUID := uuid.New()
	secret := "test-internal-secret"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, secret, r.Header.Get("X-Internal-Token"))

		if r.URL.Path == "/internal/v1/users/42" {
			user := InternalUserDTO{
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

		var req struct {
			UserIDs []int32 `json:"user_ids"`
		}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)

		resp := InternalBatchUsersResponse{
			Users: map[int32]InternalUserDTO{
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

func TestHTTPIdentityClient_GetUserByUUID(t *testing.T) {
	testUUID := uuid.New()
	secret := "test-secret"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, secret, r.Header.Get("X-Internal-Token"))
		if r.URL.Path == "/internal/v1/users/uuid/"+testUUID.String() {
			user := InternalUserDTO{
				UserID:       10,
				UserUUID:     testUUID,
				UserFullname: "UUID User",
				UserEmail:    "uuid@example.com",
				UserStatus:   1,
				UserLevel:    2,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(user)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewHTTPIdentityClient(server.URL, secret, nil)

	t.Run("success", func(t *testing.T) {
		user, err := client.GetUserByUUID(context.Background(), testUUID)
		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, int32(10), user.UserID)
		assert.Equal(t, "UUID User", user.UserFullname)
	})

	t.Run("not found", func(t *testing.T) {
		user, err := client.GetUserByUUID(context.Background(), uuid.New())
		assert.Error(t, err)
		assert.Nil(t, user)
	})

	t.Run("nil uuid", func(t *testing.T) {
		user, err := client.GetUserByUUID(context.Background(), uuid.Nil)
		assert.Error(t, err)
		assert.Nil(t, user)
	})
}

func TestHTTPIdentityClient_GetActiveAdminUserIDs(t *testing.T) {
	secret := "test-secret"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, secret, r.Header.Get("X-Internal-Token"))
		if r.URL.Path == "/internal/v1/users/admins" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"admin_ids": []int32{1, 5, 9}})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewHTTPIdentityClient(server.URL, secret, nil)

	t.Run("success", func(t *testing.T) {
		admins, err := client.GetActiveAdminUserIDs(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, []int32{1, 5, 9}, admins)
	})
}

func TestHTTPIdentityClient_Addresses(t *testing.T) {
	testAddrUUID := uuid.New()
	secret := "test-secret"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, secret, r.Header.Get("X-Internal-Token"))
		if r.URL.Path == "/internal/v1/addresses/"+testAddrUUID.String() {
			addr := AddressDTO{
				UUID:        testAddrUUID.String(),
				Name:        "Receiver",
				Phone:       "0912345678",
				Address:     "123 Street",
				FullAddress: "123 Street, District 1, HCMC",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(addr)
			return
		}
		if r.URL.Path == "/internal/v1/users/1/addresses" {
			res := map[string]any{
				"addresses": []AddressDTO{
					{
						UUID:        testAddrUUID.String(),
						Name:        "Receiver",
						Phone:       "0912345678",
						Address:     "123 Street",
						FullAddress: "123 Street, District 1, HCMC",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(res)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewHTTPIdentityClient(server.URL, secret, nil)

	t.Run("get address by uuid", func(t *testing.T) {
		addr, err := client.GetAddressByUUID(context.Background(), 1, testAddrUUID)
		assert.NoError(t, err)
		assert.NotNil(t, addr)
		assert.Equal(t, "Receiver", addr.Name)
	})

	t.Run("get addresses by user id", func(t *testing.T) {
		addrs, err := client.GetAddressesByUserID(context.Background(), 1)
		assert.NoError(t, err)
		assert.Len(t, addrs, 1)
		assert.Equal(t, "Receiver", addrs[0].Name)
	})
}

func TestGRPCIdentityClient_Interface(t *testing.T) {
	c, err := NewGRPCIdentityClient("localhost:50051", nil)
	assert.NoError(t, err)
	assert.Implements(t, (*IdentityClient)(nil), c)
}
