import http from "k6/http";
import { check } from "k6";
import { Counter } from "k6/metrics";

// ============================================================================
// SPIKE TEST - goshop (đường đọc: categories, danh sách sản phẩm, chi tiết)
//
// Chạy:
//   k6 run \
//     -e BASE_URL=http://localhost \
//     -e API_KEY=xxxx \
//     -e TOTAL_PRODUCTS=5032 \
//     -e CATEGORY_PARAM=category_uuid \
//     spike_test.js
//
// Mỗi iteration = đúng 1 request, nên "rate" bên dưới chính là req/s.
// ============================================================================

const BASE_URL = __ENV.BASE_URL;
const API_KEY = __ENV.API_KEY;

// Tổng số sản phẩm trong DB sau khi seed (SELECT count(*) FROM products)
const TOTAL_PRODUCTS = parseInt(__ENV.TOTAL_PRODUCTS || "5032", 10);
// Tên query param lọc theo category (xác nhận bằng curl)
const CATEGORY_PARAM = __ENV.CATEGORY_PARAM || "category_uuid";
// Đường dẫn chi tiết sản phẩm: ${PRODUCT_PATH}/{id}
const PRODUCT_PATH = __ENV.PRODUCT_PATH || "/api/v1/products";

// Mức tải (req/s). Đỉnh nên khoảng 3-5 lần mức tải bình thường của bạn.
const BASE_RATE = parseInt(__ENV.BASE_RATE || "100", 10);
const PEAK_RATE = parseInt(__ENV.PEAK_RATE || "1000", 10);

export const options = {
  summaryTrendStats: ["avg", "min", "med", "max", "p(90)", "p(95)", "p(99)"],
  scenarios: {
    spike: {
      executor: "ramping-arrival-rate",
      startRate: BASE_RATE,
      timeUnit: "1s",
      preAllocatedVUs: 300,
      maxVUs: 1500,
      stages: [
        { target: BASE_RATE, duration: "30s" }, // baseline: lấy mốc so sánh
        { target: PEAK_RATE, duration: "5s" }, // spike: tăng vọt
        { target: PEAK_RATE, duration: "1m" }, // giữ đỉnh
        { target: BASE_RATE, duration: "5s" }, // hạ tải đột ngột
        { target: BASE_RATE, duration: "1m" }, // quan sát khả năng hồi phục
      ],
    },
  },
  thresholds: {
    // Ngưỡng khởi điểm. Chạy 3 lần lấy baseline rồi siết lại còn ~2-3x baseline.
    "http_req_duration{ep:categories}": ["p(95)<150"],
    "http_req_duration{ep:products_page}": ["p(95)<300", "p(99)<800"],
    "http_req_duration{ep:products_by_category}": ["p(95)<300", "p(99)<800"],
    "http_req_duration{ep:product_detail}": ["p(95)<150", "p(99)<500"],

    http_req_failed: [
      "rate<0.01",
      // Hỏng nặng thì dừng sớm, khỏi chạy tiếp vô ích
      { threshold: "rate<0.5", abortOnFail: true, delayAbortEval: "15s" },
    ],
    checks: ["rate>0.99"],

    // != 0 nghĩa là k6 thiếu VU, tải thực tế thấp hơn tải cấu hình => kết quả không còn đúng
    dropped_iterations: ["count==0"],

    // Phát hiện rate limit / quá tải thay vì đọc nhầm thành "server chậm"
    "status_codes{status:429}": ["count==0"],
    "status_codes{status:502}": ["count==0"],
    "status_codes{status:503}": ["count==0"],
    "status_codes{status:504}": ["count==0"],
  },
};

// Đếm số response theo endpoint và status code (xem ở phần cuối của summary)
const statusCodes = new Counter("status_codes");

const HEADERS = {
  Accept: "application/json",
  "User-Agent": "Grafana k6/2.3.0",
  "X-API-KEY": API_KEY,
};

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function randInt(min, max) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}

function pick(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}

// Số trang tối đa phụ thuộc vào limit (5032 sp: limit 12 -> 419 trang, 36 -> 139 trang)
function maxPage(limit) {
  return Math.max(1, Math.floor(TOTAL_PRODUCTS / limit));
}

// Dò mảng đầu tiên trong response, sâu tối đa 3 tầng:
// [..] | {data: [..]} | {data: {products: [..]}} | {items: [..]}
function extractList(body, depth = 0) {
  if (Array.isArray(body)) return body;
  if (!body || typeof body !== "object" || depth > 3) return [];
  for (const key of Object.keys(body)) {
    const found = extractList(body[key], depth + 1);
    if (found.length > 0) return found;
  }
  return [];
}

