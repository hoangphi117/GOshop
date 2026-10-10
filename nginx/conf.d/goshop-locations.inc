# =============================================================================
# GOSHOP LOCATION ROUTING & REVERSE PROXY RULES
# =============================================================================

# 1. SECURITY HEADERS TOÀN CỤC
add_header X-Frame-Options "SAMEORIGIN" always;
add_header X-Content-Type-Options "nosniff" always;
add_header X-XSS-Protection "1; mode=block" always;
add_header Referrer-Policy "strict-origin-when-cross-origin" always;

# Giới hạn kích thước payload gửi lên (File upload, avatar, product images)
client_max_body_size 50M;

# -----------------------------------------------------------------------------
# -----------------------------------------------------------------------------
# 2. MEDIA & UPLOADS ROUTING (/uploads/)
# -----------------------------------------------------------------------------
# Lợi ích: Stateless storage routing. Chuyển tiếp tới backend để redirect 301 sang
# Cloudflare R2 / S3 / MinIO CDN URL, loại bỏ hoàn toàn phụ thuộc vào local disk volume.
location /uploads/ {
    proxy_pass http://goshop_backend;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_read_timeout 60s;
}

# -----------------------------------------------------------------------------
# 3. WEBSOCKET REVERSE PROXY (/api/v1/ws) - PHẢI ĐỨNG TRƯỚC /api/v1/
# -----------------------------------------------------------------------------
# Lợi ích: Hỗ trợ kết nối thời gian thực hai chiều (Chat, Notifications, Redis Bridge).
# Điều hướng trực tiếp tới ws-gateway chuyên biệt giúp giảm tải trên REST API servers.
# Timeout mở rộng 3600s để tránh ngắt kết nối WebSocket giữa chừng.
location /api/v1/ws {
    proxy_pass http://goshop_ws_gateway;
    proxy_http_version 1.1;

    # Header bắt buộc cho cơ chế HTTP Upgrade -> WebSocket
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";

    # Chuyển tiếp thông tin client thật tới Gin Engine (để ClientIP() hoạt động)
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;

    # Tối ưu timeout cho kết nối thời gian thực lâu dài
    proxy_read_timeout 3600s;
    proxy_send_timeout 3600s;
    proxy_connect_timeout 10s;
    proxy_buffering off; # Vô hiệu hóa buffering để event truyền ngay lập tức
}

# -----------------------------------------------------------------------------
# 4. REST API REVERSE PROXY & LOAD BALANCER (/api/v1/)
# -----------------------------------------------------------------------------
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
    proxy_set_header X-Forwarded-Host $host;
    proxy_set_header X-Forwarded-Port $server_port;

    # Cấu hình timeouts cho REST API
    proxy_connect_timeout 10s;
    proxy_read_timeout 60s;
    proxy_send_timeout 60s;

    # Không buffer request lớn vào disk để tăng tốc độ upload
    proxy_request_buffering off;
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
}

# -----------------------------------------------------------------------------
# 6. FRONTEND NEXT.JS STOREFRONT ROUTING (/)
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

    proxy_read_timeout 60s;
    proxy_connect_timeout 10s;
}
