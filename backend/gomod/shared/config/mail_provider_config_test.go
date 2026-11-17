// Assert-based check for the mail provider config: only smtp remains, it reads
// the SMTP_* vars under the "smtp" key, and a space-separated App Password is
// normalised. Run with:
//
//	go test ./config/ -run TestMailProviderConfig -v
package config

import (
	"strings"
	"testing"
)

func TestMailProviderConfig(t *testing.T) {
	t.Run("smtp is the default", func(t *testing.T) {
		t.Setenv("MAIL_PROVIDER_TYPE", "")
		t.Setenv("SMTP_HOST", "smtp.gmail.com")
		t.Setenv("SMTP_PORT", "587")
		t.Setenv("SMTP_USERNAME", "shop@example.com")
		t.Setenv("SMTP_PASSWORD", "app-password")
		t.Setenv("MAIL_SENDER", "shop@example.com")
		t.Setenv("MAIL_SENDER_NAME", "GOshop Admin")

		cfg := NewConfig()
		if cfg.MailProviderType != "smtp" {
			t.Fatalf("provider type = %q, want smtp", cfg.MailProviderType)
		}
		raw, ok := cfg.MailProviderConfig["smtp"]
		if !ok {
			t.Fatalf("MailProviderConfig has no %q key: %v", "smtp", keys(cfg.MailProviderConfig))
		}
		got, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("smtp config is %T, want map[string]any", raw)
		}
		for key, want := range map[string]string{
			"host":             "smtp.gmail.com",
			"port":             "587",
			"username":         "shop@example.com",
			"password":         "app-password",
			"mail_sender":      "shop@example.com",
			"mail_sender_name": "GOshop Admin",
		} {
			if got[key] != want {
				t.Errorf("smtp[%q] = %v, want %q", key, got[key], want)
			}
		}
	})

	t.Run("smtp defaults fill in", func(t *testing.T) {
		t.Setenv("MAIL_PROVIDER_TYPE", "smtp")
		t.Setenv("SMTP_HOST", "")
		t.Setenv("SMTP_PORT", "")
		t.Setenv("MAIL_SENDER_NAME", "")

		got := NewConfig().MailProviderConfig["smtp"].(map[string]any)
		if got["host"] != "smtp.gmail.com" {
			t.Errorf("host default = %v, want smtp.gmail.com", got["host"])
		}
		if got["port"] != "587" {
			t.Errorf("port default = %v, want 587", got["port"])
		}
		if got["mail_sender_name"] != "GOshop Admin" {
			t.Errorf("sender name default = %v, want GOshop Admin", got["mail_sender_name"])
		}
	})

	t.Run("app password spaces are stripped", func(t *testing.T) {
		t.Setenv("MAIL_PROVIDER_TYPE", "smtp")
		t.Setenv("SMTP_PASSWORD", "abcd efgh ijkl mnop")

		got := NewConfig().MailProviderConfig["smtp"].(map[string]any)
		if got["password"] != "abcdefghijklmnop" {
			t.Errorf("password = %v, want the 16 chars without spaces", got["password"])
		}
		if strings.Contains(got["password"].(string), " ") {
			t.Error("password still contains a space")
		}
	})

	// mailtrap was the previous provider; nothing may build its config any more.
	t.Run("mailtrap config is gone", func(t *testing.T) {
		t.Setenv("MAIL_PROVIDER_TYPE", "mailtrap")

		cfg := NewConfig()
		if _, ok := cfg.MailProviderConfig["mailtrap"]; ok {
			t.Error("mailtrap config is still built")
		}
		if _, ok := cfg.MailProviderConfig["smtp"]; ok {
			t.Error("smtp config built for an unknown provider type")
		}
	})
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