// Lấy ID với nhiều tên trường phổ biến (API của bạn dùng "uuid")
function getId(obj) {
  if (!obj || typeof obj !== "object") return undefined;
  for (const k of [
    "uuid",
    "id",
    "ID",
    "Id",
    "product_id",
    "productId",
    "slug",
  ]) {
    if (obj[k] !== undefined && obj[k] !== null) return obj[k];
  }
  return undefined;
}

function get(url, ep) {
  const res = http.get(url, {
    headers: HEADERS,
    timeout: "10s",
    tags: { ep },
  });
  statusCodes.add(1, { ep, status: String(res.status) });
  return res;
}

function safeJson(res) {
  try {
    return res.json();
  } catch (e) {
    return null;
  }
}

// ---------------------------------------------------------------------------
// Setup: chạy 1 lần, lấy ID thật để không bị 404 khi gọi chi tiết / lọc category
// ---------------------------------------------------------------------------
export function setup() {
  if (!BASE_URL || !API_KEY) {
    throw new Error("Thiếu biến môi trường: -e BASE_URL=... -e API_KEY=...");
  }

  const categoryIds = [];
  const productIds = [];

  const catRes = http.get(`${BASE_URL}/api/v1/categories`, {
    headers: HEADERS,
    timeout: "10s",
  });
  if (catRes.status === 200) {
    for (const c of extractList(safeJson(catRes))) {
      const id = getId(c);
      if (id !== undefined) categoryIds.push(id);
    }
  } else {
    console.error(`setup: categories trả ${catRes.status}`);
  }

  // Trang 1-3 chắc chắn có dữ liệu, sau đó lấy thêm trang ngẫu nhiên khắp catalog
  const pages = [1, 2, 3];
  for (let i = 0; i < 27; i++) pages.push(randInt(1, maxPage(12)));

  let debugged = false;
  for (const page of pages) {
    const url = `${BASE_URL}/api/v1/products?limit=12&page=${page}`;
    const res = http.get(url, { headers: HEADERS, timeout: "10s" });

    if (res.status !== 200) {
      console.error(`setup: ${url} trả ${res.status}`);
      continue;
    }
    let added = 0;
    for (const p of extractList(safeJson(res))) {
      const id = getId(p);
      if (id !== undefined) {
        productIds.push(id);
        added++;
      }
    }
    if (added === 0 && !debugged) {
      console.error(
        `setup: page=${page} không có ID. Body mẫu: ${String(res.body).slice(0, 400)}`,
      );
      debugged = true;
    }
  }

  console.log(
    `setup: ${categoryIds.length} category ID, ${productIds.length} product ID`,
  );
  if (productIds.length === 0) {
    throw new Error(
      "setup: không lấy được product ID nào, xem log 'Body mẫu' ở trên để biết schema thật",
    );
  }
  return { categoryIds, productIds };
}

// ---------------------------------------------------------------------------
// Kịch bản chính: mỗi iteration chọn 1 loại request theo tỉ lệ
//   10% categories | 40% danh sách theo trang | 25% lọc category | 25% chi tiết
// ---------------------------------------------------------------------------
export default function (data) {
  const roll = Math.random();

  if (roll < 0.1) {
    const r = get(`${BASE_URL}/api/v1/categories`, "categories");
    check(r, {
      "categories 200": (x) => x.status === 200,
    });
  } else if (roll < 0.5) {
    // limit và page ngẫu nhiên: mỗi tổ hợp là 1 cache key khác nhau
    const limit = pick([12, 24, 36]);
    const page = randInt(1, maxPage(limit));
    const r = get(
      `${BASE_URL}/api/v1/products?limit=${limit}&page=${page}`,
      "products_page",
    );
    check(r, {
      "products_page 200": (x) => x.status === 200,
      // trang rỗng chỉ có vài chục byte; trang có dữ liệu lớn hơn nhiều
      "products_page có dữ liệu": (x) =>
        typeof x.body === "string" && x.body.length > 300,
    });
  } else if (roll < 0.75 && data.categoryIds.length > 0) {
    const cat = pick(data.categoryIds);
    const page = randInt(1, 10);
    const r = get(
      `${BASE_URL}/api/v1/products?limit=12&page=${page}&${CATEGORY_PARAM}=${cat}`,
      "products_by_category",
    );
    check(r, {
      "products_by_category 200": (x) => x.status === 200,
      "products_by_category có dữ liệu": (x) =>
        typeof x.body === "string" && x.body.length > 300,
    });
  } else {
    const id = pick(data.productIds);
    const r = get(`${BASE_URL}${PRODUCT_PATH}/${id}`, "product_detail");
    check(r, {
      "product_detail 200": (x) => x.status === 200,
      // body phải chứa đúng id đã yêu cầu
      "product_detail đúng id": (x) =>
        typeof x.body === "string" && x.body.includes(String(id)),
    });
  }
}
