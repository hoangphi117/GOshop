package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"

	"goshop/shared/pkg/metrics"
)

// The endpoint is what Prometheus scrapes. A typo in the route path is
// invisible until dashboards show "no data", so assert the wire shape: 200,
// text/plain exposition, and at least one metric body when a collector is
// registered.
func TestHandler_ServesExposition(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reg := prometheus.NewRegistry()
	c := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_total", Help: "test"})
	reg.MustRegister(c)
	c.Inc()

	r := gin.New()
	r.GET("/metrics", metrics.Handler(reg))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("content-type = %q, want text/plain prefix", ct)
	}
	if body := w.Body.String(); !strings.Contains(body, "test_total 1") {
		t.Fatalf("body missing counter, got %q", body)
	}
}

// Every service calls metrics.Handler(nil) to reuse the default registry. When
// nil reached promhttp unmodified the route answered 500 and every service
// target showed up as down in Prometheus.
func TestHandler_NilUsesDefaultRegistry(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET("/metrics", metrics.Handler(nil))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (nil must fall back to the default registry)", w.Code)
	}
}
