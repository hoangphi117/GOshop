# Shared CORS headers for every /api/v1/* location.
#
# Why the edge and not the gateway: cmd/api sets Access-Control-Allow-Origin in
# middleware.CORSMiddleware, but its httputil.ReverseProxy handlers write the
# upstream header block straight to the client connection and overwrite what
# the middleware had set. The gateway therefore answers a cross-origin request
# with no CORS header at all, which a browser reports as a CORS failure.
#
# `add_header` is NOT inherited once a location declares any of its own, so this
# file must be included explicitly in each /api/v1/* location.
#
# `always` covers 4xx/5xx too; without it a browser reports an opaque CORS error
# instead of the real status code.
#
# $http_origin is echoed back rather than hardcoded: dev serves the storefront
# from :3001 over plain HTTP and prod from the real domain over HTTPS, and
# reflecting the caller's origin keeps both working. The api gateway still does
# the real allow-list check; this only restores the header it was dropping.
#
# proxy_hide_header is mandatory, not defensive: services that DO emit CORS
# (identity, payment, notification — anything behind the gateway's middleware)
# would otherwise reach the browser as
#   Access-Control-Allow-Origin: http://localhost:3001, http://localhost:3001
# which Chrome rejects outright. Hiding the upstream copy makes nginx the single
# source of truth for these headers on every API path.

proxy_hide_header Access-Control-Allow-Origin;
proxy_hide_header Access-Control-Allow-Credentials;
proxy_hide_header Access-Control-Allow-Methods;
proxy_hide_header Access-Control-Allow-Headers;
proxy_hide_header Access-Control-Expose-Headers;
proxy_hide_header Access-Control-Max-Age;

add_header Access-Control-Allow-Origin $http_origin always;
add_header Access-Control-Allow-Credentials "true" always;
add_header Access-Control-Allow-Methods "GET, POST, PUT, PATCH, DELETE, OPTIONS" always;
add_header Access-Control-Allow-Headers "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, Cache-Control, X-Requested-With, X-API-KEY, X-Idempotency-Key, X-Trace-Id, X-Client-App" always;
add_header Access-Control-Expose-Headers "X-Request-ID" always;
add_header Access-Control-Max-Age "86400" always;
add_header Vary "Origin" always;

# Preflight must be answered here, not proxied.
#
# Several upstreams have no OPTIONS handler (catalog, coupon, inventory, and the
# /products, /categories, /coupons, /inventory locations in general), so a
# forwarded preflight comes back 404. The headers above are still attached
# (`always`), but a browser rejects a preflight whose status is not ok — the
# reported symptom is "Response to preflight request doesn't pass access control
# check: It does not have HTTP ok status", with the real request never sent.
#
# Returning 204 here short-circuits the proxy: no upstream call, no 404, and the
# browser sees a valid preflight on every /api/v1/* path.
if ($request_method = OPTIONS) {
    return 204;
}
