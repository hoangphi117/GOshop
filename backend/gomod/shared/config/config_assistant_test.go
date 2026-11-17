package config

import "testing"

// The assistant feature must never take a service down: an empty key disables
// it, explicit ASSISTANT_ENABLED wins, and invalid numbers fall back instead of
// producing a zero timeout.
func TestAssistantConfigDefaults(t *testing.T) {
	t.Run("no key disables the feature", func(t *testing.T) {
		for _, k := range []string{"ASSISTANT_ENABLED", "ASSISTANT_API_KEY", "ASSISTANT_MODEL"} {
			t.Setenv(k, "")
		}
		cfg := NewConfig()
		if cfg.Assistant.Enabled {
			t.Fatal("Enabled = true with empty API key, want false")
		}
		if cfg.Assistant.TimeoutSec != 120 || cfg.Assistant.MaxTokens != 1024 {
			t.Fatalf("defaults not applied: timeout=%d max=%d", cfg.Assistant.TimeoutSec, cfg.Assistant.MaxTokens)
		}
	})

	t.Run("key and model enable it", func(t *testing.T) {
		t.Setenv("ASSISTANT_ENABLED", "")
		t.Setenv("ASSISTANT_API_KEY", "rv_test")
		t.Setenv("ASSISTANT_MODEL", "deepseek-ai/DeepSeek-V4.1-Flash-1017")
		cfg := NewConfig()
		if !cfg.Assistant.Enabled {
			t.Fatal("Enabled = false with key and model set, want true")
		}
		if cfg.Assistant.SessionTurns != 6 || cfg.Assistant.SessionTTL != 3600 {
			t.Fatalf("session defaults wrong: turns=%d ttl=%d", cfg.Assistant.SessionTurns, cfg.Assistant.SessionTTL)
		}
	})

	t.Run("explicit flag wins over key presence", func(t *testing.T) {
		t.Setenv("ASSISTANT_ENABLED", "false")
		t.Setenv("ASSISTANT_API_KEY", "rv_test")
		if NewConfig().Assistant.Enabled {
			t.Fatal("Enabled = true despite ASSISTANT_ENABLED=false")
		}
	})

	t.Run("garbage numbers fall back", func(t *testing.T) {
		t.Setenv("ASSISTANT_TIMEOUT_SEC", "abc")
		t.Setenv("ASSISTANT_MAX_TOKENS", "-5")
		t.Setenv("ASSISTANT_TEMPERATURE", "nope")
		cfg := NewConfig()
		if cfg.Assistant.TimeoutSec != 120 {
			t.Fatalf("TimeoutSec = %d, want fallback 120", cfg.Assistant.TimeoutSec)
		}
		if cfg.Assistant.MaxTokens != 1024 {
			t.Fatalf("MaxTokens = %d, want fallback 1024", cfg.Assistant.MaxTokens)
		}
		if cfg.Assistant.Temperature != 0.3 {
			t.Fatalf("Temperature = %v, want fallback 0.3", cfg.Assistant.Temperature)
		}
	})
}
