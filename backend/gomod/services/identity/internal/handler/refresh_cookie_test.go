package handler

// Guards the refresh cookie pairing. Logic lives inline in the handler, so a
// regression here (writing the rotated token under the header's client instead
// of the cookie's) would hand one app's session to the other and would only
// show up as "logged out every 15 minutes" in a browser.
//
// Run: go test ./identity/internal/handler/ -run RefreshCookie -v

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func cookieCtx(headers map[string]string, cookies ...*http.Cookie) *gin.Context {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	for k, v := range headers {
		ctx.Request.Header.Set(k, v)
	}
	for _, c := range cookies {
		ctx.Request.AddCookie(c)
	}
	return ctx
}

func TestRefreshCookieReadsPerClientName(t *testing.T) {
	ctx := cookieCtx(map[string]string{"X-Client-App": "storefront"},
		&http.Cookie{Name: "goshop_refresh_token_storefront", Value: "store-token"})

	name, token := refreshTokenFromCookie(ctx)
	if name != "goshop_refresh_token_storefront" || token != "store-token" {
		t.Fatalf("got (%q, %q)", name, token)
	}
}

func TestRefreshCookieFallsBackToLegacyName(t *testing.T) {
	ctx := cookieCtx(map[string]string{"X-Client-App": "dashboard"},
		&http.Cookie{Name: "refresh_token", Value: "legacy"})

	// The returned name must be the cookie actually read, not the header's
	// client name: the rotated token goes back into that same cookie.
	name, token := refreshTokenFromCookie(ctx)
	if name != "refresh_token" || token != "legacy" {
		t.Fatalf("got (%q, %q)", name, token)
	}
}

func TestRefreshCookieMissingHeaderKeepsLegacyJar(t *testing.T) {
	// No X-Client-App (exactly what the shared api-client middleware sends):
	// it must never be able to overwrite a per-client cookie.
	ctx := cookieCtx(nil,
		&http.Cookie{Name: "refresh_token", Value: "legacy"},
		&http.Cookie{Name: "goshop_refresh_token_dashboard", Value: "dash"})

	name, token := refreshTokenFromCookie(ctx)
	if name != "refresh_token" || token != "legacy" {
		t.Fatalf("got (%q, %q)", name, token)
	}
}

func TestRefreshCookieFallsBackToOtherClientName(t *testing.T) {
	// A headerless refresh has no legacy cookie, but the middleware rotated the
	// session into the other client's per-client cookie: the newest token is
	// there and must be picked up rather than reported as missing.
	ctx := cookieCtx(nil,
		&http.Cookie{Name: "goshop_refresh_token_dashboard", Value: "dash"})

	name, token := refreshTokenFromCookie(ctx)
	if name != "goshop_refresh_token_dashboard" || token != "dash" {
		t.Fatalf("got (%q, %q)", name, token)
	}
}

func TestSetRefreshCookieNamedStaysOnRefreshPath(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	ctx := cookieCtx(nil)

	setRefreshCookieNamed(ctx, "goshop_refresh_token_dashboard", "rotated")

	cookies := ctx.Writer.Header().Values("Set-Cookie")
	if len(cookies) != 1 {
		t.Fatalf("expected exactly one cookie, got %v", cookies)
	}
	want := "goshop_refresh_token_dashboard=rotated"
	if len(cookies[0]) < len(want) || cookies[0][:len(want)] != want {
		t.Fatalf("unexpected cookie %q", cookies[0])
	}
	if !contains(cookies[0], "Path=/api/v1/auth") || !contains(cookies[0], "HttpOnly") {
		t.Fatalf("cookie lost its scope/httponly flags: %q", cookies[0])
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
