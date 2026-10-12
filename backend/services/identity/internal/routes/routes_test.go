package routes_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"goshop/pkg/auth"
	"goshop/services/identity/internal/handler"
	"goshop/services/identity/internal/routes"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRegisterRoutes_PublicAndJWKS(t *testing.T) {
	t.Setenv("API_KEY", "test-api-key")
	gin.SetMode(gin.TestMode)
	r := gin.New()

	tokenService := auth.NewJWTService(nil)
	authHandler := handler.NewAuthHandler(nil)
	userHandler := handler.NewUserHandler(nil)
	addressHandler := handler.NewAddressHandler(nil)

	routes.RegisterRoutes(r, routes.IdentityRouteDeps{
		AuthHandler:    authHandler,
		UserHandler:    userHandler,
		AddressHandler: addressHandler,
		TokenService:   tokenService,
		CacheService:   nil,
	})

	t.Run("JWKS endpoint is accessible publicly", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/.well-known/jwks.json", http.NoBody)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "keys")
	})

	t.Run("API v1 JWKS endpoint is accessible", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/.well-known/jwks.json", http.NoBody)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Internal REST endpoints are no longer exposed", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/internal/v1/users/1", http.NoBody)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}
