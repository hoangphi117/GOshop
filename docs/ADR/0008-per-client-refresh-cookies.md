# ADR 0008 — Two frontends share one origin, so refresh tokens are scoped per client

- **Status:** Accepted
- **Date:** 2026-11
- **Scope:** storefront, dashboard, `@goshop/api-client`, identity, nginx
- **Code:** `frontend/packages/api-client`, `backend/gomod/services/identity`

## Context

The storefront (Next.js) and the admin dashboard (React SPA) are served from the same origin
by the edge — the dashboard at `/admin/`. Cookies are scoped by origin and path, not by app.
A single `refresh_token` cookie would therefore be shared: signing into the dashboard would
overwrite the storefront's session, and a logout in one would sign the other out.

The dashboard cannot keep a refresh token in `localStorage` either: an XSS in admin markup
would then let an attacker mint access tokens indefinitely.

## Decision

**Access tokens live in memory only.** A page reload restores the session by refreshing.

**Refresh cookies are per client.** Both apps send `X-Client-App` (`storefront` |
`dashboard`); identity issues the refresh token into a cookie named per client and, on
rotation, writes the new token back into the exact cookie it was read from.

**The dashboard wraps the generated client in a Proxy** that refreshes once on `401` and
retries the original request. The generated client's own blind retry is not used: retrying
without first refreshing just burns the rate limit.

**`API_KEY` stays server-side.** Edge nginx injects `X-API-KEY` on `/api/v1/*` at request time;
`NEXT_PUBLIC_API_KEY` / `VITE_API_KEY` stay empty in production and frontend CI asserts no key
reaches a bundle.

## Consequences

**Positive**

- Both apps can be signed in simultaneously, and a logout affects only its own client.
- An XSS in either frontend cannot exfiltrate a refresh token from `localStorage`.
- Rotated refresh tokens cannot land in the wrong cookie, which is the classic cause of
  "logged out at random".

**Trade-offs**

- A reload costs one extra round trip (refresh) before the first API call.
- Two cookie names to reason about when debugging auth.
- Any new frontend must pick a client name and be added to identity's allow-list.

## Rejected alternatives

- **Refresh token in `localStorage`** — XSS becomes permanent account takeover.
- **One shared cookie between apps** — the two sessions clobber each other, as above.
- **Separate origins (dashboard on its own domain)** — cleanest in theory, but then the API
  key and CORS have to be handled twice, and the dashboard loses the free same-origin proxy.
- **Edge-level silent refresh** — moves token logic into nginx, where it is untested and
  invisible to both apps.
