# =============================================================================
# GOSHOP LOCATION ROUTING & REVERSE PROXY RULES
# =============================================================================

# 1. SECURITY HEADERS TOÀN CẦU
add_header X-Frame-Options "SAMEORIGIN" always;
add_header X-Content-Type-Options "nosniff" always;
add_header X-XSS-Protection "1; mode=block" always;
add_header Referrer-Policy "strict-origin-when-cross-origin" always;
# Echo nginx's per-request trace id back to the caller for support/debugging.
add_header X-Request-ID $request_id always;

# Inheritable default: blocked requests get 429 (Too Many Requests), not 503.
limit_req_status 429;

# Giới hạn kích thước payload gửi lên (File upload, avatar, product images)
client_max_body_size 50M;

# -----------------------------------------------------------------------------
# 1.1. HEALTH PROBES
# -----------------------------------------------------------------------------
# /healthz      edge-only, instant. Use for LB / uptime checks.
# /health/deep  probes the storefront-critical upstream (catalog) via
#               auth_request: 2xx => ok JSON; anything else => 503 naming the
#               dependency. ponytail: nginx OSS cannot AND multiple probes in
#               one request (auth_request is once per location). When more
#               dependencies must gate this, upgrade to an njs js_content
#               handler (nginx-module-njs, ~15 lines) instead of growing this.
# The probe target is a variable upstream so nginx startup does not hard-fail
# when the container is down; DNS re-resolves per request (resolver in goshop.conf).
location = /healthz {
    access_log off;
    default_type application/json;
    return 200 '{"edge":"nginx","status":"ok"}';
}

location = /health/deep {
    access_log off;
    default_type application/json;
    # auth_request runs in the access phase; a `return` here would short-circuit
    # in the rewrite phase and the probe would never fire. The 200 body is
    # therefore served by proxying to the edge /healthz location (port 80).
    auth_request /_hp_catalog;
    error_page 500 502 503 = @health_degraded;
    proxy_pass http://127.0.0.1:80/healthz;
    add_header X-Deep-Probe "catalog" always;
}

location @health_degraded {
    default_type application/json;
    return 503 '{"status":"degraded","down":["catalog"]}';
}

location = /_hp_catalog {
    internal;
    access_log off;
    proxy_connect_timeout 1s;
    proxy_read_timeout 1s;
    set $hp_catalog_target http://catalog-service:8086;
    proxy_pass $hp_catalog_target/health;
}

# -----------------------------------------------------------------------------
# 2. MEDIA & UPLOADS ROUTING (/uploads/)
# -----------------------------------------------------------------------------
# Lợi ích: Stateless storage routing. Chuyển tiếp tới backend để redirect 301 sang
# Cloudflare R2 / S3 / MinIO CDN URL, loại bỏ hoàn toàn phụ thuộc vào local disk volume.
location /uploads/ {
    limit_req zone=api burst=100 nodelay;
    proxy_pass http://goshop_backend;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
    # The key never ships to a browser: the edge injects it server-side.
    proxy_set_header X-API-KEY $goshop_api_key;
    proxy_read_timeout 60s;
}

# -----------------------------------------------------------------------------
# 2b. OBJECT STORAGE (MinIO) — public reads via edge, no published port
# -----------------------------------------------------------------------------
# S3_PUBLIC_BASE_URL points at this path (see backend/.env), so DB rows and
# API responses carry edge-relative URLs and no host is baked into storage
# config. Reached at http://<edge>/<bucket>/<key> — the URL path IS the MinIO
# path, no rewrite. SigV4 presigned uploads sign the exact URI and Host, so a
# rewrite or Host change would invalidate the signature.
location ^~ /goshop-media/ {
    # Dynamic variable: nginx keeps booting even if minio is not up yet
    set $minio_target http://minio:9000;
    # No URI on proxy_pass: original path+query are forwarded verbatim.
    proxy_pass $minio_target;
    # Presigned URLs are signed for the PUBLIC host (S3_PUBLIC_BASE_URL), and
    # the SigV4 signature covers the Host header — it must reach MinIO intact.
    proxy_set_header Host $http_host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_http_version 1.1;
    proxy_read_timeout 60s;
    proxy_buffering off;
    # Never echo Expires on uploads. Immutable cache only for reads.
    set $minio_cache "";
    if ($request_method = GET) { set $minio_cache "public, max-age=604800, immutable"; }
    if ($request_method = HEAD) { set $minio_cache "public, max-age=604800, immutable"; }
    add_header Cache-Control $minio_cache;
}

