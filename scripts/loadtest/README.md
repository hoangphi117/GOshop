# 🚀 GOshop Performance & Load Testing Suite (Grafana k6)

Bộ kịch bản kiểm thử hiệu năng và sức tải hệ thống chạy trên **Grafana k6 (Docker)**, không cần cài đặt thêm runtime trên máy.

---

## 📋 Các kịch bản có sẵn

| Lệnh              | Kịch bản        | Mục đích                                                               | Số lượng Virtual Users (VUs) / Rate          |
| :---------------- | :-------------- | :--------------------------------------------------------------------- | :------------------------------------------- |
| `make loadtest`   | **Load Test**   | Kiểm tra hoạt động bình thường, đo P95 Latency & RPS với tải ổn định.  | Ramping arrival rate lên mức tiêu chuẩn      |
| `make stresstest` | **Stress Test** | Tìm điểm gãy (Breaking point), tải cực đại của DB & Go API.            | Ramping arrival rate lên tới 600 req/s       |
| `make spiketest`  | **Spike Test**  | Giả lập mở bán Flash Sale (lượng truy cập tăng vọt đột ngột trong 5s). | Đột biến từ baseline lên 1000 req/s trong 5s |

---

## ⚡ Hướng dẫn chạy nhanh

1. **Khởi động server Production** (để test Nginx Load Balancer & Multi-Instance Go):
   ```bash
   make prod
   ```
