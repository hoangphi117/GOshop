package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"goshop/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestSetTrustedProxies verifies that Gin Engine accurately extracts the real client IP
// from X-Forwarded-For when requests originate from trusted proxy CIDR ranges (Nginx / Docker subnet).
func TestSetTrustedProxies(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name             string
		appEnv           string
		remoteAddr       string // IP of the connecting proxy/client
		xForwardedFor    string // Header injected by Nginx / proxy
		expectedClientIP string // What c.ClientIP() should resolve to
	}{
		{
			name:             "Production - Request from Docker bridge network (172.18.0.2) with valid X-Forwarded-For",
			appEnv:           "production",
			remoteAddr:       "172.18.0.2:45678",
			xForwardedFor:    "203.0.113.195",
			expectedClientIP: "203.0.113.195",
		},
		{
			name:             "Production - Request from private subnet (10.0.1.5) with proxy chain",
			appEnv:           "production",
			remoteAddr:       "10.0.1.5:54321",
			xForwardedFor:    "198.51.100.42, 10.0.1.5",
			expectedClientIP: "198.51.100.42",
		},
		{
			name:             "Production - Request from Untrusted Public IP attempting IP Spoofing",
			appEnv:           "production",
			remoteAddr:       "198.51.100.99:12345", // Untrusted public IP directly connecting to Go
			xForwardedFor:    "8.8.8.8",
			expectedClientIP: "198.51.100.99", // Untrusted proxy ignored, remote address used instead
		},
		{
			name:             "Development - Request from localhost loopback",
			appEnv:           "development",
			remoteAddr:       "127.0.0.1:50000",
			xForwardedFor:    "114.119.128.5",
			expectedClientIP: "114.119.128.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				AppEnv: tt.appEnv,
			}

			r := gin.New()
			if cfg.AppEnv == "production" {
				_ = r.SetTrustedProxies([]string{
					"127.0.0.1",
					"::1",
					"10.0.0.0/8",
					"172.16.0.0/12",
					"192.168.0.0/16",
				})
			} else {
				_ = r.SetTrustedProxies([]string{
					"127.0.0.1",
					"::1",
				})
			}

			var detectedIP string
			r.GET("/test-ip", func(c *gin.Context) {
				detectedIP = c.ClientIP()
				c.String(http.StatusOK, detectedIP)
			})

			req := httptest.NewRequest(http.MethodGet, "/test-ip", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, tt.expectedClientIP, detectedIP, "ClientIP should match expected resolution")
		})
	}
}
