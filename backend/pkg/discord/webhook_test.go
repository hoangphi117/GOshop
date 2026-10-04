package discord

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	notifier := NewDiscordNotifier(server.URL, nil)

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
}

func TestDiscordNotifier_SendAlert(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := NewDiscordNotifier(server.URL, nil)
	err := notifier.SendAlert(context.Background(), "Low Stock Warning", "Product #10 is low", SeverityWarning, map[string]string{
		"Product ID": "10",
		"Stock":      "2",
	})
	assert.NoError(t, err)
}
