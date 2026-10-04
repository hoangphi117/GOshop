package discord

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDiscordNotifier_NoWebhookURL(t *testing.T) {
	notifier := NewDiscordNotifier("", nil)
	err := notifier.SendAlert(context.Background(), "Test Alert", "Description", SeverityInfo, nil)
	assert.NoError(t, err)

	err = notifier.SendHealthStatus(context.Background(), true, map[string]string{"database": "up"}, "")
	assert.NoError(t, err)
}

func TestDiscordNotifier_SendHealthStatus(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	notifier := NewDiscordNotifier(server.URL, nil)
	notifier.SetCooldown(0) // Disable cooldown for direct test

	// Test Healthy
	err := notifier.SendHealthStatus(context.Background(), true, map[string]string{
		"database": "up",
		"redis":    "up",
	}, "All systems optimal")
	assert.NoError(t, err)

	// Test Unhealthy / Critical
	err = notifier.SendHealthStatus(context.Background(), false, map[string]string{
		"database": "down",
		"redis":    "up",
	}, "Database connection timed out")
	assert.NoError(t, err)

	assert.Equal(t, int32(2), atomic.LoadInt32(&callCount))
}

func TestDiscordNotifier_CooldownRateLimiting(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := NewDiscordNotifier(server.URL, nil)
	notifier.SetCooldown(100 * time.Millisecond)

	// 1st alert -> Should send
	err := notifier.SendHealthStatus(context.Background(), false, map[string]string{"database": "down"}, "err1")
	assert.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))

	// 2nd alert immediately -> Should be suppressed by cooldown
	err = notifier.SendHealthStatus(context.Background(), false, map[string]string{"database": "down"}, "err2")
	assert.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount), "Second alert should be suppressed within cooldown")

	// Wait for cooldown to expire
	time.Sleep(120 * time.Millisecond)

	// 3rd alert -> Should send again
	err = notifier.SendHealthStatus(context.Background(), false, map[string]string{"database": "down"}, "err3")
	assert.NoError(t, err)
	assert.Equal(t, int32(2), atomic.LoadInt32(&callCount), "Third alert should be sent after cooldown window expires")
}

func TestDiscordNotifier_SendAlert(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := NewDiscordNotifier(server.URL, nil)
	notifier.SetCooldown(0)
	err := notifier.SendAlert(context.Background(), "Low Stock Warning", "Product #10 is low", SeverityWarning, map[string]string{
		"Product ID": "10",
		"Stock":      "2",
	})
	assert.NoError(t, err)
}