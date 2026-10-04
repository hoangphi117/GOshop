package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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
}

type discordNotifier struct {
	webhookURL string
	httpClient *http.Client
	logger     *zerolog.Logger
}

// NewDiscordNotifier creates a Discord webhook notifier.
func NewDiscordNotifier(webhookURL string, logger *zerolog.Logger) Notifier {
	return &discordNotifier{
		webhookURL: strings.TrimSpace(webhookURL),
		httpClient: &http.Client{Timeout: 5 * time.Second},
		logger:     logger,
	}
}

func (d *discordNotifier) SendAlert(ctx context.Context, title, description string, severity AlertSeverity, fields map[string]string) error {
	if d.webhookURL == "" {
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

	title := "System Services Are Healthy"
	severity := SeverityInfo
	desc := "All infrastructure components (Database, Redis, RabbitMQ, Storage) are operating normally."

	if !isHealthy {
		title = "System Degradation Detected!"
		severity = SeverityCritical
		desc = "One or more infrastructure services failed health validation. Please investigate immediately."
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

	return d.SendAlert(ctx, title, desc, severity, fields)
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
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if d.logger != nil {
			d.logger.Warn().Int("status_code", resp.StatusCode).Msg("Discord webhook returned non-2xx status")
		}
		return fmt.Errorf("discord webhook returned HTTP %d", resp.StatusCode)
	}

	return nil
}
