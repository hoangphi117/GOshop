import http from "k6/http";
import { check } from "k6";
import { Counter } from "k6/metrics";

// ============================================================================
// STRESS TEST - goshop (đường đọc: categories, danh sách sản phẩm, chi tiết)
//
// Chạy:
//   k6 run \
//     -e BASE_URL=http://localhost \
//     -e API_KEY=xxxx \
//     -e TOTAL_PRODUCTS=5032 \
//     -e CATEGORY_PARAM=category_uuid \
//     stress_test.js
// ============================================================================

const BASE_URL = __ENV.BASE_URL;
const API_KEY = __ENV.API_KEY;

const TOTAL_PRODUCTS = parseInt(__ENV.TOTAL_PRODUCTS || "5032", 10);
const CATEGORY_PARAM = __ENV.CATEGORY_PARAM || "category_uuid";
const PRODUCT_PATH = __ENV.PRODUCT_PATH || "/api/v1/products";

const BASE_RATE = parseInt(__ENV.BASE_RATE || "100", 10);
const BREAKING_RATE = parseInt(__ENV.BREAKING_RATE || "1500", 10);

export const options = {
  summaryTrendStats: ["avg", "min", "med", "max", "p(90)", "p(95)", "p(99)"],
  scenarios: {
    stress: {
      executor: "ramping-arrival-rate",
      startRate: BASE_RATE,
      timeUnit: "1s",
      preAllocatedVUs: 300,
      maxVUs: 2000,
      stages: [
        { target: BASE_RATE, duration: "30s" }, // Khởi động
        { target: 300, duration: "1m" }, // Tăng dần tải trọng
        { target: BREAKING_RATE, duration: "2m" }, // Đẩy lên mức tìm điểm gãy (Breaking point)
        { target: BREAKING_RATE, duration: "1m" }, // Giữ tải cực đại
        { target: 0, duration: "30s" }, // Hạ tải kết thúc
      ],
    },
  },
  thresholds: {
    "http_req_duration{ep:categories}": ["p(95)<200"],
    "http_req_duration{ep:products_page}": ["p(95)<500", "p(99)<1200"],
    "http_req_duration{ep:products_by_category}": ["p(95)<500", "p(99)<1200"],
    "http_req_duration{ep:product_detail}": ["p(95)<300", "p(99)<800"],

    http_req_failed: [
      "rate<0.05",
      { threshold: "rate<0.5", abortOnFail: true, delayAbortEval: "15s" },
    ],
    checks: ["rate>0.95"],
    dropped_iterations: ["count==0"],

    "status_codes{status:502}": ["count==0"],
    "status_codes{status:503}": ["count==0"],
    "status_codes{status:504}": ["count==0"],
  },
};

const statusCodes = new Counter("status_codes");

const HEADERS = {
  Accept: "application/json",
  "User-Agent": "Grafana k6/2.3.0",
  "X-API-KEY": API_KEY,
};

function randInt(min, max) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}

function pick(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}

function maxPage(limit) {
  return Math.max(1, Math.floor(TOTAL_PRODUCTS / limit));
}

function extractList(body, depth = 0) {
  if (Array.isArray(body)) return body;
  if (!body || typeof body !== "object" || depth > 3) return [];
  for (const key of Object.keys(body)) {
    const found = extractList(body[key], depth + 1);
    if (found.length > 0) return found;
  }
  return [];
}

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
  }

  const pages = [1, 2, 3];
  for (let i = 0; i < 27; i++) pages.push(randInt(1, maxPage(12)));

  for (const page of pages) {
    const url = `${BASE_URL}/api/v1/products?limit=12&page=${page}`;
    const res = http.get(url, { headers: HEADERS, timeout: "10s" });

    if (res.status === 200) {
      for (const p of extractList(safeJson(res))) {
        const id = getId(p);
        if (id !== undefined) productIds.push(id);
      }
    }
  }

  return { categoryIds, productIds };
}

export default function (data) {
  const roll = Math.random();

  if (roll < 0.1) {
    const r = get(`${BASE_URL}/api/v1/categories`, "categories");
    check(r, { "categories 200": (x) => x.status === 200 });
  } else if (roll < 0.5) {
    const limit = pick([12, 24, 36]);
    const page = randInt(1, maxPage(limit));
    const r = get(
      `${BASE_URL}/api/v1/products?limit=${limit}&page=${page}`,
      "products_page",
    );
    check(r, {
      "products_page 200": (x) => x.status === 200,
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
      "product_detail đúng id": (x) =>
        typeof x.body === "string" && x.body.includes(String(id)),
    });
  }
}
