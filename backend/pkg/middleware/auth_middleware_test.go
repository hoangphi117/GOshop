package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"goshop/pkg/middleware"
	"goshop/internal/testutils"
	"goshop/pkg/auth"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func TestAuthMiddleware_FamilyBlacklist(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Instant Kick-out when family is blacklisted", func(t *testing.T) {
		mockCache := &testutils.MockRedisCacheService{
			ExistsFunc: func(key string) (bool, error) {
				if key == "blacklist:family:revoked-family-123" {
					return true, nil
				}
				return false, nil
			},
		}

		mockTokenSvc := &testutils.MockTokenService{
			ParseTokenFunc: func(tokenString string) (*jwt.Token, jwt.MapClaims, error) {
				return &jwt.Token{Valid: true}, jwt.MapClaims{
					"jti": "some-jti",
					"fid": "revoked-family-123",
				}, nil
			},
		}

		middleware.InitAuthMiddleware(mockTokenSvc, mockCache)

		router := gin.New()
		router.Use(middleware.AuthMiddleware())
		router.GET("/test-protected", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		req, _ := http.NewRequest(http.MethodGet, "/test-protected", nil)
		req.Header.Set("Authorization", "Bearer valid-looking-token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Session has been revoked")
	})

	t.Run("Happy path when family is not blacklisted", func(t *testing.T) {
		mockCache := &testutils.MockRedisCacheService{
			ExistsFunc: func(key string) (bool, error) {
				return false, nil
			},
		}

		mockTokenSvc := &testutils.MockTokenService{
			ParseTokenFunc: func(tokenString string) (*jwt.Token, jwt.MapClaims, error) {
				return &jwt.Token{Valid: true}, jwt.MapClaims{
					"jti": "some-jti",
					"fid": "active-family-456",
				}, nil
			},
			DecryptAccessTokenPayloadFunc: func(tokenString string) (*auth.EncryptedPayload, error) {
				return &auth.EncryptedPayload{
					UserUUID: "user-uuid-1",
					Email:    "test@example.com",
					Role:     1,
				}, nil
			},
		}

		middleware.InitAuthMiddleware(mockTokenSvc, mockCache)

		router := gin.New()
		router.Use(middleware.AuthMiddleware())
		router.GET("/test-protected", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req, _ := http.NewRequest(http.MethodGet, "/test-protected", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}
