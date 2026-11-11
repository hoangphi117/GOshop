package middleware

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AnonCookieName is the guest identity carried on personalization surfaces.
// Kept in sync with the storefront constant of the same name.
const AnonCookieName = "goshop_anon_id"

// anonCookieMaxAge is one year. The identity is low value and rebuildable, so a
// long life avoids re-minting on every visit.
const anonCookieMaxAge = 365 * 24 * 3600

// AnonCookie mints a stable guest identity when the cookie is missing or
// malformed and exposes it as "anon_id" in the gin context. It never aborts:
// personalization degrades to best sellers rather than failing the request.
//
// HttpOnly keeps scripts from forging an identity; the browser still sends it
// automatically on same-site requests. Secure is driven by ANON_COOKIE_SECURE
// (default true) so a plain-HTTP dev stack can still set the cookie.
func AnonCookie() gin.HandlerFunc {
	secure := os.Getenv("ANON_COOKIE_SECURE") != "false"

	return func(ctx *gin.Context) {
		if v, err := ctx.Cookie(AnonCookieName); err == nil {
			if u, perr := uuid.Parse(v); perr == nil {
				ctx.Set("anon_id", u.String())
				ctx.Next()
				return
			}
		}

		id := uuid.NewString()
		http.SetCookie(ctx.Writer, &http.Cookie{
			Name:     AnonCookieName,
			Value:    id,
			Path:     "/",
			MaxAge:   anonCookieMaxAge,
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
		})
		ctx.Set("anon_id", id)
		ctx.Next()
	}
}

// GetAnonID returns the guest identity set by AnonCookie.
func GetAnonID(ctx *gin.Context) string {
	val, exists := ctx.Get("anon_id")
	if !exists || val == nil {
		return ""
	}
	id, ok := val.(string)
	if !ok {
		return ""
	}
	return id
}
