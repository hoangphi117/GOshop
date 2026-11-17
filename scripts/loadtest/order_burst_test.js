import http from "k6/http";
import { check, fail } from "k6";

// ============================================================================
// ORDER BURST - bắn tạo đơn hàng loạt để xem Kafka consumer lag trên kafka-ui.
//
// Chạy:
//   k6 run \
//     -e BASE_URL=http://localhost \
//     -e API_KEY=xxxx \
//     -e TOKEN=eyJhbGciOi... \
//     -e PRODUCT_UUID=<uuid-san-pham-con-hang> \
//     -e QTY=1 \
//     scripts/loadtest/order_burst_test.js
//
// Mặc định: 20 đơn trong ~30s (rate 10/s giữ 2s... chỉnh ORDER_RATE/DURATION).
// payment_method=1 (COD) để không cần VNPay. Mỗi VU dùng idempotency key riêng.
// ============================================================================

const BASE_URL = __ENV.BASE_URL;
const API_KEY = __ENV.API_KEY;
const TOKEN = __ENV.TOKEN;
const PRODUCT_UUID = __ENV.PRODUCT_UUID;
const QTY = parseInt(__ENV.QTY || "1", 10);
const ORDER_RATE = parseInt(__ENV.ORDER_RATE || "10", 10);
const DURATION = __ENV.DURATION || "30s";

export const options = {
  summaryTrendStats: ["avg", "min", "med", "max", "p(90)", "p(95)", "p(99)"],
  scenarios: {
    burst: {
      executor: "constant-arrival-rate",
      rate: ORDER_RATE,
      timeUnit: "1s",
      duration: DURATION,
      preAllocatedVUs: 20,
      maxVUs: 100,
    },
  },
  thresholds: {
    "http_req_duration{ep:create_order}": ["p(95)<1500"],
    http_req_failed: ["rate<0.05"],
    checks: ["rate>0.95"],
  },
};

export function setup() {
  if (!BASE_URL || !API_KEY || !TOKEN || !PRODUCT_UUID) {
    fail("Thiếu biến môi trường: -e BASE_URL=... -e API_KEY=... -e TOKEN=... -e PRODUCT_UUID=...");
  }
}

export default function () {
  const key = `k6-burst-${__VU}-${__ITER}-${Date.now()}`;
  const payload = JSON.stringify({
    items: [{ product_uuid: PRODUCT_UUID, quantity: QTY }],
    shipping_name: "k6 Burst",
    shipping_phone: "0900000000",
    shipping_address: "123 Loadtest Street",
    shipping_fee: 30000,
    payment_method: 1,
    note: "k6 order burst",
  });
  const res = http.post(`${BASE_URL}/api/v1/orders`, payload, {
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
      "X-API-KEY": API_KEY,
      Authorization: `Bearer ${TOKEN}`,
      "X-Idempotency-Key": key,
    },
    timeout: "15s",
    tags: { ep: "create_order" },
  });
  check(res, { "create_order 200/201": (r) => r.status === 200 || r.status === 201 });
}