# -----------------------------------------------------------------------------
# 3. WEBSOCKET REVERSE PROXY (/api/v1/ws) - PHẢI ĐỨNG TRƯỚC /api/v1/
# -----------------------------------------------------------------------------
# Lợi ích: Hỗ trợ kết nối thời gian thực hai chiều (Chat, Notifications, Redis Bridge).
# Điều hướng trực tiếp tới ws-gateway chuyên biệt giúp giảm tải trên REST API servers.
# Timeout mở rộng 3600s để tránh ngắt kết nối WebSocket giữa chừng.
location /api/v1/ws {
    # Dùng biến động để Nginx không bị crash nếu container ws-gateway chưa khởi động
    set $ws_target http://ws-gateway:8085;
    proxy_pass $ws_target;
    proxy_http_version 1.1;

    # Header bắt buộc cho cơ chế HTTP Upgrade -> WebSocket
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";

    # Chuyển tiếp thông tin client thật tới Gin Engine (để ClientIP() hoạt động)
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
    # The key never ships to a browser: the edge injects it server-side.
    proxy_set_header X-API-KEY $goshop_api_key;

    # Tối ưu timeout cho kết nối thời gian thực lâu dài
    proxy_read_timeout 3600s;
    proxy_send_timeout 3600s;
    proxy_connect_timeout 10s;
    proxy_buffering off; # Vô hiệu hóa buffering để event truyền ngay lập tức
}

# -----------------------------------------------------------------------------
# 4. IDENTITY SERVICE REVERSE PROXY (/api/v1/auth, /api/v1/users, /api/v1/addresses)
# -----------------------------------------------------------------------------
# Điều hướng các API định danh sang identity-service:8081
location /api/v1/auth {
    limit_req zone=auth burst=10 nodelay;
    set $identity_target http://identity-service:8081;
    proxy_pass $identity_target;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
    # The key never ships to a browser: the edge injects it server-side.
    proxy_set_header X-API-KEY $goshop_api_key;
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Forwarded-Port $server_port;
    proxy_connect_timeout 10s;
    proxy_read_timeout 60s;
    proxy_send_timeout 60s;
    proxy_request_buffering off;
}

location /api/v1/users {
    set $identity_target http://identity-service:8081;
    proxy_pass $identity_target;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
    # The key never ships to a browser: the edge injects it server-side.
    proxy_set_header X-API-KEY $goshop_api_key;
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Forwarded-Port $server_port;
    proxy_connect_timeout 10s;
    proxy_read_timeout 60s;
    proxy_send_timeout 60s;
    proxy_request_buffering off;
    limit_req zone=api burst=100 nodelay;
}

location /api/v1/admin/users {
    set $identity_target http://identity-service:8081;
    proxy_pass $identity_target;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
    # The key never ships to a browser: the edge injects it server-side.
    proxy_set_header X-API-KEY $goshop_api_key;
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Forwarded-Port $server_port;
    proxy_connect_timeout 10s;
    proxy_read_timeout 60s;
    proxy_send_timeout 60s;
    proxy_request_buffering off;
    limit_req zone=api burst=100 nodelay;
}

location /api/v1/addresses {
    set $identity_target http://identity-service:8081;
    proxy_pass $identity_target;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
    # The key never ships to a browser: the edge injects it server-side.
    proxy_set_header X-API-KEY $goshop_api_key;
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Forwarded-Port $server_port;
    proxy_connect_timeout 10s;
    proxy_read_timeout 60s;
    proxy_send_timeout 60s;
    proxy_request_buffering off;
    limit_req zone=api burst=100 nodelay;
}

# -----------------------------------------------------------------------------
# 4.2. COUPON SERVICE REVERSE PROXY (/api/v1/coupons, /api/v1/admin/coupons)
# -----------------------------------------------------------------------------
location /api/v1/coupons {
    set $coupon_target http://coupon-service:8084;
    proxy_pass $coupon_target;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
    # The key never ships to a browser: the edge injects it server-side.
    proxy_set_header X-API-KEY $goshop_api_key;
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Forwarded-Port $server_port;
    proxy_connect_timeout 10s;
    proxy_read_timeout 60s;
    proxy_send_timeout 60s;
    proxy_request_buffering off;
    limit_req zone=api burst=100 nodelay;
}

