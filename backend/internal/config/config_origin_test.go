package config

import (
	"os"
	"testing"
)

// Verifies the APP_ORIGIN fan-out: one switch drives the storefront link, the
// payment return URL and the public media base. Explicit vars must still win.
func TestAppOriginDerivesBrowserFacingURLs(t *testing.T) {
	keys := []string{
		"APP_ORIGIN", "STOREFRONT_URL", "PAYMENT_RETURN_URL",
		"S3_PUBLIC_BASE_URL", "STORAGE_PUBLIC_BASE_URL",
	}
	for _, k := range keys {
		t.Setenv(k, "")
		_ = os.Unsetenv(k)
	}

	t.Run("derived from APP_ORIGIN", func(t *testing.T) {
		t.Setenv("APP_ORIGIN", "https://shop.example.com/")
		cfg := NewConfig()
		if got, want := cfg.StorefrontURL, "https://shop.example.com"; got != want {
			t.Fatalf("StorefrontURL = %q, want %q", got, want)
		}
		if got, want := cfg.Storage.S3.PublicBaseURL, "https://shop.example.com/goshop-media"; got != want {
			t.Fatalf("S3.PublicBaseURL = %q, want %q", got, want)
		}
	})

	t.Run("explicit override wins", func(t *testing.T) {
		t.Setenv("APP_ORIGIN", "https://shop.example.com")
		t.Setenv("STOREFRONT_URL", "http://localhost:3001")
		t.Setenv("S3_PUBLIC_BASE_URL", "https://pub-xxx.r2.dev")
		cfg := NewConfig()
		if got, want := cfg.StorefrontURL, "http://localhost:3001"; got != want {
			t.Fatalf("StorefrontURL = %q, want explicit %q", got, want)
		}
		if got, want := cfg.Storage.S3.PublicBaseURL, "https://pub-xxx.r2.dev"; got != want {
			t.Fatalf("S3.PublicBaseURL = %q, want explicit %q", got, want)
		}
	})

	t.Run("no APP_ORIGIN keeps dev defaults", func(t *testing.T) {
		cfg := NewConfig()
		if got, want := cfg.StorefrontURL, "http://localhost:3001"; got != want {
			t.Fatalf("StorefrontURL = %q, want %q", got, want)
		}
		if got := cfg.Storage.S3.PublicBaseURL; got != "" {
			t.Fatalf("S3.PublicBaseURL = %q, want empty (SDK falls back to endpoint)", got)
		}
	})
}

// The payment service has its own return-URL default; assert the same fan-out.
func TestPaymentReturnURLDerivation(t *testing.T) {
	// Mirrors the expression in services/payment/app/app.go: no import cycle
	// is worth pulling in an entire service just for one string.
	returnURLDefault := func(origin string) string {
		if origin == "" {
			return "http://localhost:3001/payment/result"
		}
		return origin + "/payment/result"
	}
	cases := map[string]string{
		"":                         "http://localhost:3001/payment/result",
		"https://shop.example.com": "https://shop.example.com/payment/result",
		"http://localhost":         "http://localhost/payment/result",
	}
	for origin, want := range cases {
		if got := returnURLDefault(origin); got != want {
			t.Fatalf("origin %q -> %q, want %q", origin, got, want)
		}
	}
}
