package kafka

// Helper route contract: status shape, threshold flip, nil-safe broker.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubLagBroker struct{ lag map[string]int64 }

func (s stubLagBroker) LagInfo() map[string]int64 { return s.lag }
func (s stubLagBroker) IsHealthy() bool           { return true }

func decodeHealthz(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body
}

func TestRegisterHealthz_HealthyWithZeroLag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterHealthz(r, "order", stubLagBroker{lag: map[string]int64{"g": 0}}, func(context.Context) error { return nil })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := decodeHealthz(t, w)
	assert.Equal(t, "healthy", body["status"])
	assert.Equal(t, "order", body["service"])
}

func TestRegisterHealthz_LagAboveThresholdDegrades(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterHealthz(r, "order", stubLagBroker{lag: map[string]int64{"g": LagAlertThreshold + 1}}, func(context.Context) error { return nil })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	body := decodeHealthz(t, w)
	assert.Equal(t, "degraded", body["status"])
}

func TestRegisterHealthz_NilBrokerReportsDown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterHealthz(r, "cart", nil, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	body := decodeHealthz(t, w)
	assert.Equal(t, "degraded", body["status"])
}

func TestRegisterHealthz_DBPingFailureDegrades(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterHealthz(r, "cart", stubLagBroker{}, func(context.Context) error { return errors.New("db down") })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, "degraded", decodeHealthz(t, w)["status"])
}