location /api/v1/admin/coupons {
    set $coupon_target http://coupon-service:8084;
    proxy_pass $coupon_target;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
    # The key never ships to a browser: the edge injects it server-side.
    proxy_set_header X-API-KEY $goshop_api_key;
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Forwarded-Port $server_port;
    proxy_connect_timeout 10s;
    proxy_read_timeout 60s;
    proxy_send_timeout 60s;
    proxy_request_buffering off;
    limit_req zone=api burst=100 nodelay;
}

# -----------------------------------------------------------------------------

# -----------------------------------------------------------------------------
# 4.2.5. BANNER SERVICE REVERSE PROXY (/api/v1/banners, /api/v1/admin/banners)
# -----------------------------------------------------------------------------
location /api/v1/banners {
    set $banner_target http://banner-service:8089;
    proxy_pass $banner_target;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
    # The key never ships to a browser: the edge injects it server-side.
    proxy_set_header X-API-KEY $goshop_api_key;
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Forwarded-Port $server_port;
    proxy_connect_timeout 10s;
    proxy_read_timeout 60s;
    proxy_send_timeout 60s;
    proxy_request_buffering off;
    limit_req zone=api burst=100 nodelay;
}

location /api/v1/admin/banners {
    set $banner_target http://banner-service:8089;
    proxy_pass $banner_target;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
    # The key never ships to a browser: the edge injects it server-side.
    proxy_set_header X-API-KEY $goshop_api_key;
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Forwarded-Port $server_port;
    proxy_connect_timeout 10s;
    proxy_read_timeout 60s;
    proxy_send_timeout 60s;
    proxy_request_buffering off;
    limit_req zone=api burst=100 nodelay;
}
# 4.3. INVENTORY SERVICE REVERSE PROXY (/api/v1/admin/inventory, /api/v1/inventory)
# -----------------------------------------------------------------------------
location /api/v1/admin/inventory {
    set $inventory_target http://inventory-service:8090;
    proxy_pass $inventory_target;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
    # The key never ships to a browser: the edge injects it server-side.
    proxy_set_header X-API-KEY $goshop_api_key;
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Forwarded-Port $server_port;
    proxy_connect_timeout 10s;
    proxy_read_timeout 60s;
    proxy_send_timeout 60s;
    proxy_request_buffering off;
    limit_req zone=api burst=100 nodelay;
}

location /api/v1/inventory {
    set $inventory_target http://inventory-service:8090;
    proxy_pass $inventory_target;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
    # The key never ships to a browser: the edge injects it server-side.
    proxy_set_header X-API-KEY $goshop_api_key;
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Forwarded-Port $server_port;
    proxy_connect_timeout 10s;
    proxy_read_timeout 60s;
    proxy_send_timeout 60s;
    proxy_request_buffering off;
    limit_req zone=api burst=100 nodelay;
}

# -----------------------------------------------------------------------------
# 4.4. CATALOG SERVICE REVERSE PROXY (/api/v1/products, /api/v1/categories)
# -----------------------------------------------------------------------------
# Review product is served by the dedicated review service via the API
# gateway's REVIEW_SERVICE_URL proxy.
location ~ ^/api/v1/products/[^/]+/reviews {
    proxy_pass http://goshop_backend;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
    # The key never ships to a browser: the edge injects it server-side.
    proxy_set_header X-API-KEY $goshop_api_key;
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Forwarded-Port $server_port;
    proxy_connect_timeout 10s;
    proxy_read_timeout 60s;
    proxy_send_timeout 60s;
    proxy_request_buffering off;
    limit_req zone=api burst=100 nodelay;
}

location /api/v1/products {
    set $catalog_target http://catalog-service:8086;
    proxy_pass $catalog_target;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
    # The key never ships to a browser: the edge injects it server-side.
    proxy_set_header X-API-KEY $goshop_api_key;
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Forwarded-Port $server_port;
    proxy_connect_timeout 10s;
    proxy_read_timeout 60s;
    proxy_send_timeout 60s;
    proxy_request_buffering off;
    limit_req zone=api burst=100 nodelay;
}

