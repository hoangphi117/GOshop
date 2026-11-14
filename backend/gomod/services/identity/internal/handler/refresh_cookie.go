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

// clearRefreshCookies expires only the calling client's cookie, plus the legacy
// shared names (no live client writes those any more). Clearing both clients'
// cookies made signing out of the storefront silently sign the admin out of the
// dashboard — two sessions on one origin, two independent logouts.
// Path=/ is included so cookies written by the previous broad scope die too.
func clearRefreshCookies(ctx *gin.Context) {
	ctx.SetSameSite(http.SameSiteLaxMode)
	names := []string{
		refreshCookieName(ctx),
		refreshCookieBase,
	}
	for _, name := range names {
		ctx.SetCookie(name, "", -1, refreshCookiePath, "", cookieSecure(), true)
		ctx.SetCookie(name, "", -1, "/", "", cookieSecure(), true)
	}
}

// refreshTokenFromCookie reads this client's cookie, falling back to the legacy
// shared name for sessions created before per-client cookies existed. It never
// reads the other client's cookie: the two apps share an origin, so doing that
// silently traded an admin's session for the storefront's on any refresh — the
// storefront ended up as the admin, and the dashboard, whose token had just
// been rotated out from under it, got 403 on every admin route. Returns the
// name it read, so the rotated token is written back into that same cookie.
func refreshTokenFromCookie(ctx *gin.Context) (name, token string) {
	// The shared legacy name is only a bucket for pre-migration sessions; it is
	// never a stand-in for a different client's live cookie.
	candidates := []string{refreshCookieName(ctx), refreshCookieBase}
	for _, candidate := range candidates {
		if val, err := ctx.Cookie(candidate); err == nil && val != "" {
			return candidate, val
		}
	}
	return "", ""
}
