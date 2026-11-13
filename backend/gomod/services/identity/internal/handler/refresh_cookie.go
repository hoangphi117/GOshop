package handler

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"goshop/shared/pkg/auth"
)

// Clients sharing one origin must not share one refresh cookie. The storefront
// and the admin dashboard both run behind the same edge, so a single
// `refresh_token` cookie at Path=/ leaks the admin session into the storefront
// after any reload. Each client names itself with X-Client-App and gets its own
// cookie name instead.
const (
	clientHeader        = "X-Client-App"
	clientDashboard     = "dashboard"
	clientStorefront    = "storefront"
	refreshCookieBase   = "refresh_token"
	refreshCookiePrefix = "goshop_"
	// Narrow scope: refresh cookies only ride auth calls, never product,
	// order or catalog traffic. The per-client names keep the two apps apart;
	// Path only limits where the browser sends them.
	refreshCookiePath = "/api/v1/auth"
)

// refreshCookieName returns the per-client cookie name. Unknown or missing
// client headers keep the legacy shared name so older callers keep working.
func refreshCookieName(ctx *gin.Context) string {
	switch ctx.GetHeader(clientHeader) {
	case clientDashboard:
		return refreshCookiePrefix + refreshCookieBase + "_" + clientDashboard
	case clientStorefront:
		return refreshCookiePrefix + refreshCookieBase + "_" + clientStorefront
	default:
		return refreshCookieBase
	}
}

func cookieSecure() bool { return os.Getenv("APP_ENV") == "production" }

// setRefreshCookie writes the refresh token for the calling client.
func setRefreshCookie(ctx *gin.Context, token string) {
	setRefreshCookieNamed(ctx, refreshCookieName(ctx), token)
}

// setRefreshCookieNamed writes the refresh token into an explicit cookie name,
// used when rotating a token that arrived in a cookie.
func setRefreshCookieNamed(ctx *gin.Context, name, token string) {
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(name, token, int(auth.RefreshTokenTTL.Seconds()), refreshCookiePath, "", cookieSecure(), true)
}

// clearRefreshCookies expires this client's cookie plus every name in use, so a
// logout never leaves a live session behind under another client's name.
// Path=/ is included so cookies written by the previous broad scope die too.
func clearRefreshCookies(ctx *gin.Context) {
	ctx.SetSameSite(http.SameSiteLaxMode)
	names := []string{
		refreshCookieName(ctx),
		refreshCookieBase,
		refreshCookiePrefix + refreshCookieBase + "_" + clientDashboard,
		refreshCookiePrefix + refreshCookieBase + "_" + clientStorefront,
	}
	for _, name := range names {
		ctx.SetCookie(name, "", -1, refreshCookiePath, "", cookieSecure(), true)
		ctx.SetCookie(name, "", -1, "/", "", cookieSecure(), true)
	}
}

// refreshTokenFromCookie reads this client's cookie, falling back to the legacy
// shared name for sessions created before per-client cookies existed. It also
// looks up the other client's cookie by name: after the api-client middleware
// rotated a token with no X-Client-App header, the newest token lives under the
// other app's name. Returns the name it read, so the rotated token is written
// back into that same cookie instead of whichever name the header implies.
func refreshTokenFromCookie(ctx *gin.Context) (name, token string) {
	candidates := []string{
		refreshCookieName(ctx),
		refreshCookieBase,
		refreshCookiePrefix + refreshCookieBase + "_" + clientDashboard,
		refreshCookiePrefix + refreshCookieBase + "_" + clientStorefront,
	}
	for _, candidate := range candidates {
		if val, err := ctx.Cookie(candidate); err == nil && val != "" {
			return candidate, val
		}
	}
	return "", ""
}
