package middleware_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"goshop/internal/middleware"
	"goshop/internal/testutils"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRateLimitMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Passes when request count is within limit", func(t *testing.T) {
		mockCache := &testutils.MockRedisCacheService{
			IncrFunc: func(key string) (int64, error) {
				return 1, nil
			},
			ExpireFunc: func(key string, ttl time.Duration) error {
				return nil
			},
		}

		r := gin.New()
		r.POST("/test", middleware.RateLimit(mockCache, "login", 5, time.Minute), func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		req, _ := http.NewRequest(http.MethodPost, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Blocks with 429 and Retry-After header when limit is exceeded", func(t *testing.T) {
		mockCache := &testutils.MockRedisCacheService{
			IncrFunc: func(key string) (int64, error) {
				return 6, nil // Limit is 5
			},
			TTLFunc: func(key string) (time.Duration, error) {
				return 45 * time.Second, nil
			},
		}

		r := gin.New()
		r.POST("/test", middleware.RateLimit(mockCache, "login", 5, time.Minute), func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		req, _ := http.NewRequest(http.MethodPost, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		assert.Equal(t, "45", w.Header().Get("Retry-After"))
		assert.Contains(t, w.Body.String(), "TOO_MANY_REQUESTS")
	})

	t.Run("Fails open on Redis error", func(t *testing.T) {
		mockCache := &testutils.MockRedisCacheService{
			IncrFunc: func(key string) (int64, error) {
				return 0, errors.New("redis connection refused")
			},
		}

		r := gin.New()
		r.POST("/test", middleware.RateLimit(mockCache, "login", 5, time.Minute), func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		req, _ := http.NewRequest(http.MethodPost, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}
