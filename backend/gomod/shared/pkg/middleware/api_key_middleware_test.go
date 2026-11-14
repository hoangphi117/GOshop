package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// A request that already carries a bearer token passed the edge or the gateway;
// the key must not be demanded a second time, or every host topology without
// nginx (dev-host) fails authenticated calls with 400.
func TestApiKeyMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("API_KEY", "secret-key")

	cases := []struct {
		name    string
		apiKey  string
		bearer  string
		want    int
		reached bool
	}{
		{name: "valid key and no auth", apiKey: "secret-key", want: http.StatusOK, reached: true},
		{name: "bearer without key", bearer: "abc.def.ghi", want: http.StatusOK, reached: true},
		{name: "no key and no bearer", want: http.StatusBadRequest},
		{name: "empty bearer is not proof", bearer: "   ", want: http.StatusBadRequest},
		{name: "wrong key", apiKey: "nope", want: http.StatusUnauthorized},
		{name: "wrong key even with bearer", apiKey: "nope", bearer: "abc.def.ghi", want: http.StatusUnauthorized},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reached := false
			r := gin.New()
			r.Use(ApiKeyMiddleware())
			r.GET("/x", func(ctx *gin.Context) {
				reached = true
				ctx.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			if tc.apiKey != "" {
				req.Header.Set("X-API-KEY", tc.apiKey)
			}
			if tc.bearer != "" {
				req.Header.Set("Authorization", "Bearer "+tc.bearer)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tc.want {
				t.Errorf("status = %d, want %d (body %s)", w.Code, tc.want, w.Body.String())
			}
			if reached != tc.reached {
				t.Errorf("handler reached = %v, want %v", reached, tc.reached)
			}
		})
	}
}