location /api/v1/admin/products {
    set $catalog_target http://catalog-service:8086;
    proxy_pass $catalog_target;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
    # The key never ships to a browser: the edge injects it server-side.
    proxy_set_header X-API-KEY $goshop_api_key;
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Forwarded-Port $server_port;
    proxy_connect_timeout 10s;
    proxy_read_timeout 60s;
    proxy_send_timeout 60s;
    proxy_request_buffering off;
    limit_req zone=api burst=100 nodelay;
}

location /api/v1/categories {
    set $catalog_target http://catalog-service:8086;
    proxy_pass $catalog_target;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
    # The key never ships to a browser: the edge injects it server-side.
    proxy_set_header X-API-KEY $goshop_api_key;
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Forwarded-Port $server_port;
    proxy_connect_timeout 10s;
    proxy_read_timeout 60s;
    proxy_send_timeout 60s;
    proxy_request_buffering off;
    limit_req zone=api burst=100 nodelay;
}

location /api/v1/admin/categories {
    set $catalog_target http://catalog-service:8086;
    proxy_pass $catalog_target;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
    # The key never ships to a browser: the edge injects it server-side.
    proxy_set_header X-API-KEY $goshop_api_key;
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Forwarded-Port $server_port;
    proxy_connect_timeout 10s;
    proxy_read_timeout 60s;
    proxy_send_timeout 60s;
    proxy_request_buffering off;
    limit_req zone=api burst=100 nodelay;
}

# -----------------------------------------------------------------------------
# 4.5. CART SERVICE REVERSE PROXY (/api/v1/cart)
# -----------------------------------------------------------------------------
location /api/v1/cart {
    set $cart_target http://cart-service:8087;
    proxy_pass $cart_target;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
    # The key never ships to a browser: the edge injects it server-side.
    proxy_set_header X-API-KEY $goshop_api_key;
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Forwarded-Port $server_port;
    proxy_connect_timeout 10s;
    proxy_read_timeout 60s;
    proxy_send_timeout 60s;
    proxy_request_buffering off;
    limit_req zone=api burst=100 nodelay;
}

# -----------------------------------------------------------------------------
# 4.6. ORDER SERVICE REVERSE PROXY (/api/v1/orders, /api/v1/admin/orders)
# -----------------------------------------------------------------------------
location /api/v1/orders {
    set $order_target http://order-service:8088;
    proxy_pass $order_target;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
    # The key never ships to a browser: the edge injects it server-side.
    proxy_set_header X-API-KEY $goshop_api_key;
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Forwarded-Port $server_port;
    proxy_connect_timeout 10s;
    proxy_read_timeout 60s;
    proxy_send_timeout 60s;
    proxy_request_buffering off;
    limit_req zone=api burst=100 nodelay;
}

location /api/v1/admin/orders {
    set $order_target http://order-service:8088;
    proxy_pass $order_target;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
    # The key never ships to a browser: the edge injects it server-side.
    proxy_set_header X-API-KEY $goshop_api_key;
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Forwarded-Port $server_port;
    proxy_connect_timeout 10s;
    proxy_read_timeout 60s;
    proxy_send_timeout 60s;
    proxy_request_buffering off;
    limit_req zone=api burst=100 nodelay;
}

location /api/v1/admin/returns {
    set $order_target http://order-service:8088;
    proxy_pass $order_target;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
    # The key never ships to a browser: the edge injects it server-side.
    proxy_set_header X-API-KEY $goshop_api_key;
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Forwarded-Port $server_port;
    proxy_connect_timeout 10s;
    proxy_read_timeout 60s;
    proxy_send_timeout 60s;
    proxy_request_buffering off;
    limit_req zone=api burst=100 nodelay;
}

location /api/v1/shipping {
    set $order_target http://order-service:8088;
    proxy_pass $order_target;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
    # The key never ships to a browser: the edge injects it server-side.
    proxy_set_header X-API-KEY $goshop_api_key;
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Forwarded-Port $server_port;
    proxy_connect_timeout 10s;
    proxy_read_timeout 60s;
    proxy_send_timeout 60s;
    proxy_request_buffering off;
    limit_req zone=api burst=100 nodelay;
}

# -----------------------------------------------------------------------------
# 4.7. BLOCK EXTERNAL ACCESS TO INTERNAL MICROSERVICE ENDPOINTS (/internal/)
# -----------------------------------------------------------------------------
# Các endpoint /internal/* chỉ dành riêng cho giao tiếp IPC nội bộ giữa các services
location /internal/ {
    return 403;
}

