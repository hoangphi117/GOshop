package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestInternalAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "super-secure-internal-token-123"

	r := gin.New()
	r.Use(InternalAuthMiddleware(secret))
	r.GET("/internal/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	t.Run("valid token", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/internal/ping", nil)
		req.Header.Set("X-Internal-Token", secret)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("missing token", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/internal/ping", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid token", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/internal/ping", nil)
		req.Header.Set("X-Internal-Token", "wrong-token")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
