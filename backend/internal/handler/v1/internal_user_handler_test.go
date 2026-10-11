package v1handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/testutils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestInternalUserHandler_GetUserByID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testUUID := uuid.New()

	mockRepo := &testutils.MockUserRepository{
		GetByIDFunc: func(ctx context.Context, id int32) (sqlc.User, error) {
			if id == 1 {
				return sqlc.User{
					UserID:       1,
					UserUuid:     testUUID,
					UserFullname: "John Doe",
					UserEmail:    "john@example.com",
					UserStatus:   1,
					UserLevel:    3,
				}, nil
			}
			return sqlc.User{}, assert.AnError
		},
	}

	handler := NewInternalUserHandler(mockRepo, nil)
	r := gin.New()
	r.GET("/internal/v1/users/:id", handler.GetUserByID)

	t.Run("found user", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/internal/v1/users/1", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var user v1dto.InternalUserDTO
		err := json.NewDecoder(w.Body).Decode(&user)
		assert.NoError(t, err)
		assert.Equal(t, int32(1), user.UserID)
		assert.Equal(t, "John Doe", user.UserFullname)
	})

	t.Run("not found user", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/internal/v1/users/99", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("invalid id", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/internal/v1/users/abc", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestInternalUserHandler_GetBatchUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	u1 := uuid.New()
	u2 := uuid.New()

	mockRepo := &testutils.MockUserRepository{
		GetByIDsFunc: func(ctx context.Context, ids []int32) ([]sqlc.GetUsersByIDsRow, error) {
			var results []sqlc.GetUsersByIDsRow
			for _, id := range ids {
				if id == 1 {
					results = append(results, sqlc.GetUsersByIDsRow{
						UserID:       1,
						UserUuid:     u1,
						UserFullname: "Alice",
						UserEmail:    "alice@example.com",
						UserStatus:   1,
						UserLevel:    1,
					})
				}
				if id == 2 {
					results = append(results, sqlc.GetUsersByIDsRow{
						UserID:       2,
						UserUuid:     u2,
						UserFullname: "Bob",
						UserEmail:    "bob@example.com",
						UserStatus:   1,
						UserLevel:    3,
					})
				}
			}
			return results, nil
		},
	}

	handler := NewInternalUserHandler(mockRepo, nil)
	r := gin.New()
	r.POST("/internal/v1/users/batch", handler.GetBatchUsers)

	t.Run("successful batch", func(t *testing.T) {
		body, _ := json.Marshal(v1dto.InternalBatchUsersRequest{UserIDs: []int32{1, 2}})
		req, _ := http.NewRequest(http.MethodPost, "/internal/v1/users/batch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp v1dto.InternalBatchUsersResponse
		err := json.NewDecoder(w.Body).Decode(&resp)
		assert.NoError(t, err)
		assert.Len(t, resp.Users, 2)
		assert.Equal(t, "Alice", resp.Users[1].UserFullname)
		assert.Equal(t, "Bob", resp.Users[2].UserFullname)
	})

	t.Run("empty list", func(t *testing.T) {
		body, _ := json.Marshal(v1dto.InternalBatchUsersRequest{UserIDs: []int32{}})
		req, _ := http.NewRequest(http.MethodPost, "/internal/v1/users/batch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp v1dto.InternalBatchUsersResponse
		err := json.NewDecoder(w.Body).Decode(&resp)
		assert.NoError(t, err)
		assert.Empty(t, resp.Users)
	})
}
