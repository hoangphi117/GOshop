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
    proxy_read_timeout 60s;
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
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Forwarded-Port $server_port;
    proxy_connect_timeout 10s;
    proxy_read_timeout 60s;
    proxy_send_timeout 60s;
    proxy_request_buffering off;
    limit_req zone=api burst=100 nodelay;
}

# -----------------------------------------------------------------------------
# 4.3. INVENTORY SERVICE REVERSE PROXY (/api/v1/admin/inventory, /api/v1/inventory)
# -----------------------------------------------------------------------------
location /api/v1/admin/inventory {
    set $inventory_target http://inventory-service:8085;
    proxy_pass $inventory_target;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Forwarded-Port $server_port;
    proxy_connect_timeout 10s;
    proxy_read_timeout 60s;
    proxy_send_timeout 60s;
    proxy_request_buffering off;
    limit_req zone=api burst=100 nodelay;
}

location /api/v1/inventory {
    set $inventory_target http://inventory-service:8085;
    proxy_pass $inventory_target;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
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
# Product Reviews are served by Monolith Core Backend
location ~ ^/api/v1/products/[^/]+/reviews {
    set $core_backend_target http://core-backend:8080;
    proxy_pass $core_backend_target;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
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
# 5. CORE BACKEND REVERSE PROXY & LOAD BALANCER (/api/v1/)
# -----------------------------------------------------------------------------
# Điều hướng tất cả API còn lại tới core-backend:8080
location /api/v1/ {
    set $core_backend_target http://core-backend:8080;
    proxy_pass $core_backend_target;
    proxy_http_version 1.1;

    # Xóa connection close để tận dụng keepalive pool
    proxy_set_header Connection "";

    # Chuyển giao các headers nhận diện client cho backend Gin
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Request-ID $request_id;
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

    # Bảo vệ an toàn: Chỉ cho phép GET, POST, OPTIONS phục vụ tìm kiếm tài liệu
    limit_except GET POST OPTIONS {
        deny all;
    }
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

    proxy_read_timeout 60s;
    proxy_connect_timeout 10s;
}