# -----------------------------------------------------------------------------
# 5. API GATEWAY REVERSE PROXY & LOAD BALANCER (/api/v1/)
# -----------------------------------------------------------------------------
# Remaining API paths go to the API gateway, which fans out to the owning
# services (catalog, cart, order, review, analytics, media, chat...).
location /api/v1/ {
    proxy_pass http://goshop_backend;
    proxy_http_version 1.1;

    # Xóa connection close để tận dụng keepalive pool
    proxy_set_header Connection "";

    # Chuyển giao các headers nhận diện client cho backend Gin
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
    # The key never ships to a browser: the edge injects it server-side.
    proxy_set_header X-API-KEY $goshop_api_key;
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Forwarded-Port $server_port;

    # Cấu hình timeouts cho REST API
    proxy_connect_timeout 10s;
    proxy_read_timeout 60s;
    proxy_send_timeout 60s;

    # Không buffer request lớn vào disk để tăng tốc độ upload
    proxy_request_buffering off;
    limit_req zone=api burst=100 nodelay;
}

# -----------------------------------------------------------------------------
# 5. SWAGGER DOCUMENTATION PROXY (/swagger/)
# -----------------------------------------------------------------------------
location /swagger/ {
    proxy_pass http://goshop_backend;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
    # The key never ships to a browser: the edge injects it server-side.
    proxy_set_header X-API-KEY $goshop_api_key;
}

# -----------------------------------------------------------------------------
# 6. OPENID / JWKS WELL-KNOWN PROXY (/.well-known/)
# -----------------------------------------------------------------------------
# Cung cấp public key JWKS để microservices ngoại vi (ws-gateway, worker, gateway)
# tự verify JWT signature offline mà không cần gọi network tới Auth service.
location /.well-known/ {
    proxy_pass http://goshop_backend;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
    # The key never ships to a browser: the edge injects it server-side.
    proxy_set_header X-API-KEY $goshop_api_key;
}

# -----------------------------------------------------------------------------
# 7. DIRECT MEILISEARCH SEARCH PROXY (/search-api/)
# -----------------------------------------------------------------------------
# Chuyển tiếp trực tiếp truy vấn tìm kiếm sang Meilisearch cluster,
# giải phóng 100% tải đọc tìm kiếm ra khỏi PostgreSQL Master và Backend API.
location /search-api/ {
    limit_req zone=api burst=100 nodelay;
    # Dùng biến động để Nginx không bị crash nếu container meilisearch chưa khởi động
    set $meili_target http://meilisearch:7700;
    rewrite ^/search-api/(.*)$ /$1 break;
    proxy_pass $meili_target;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
    # The key never ships to a browser: the edge injects it server-side.
    proxy_set_header X-API-KEY $goshop_api_key;

    # Bảo vệ an toàn: Chỉ cho phép GET, POST, OPTIONS phục vụ tìm kiếm tài liệu
    limit_except GET POST OPTIONS {
        deny all;
    }
}

# -----------------------------------------------------------------------------
# 7.5 ADMIN DASHBOARD SPA (/admin/) — static Vite build mounted straight into
#     the edge nginx (no dashboard container, no extra port-forward hop).
#     Build with:  pnpm --filter dashboard build   (host or CI)
#     Output:      frontend/apps/dashboard/dist  → mounted at /srv/dashboard
#     SPA fallback: any unknown path under /admin/ serves index.html so the
#     client-side router (TanStack) owns the route table.
# -----------------------------------------------------------------------------
location = /admin {
    return 301 /admin/;
}

location ^~ /admin/ {
    alias /srv/dashboard/;
    index index.html;
    try_files $uri $uri/ /admin/index.html;
}

# -----------------------------------------------------------------------------
# 8. FRONTEND NEXT.JS STOREFRONT ROUTING (/)
# -----------------------------------------------------------------------------
location / {
    # Dùng biến động để Nginx không bị crash nếu container storefront chưa khởi động
    set $storefront_target http://storefront:3000;
    proxy_pass $storefront_target;
    proxy_http_version 1.1;

    # Hỗ trợ Next.js HMR và WebSocket nội bộ
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";

    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
    # The key never ships to a browser: the edge injects it server-side.
    proxy_set_header X-API-KEY $goshop_api_key;

    proxy_read_timeout 60s;
    proxy_connect_timeout 10s;
}
