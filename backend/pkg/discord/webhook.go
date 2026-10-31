package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

type AlertSeverity string

const (
	SeverityInfo     AlertSeverity = "INFO"
	SeverityWarning  AlertSeverity = "WARNING"
	SeverityCritical AlertSeverity = "CRITICAL"
)

// Discord embed colors (Decimal integers)
const (
	ColorGreen  = 3066993  // 0x2ECC71 (Healthy / Info)
	ColorYellow = 15844367 // 0xF1C40F (Warning)
	ColorRed    = 15158332 // 0xE74C3C (Critical / Outage)
)

type EmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type EmbedFooter struct {
	Text string `json:"text"`
}

type Embed struct {
	Title       string       `json:"title"`
	Description string       `json:"description,omitempty"`
	Color       int          `json:"color"`
	Fields      []EmbedField `json:"fields,omitempty"`
	Footer      EmbedFooter  `json:"footer,omitempty"`
	Timestamp   string       `json:"timestamp,omitempty"`
}

type WebhookPayload struct {
	Username  string  `json:"username,omitempty"`
	AvatarURL string  `json:"avatar_url,omitempty"`
	Embeds    []Embed `json:"embeds"`
}

type Notifier interface {
	SendAlert(ctx context.Context, title, description string, severity AlertSeverity, fields map[string]string) error
	SendHealthStatus(ctx context.Context, isHealthy bool, services map[string]string, extraInfo string) error
	SetCooldown(duration time.Duration)
}

type discordNotifier struct {
	webhookURL       string
	httpClient       *http.Client
	logger           *zerolog.Logger
	cooldownDuration time.Duration
	alertCache       sync.Map
}

// NewDiscordNotifier creates a Discord webhook notifier with built-in alert cooldown / rate limiting.
func NewDiscordNotifier(webhookURL string, logger *zerolog.Logger) Notifier {
	cooldownSec := 120 // Default: 2 minutes cooldown between identical/burst alerts
	if envSec := os.Getenv("DISCORD_ALERT_COOLDOWN_SEC"); envSec != "" {
		if val, err := strconv.Atoi(envSec); err == nil && val >= 0 {
			cooldownSec = val
		}
	}

	return &discordNotifier{
		webhookURL:       strings.TrimSpace(webhookURL),
		httpClient:       &http.Client{Timeout: 5 * time.Second},
		logger:           logger,
		cooldownDuration: time.Duration(cooldownSec) * time.Second,
	}
}

func (d *discordNotifier) SetCooldown(duration time.Duration) {
	d.cooldownDuration = duration
}

func (d *discordNotifier) shouldSend(alertKey string) bool {
	if d.cooldownDuration <= 0 {
		return true
	}

	now := time.Now()
	if val, ok := d.alertCache.Load(alertKey); ok {
		if lastSent, ok := val.(time.Time); ok {
			if now.Sub(lastSent) < d.cooldownDuration {
				if d.logger != nil {
					d.logger.Debug().
						Str("alert_key", alertKey).
						Dur("cooldown", d.cooldownDuration).
						Msg("Discord alert suppressed due to active cooldown window")
				}
				return false
			}
		}
	}

	d.alertCache.Store(alertKey, now)
	return true
}

func (d *discordNotifier) SendAlert(ctx context.Context, title, description string, severity AlertSeverity, fields map[string]string) error {
	if d.webhookURL == "" {
		return nil
	}

	// Rate limit / debounce alerts by category or title
	alertKey := fmt.Sprintf("alert:%s:%s", severity, title)
	if !d.shouldSend(alertKey) {
		return nil
	}

	color := ColorGreen
	prefix := "ℹ️ [INFO]"
	switch severity {
	case SeverityWarning:
		color = ColorYellow
		prefix = "⚠️ [WARNING]"
	case SeverityCritical:
		color = ColorRed
		prefix = "🚨 [CRITICAL OUTAGE]"
	}

	var embedFields []EmbedField
	for k, v := range fields {
		embedFields = append(embedFields, EmbedField{
			Name:   k,
			Value:  v,
			Inline: true,
		})
	}

	payload := WebhookPayload{
		Username: "GOshop Health Monitor",
		Embeds: []Embed{
			{
				Title:       fmt.Sprintf("%s %s", prefix, title),
				Description: description,
				Color:       color,
				Fields:      embedFields,
				Footer:      EmbedFooter{Text: "GOshop Production Monitor"},
				Timestamp:   time.Now().UTC().Format(time.RFC3339),
			},
		},
	}

	return d.send(ctx, payload)
}

func (d *discordNotifier) SendHealthStatus(ctx context.Context, isHealthy bool, services map[string]string, extraInfo string) error {
	if d.webhookURL == "" {
		return nil
	}

	desc := "All infrastructure components (Database, Redis, RabbitMQ, Storage) are operating normally."
	color := ColorGreen
	prefix := "ℹ️ [INFO]"
	title := "System Services Are Healthy"

	if !isHealthy {
		title = "System Degradation Detected!"
		color = ColorRed
		prefix = "🚨 [CRITICAL OUTAGE]"
		desc = "One or more infrastructure services failed health validation. Please investigate immediately."

		// Apply cooldown for health degradation alerts to prevent spamming during service outages
		if !d.shouldSend("health:degraded") {
			return nil
		}
	} else {
		// Healthy notifications also get debounced to avoid flooding logs
		if !d.shouldSend("health:healthy") {
			return nil
		}
	}

	fields := make(map[string]string)
	for service, status := range services {
		icon := "✅"
		if status != "up" && status != "healthy" {
			icon = "❌"
		}
		fields[service] = fmt.Sprintf("%s %s", icon, strings.ToUpper(status))
	}

	if extraInfo != "" {
		fields["Details"] = extraInfo
	}

	var embedFields []EmbedField
	for k, v := range fields {
		embedFields = append(embedFields, EmbedField{
			Name:   k,
			Value:  v,
			Inline: true,
		})
	}

	payload := WebhookPayload{
		Username: "GOshop Health Monitor",
		Embeds: []Embed{
			{
				Title:       fmt.Sprintf("%s %s", prefix, title),
				Description: desc,
				Color:       color,
				Fields:      embedFields,
				Footer:      EmbedFooter{Text: "GOshop Production Monitor"},
				Timestamp:   time.Now().UTC().Format(time.RFC3339),
			},
		},
	}

	return d.send(ctx, payload)
}

func (d *discordNotifier) send(ctx context.Context, payload WebhookPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal discord payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.webhookURL, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create discord request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		if d.logger != nil {
			d.logger.Warn().Err(err).Msg("Failed to send alert to Discord Webhook")
		}
		return fmt.Errorf("failed to send discord webhook: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if d.logger != nil {
			d.logger.Warn().Int("status_code", resp.StatusCode).Msg("Discord webhook returned non-2xx status")
		}
		return fmt.Errorf("discord webhook returned HTTP %d", resp.StatusCode)
	}

	return nil
}
