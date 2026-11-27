# Backend Test Coverage Plan

> **Generated:** 2026-11-27  
> **Branch:** `perf/dashboard-query-optimization`  
> **Status:** 🔴 Incomplete — 14 of 16 services have critical coverage gaps

---

## Coverage Snapshot (Current)

| Service | Test Files | Service % | Handler % | Repo % | Priority |
|:--------|:----------:|:---------:|:---------:|:------:|:--------:|
| `order` | 22 | **68.6%** | 4.2% | 0% | P1 — handler gap |
| `recommendations` | 19 | ~65% | Has test | Has test | P2 — good shape |
| `assistant` | 12 | ~60% | Has test | — | P2 — good shape |
| `payment` | 6 | **48.7%** | 0% | 0% | P1 — handler + repo |
| `cart` | 3 | **50.0%** | 0% | 0% | P1 — handler + repo |
| `analytics` | 4 | **33.0%** | 0% | 0% | P1 — handler + repo |
| `identity` | 9 | **3.9%** | 9.7% | 0% | 🔴 P0 — business logic |
| `catalog` | 5 | **9.2%** | 0% | 4.2% | 🔴 P0 — wide surface |
| `inventory` | 2 | **0.0%** | 0% | 0% | 🔴 P0 — zero coverage |
| `review` | 2 | Has tests | 0% | 0% | P1 — handler + admin |
| `notification` | 2 | partial | 0% | 0% | P2 |
| `banner` | 2 | Has tests | 0% | 0% | P2 |
| `coupon` | 3 | partial | 0% | 0% | P2 |
| `chat` | 4 | partial | 0% | 0% | P2 |
| `media` | 2 | 0% | Has test | — | P2 |

---

## P0 — Báo Động Đỏ (Implement First)

> Các service có coverage < 15% hoặc nghiệp vụ có rủi ro tài chính/bảo mật cao.

---

### P0.1 — `inventory/internal/service` (0.0% → target 70%)

**File cần tạo:** `backend/gomod/services/inventory/internal/service/inventory_service_test.go`

**Mock cần tạo:** `backend/gomod/services/inventory/internal/service/mock_repo_test.go`

**Các test case:**

| Test Function | Mô tả | Priority |
|:---|:---|:---:|
| `TestLockAndDeductStock_SuccessReducesStock` | Trừ kho thành công, tạo stock movement | High |
| `TestLockAndDeductStock_InsufficientStockReturnsError` | Kho không đủ → trả error không giảm DB | High |
| `TestLockAndDeductStock_IdempotentWithSameRef` | Cùng ref → không trừ kép (idempotency) | High |
| `TestRestoreStockWithRef_RevertsDeduction` | Restore stock sau cancel order | High |
| `TestGetStock_MultipleProducts` | Lấy stock của nhiều product_id cùng lúc | Medium |
| `TestListLowStockProducts_RespectsThreshold` | Chỉ trả sản phẩm có qty < threshold | Medium |
| `TestAdjustStock_RecordsMovement` | AdjustStock ghi log movement đúng loại | Medium |
| `TestListStockMovements_PaginationWorks` | Phân trang movement đúng offset/limit | Low |

**Pattern (mock repo):**
```go
// mock_repo_test.go
type mockInventoryRepo struct {
    deductFn   func(ctx context.Context, params dto.DeductParams) error
    restoreFn  func(ctx context.Context, params dto.RestoreParams) error
    getStockFn func(ctx context.Context, uuids []string, ids []int32) ([]dto.ItemStockDTO, error)
}
```

---

### P0.2 — `identity/internal/service` (3.9% → target 65%)

**Files cần tạo:**
- `backend/gomod/services/identity/internal/service/auth_service_test.go`
- `backend/gomod/services/identity/internal/service/user_service_test.go`

**Auth Service test cases:**

| Test Function | Mô tả | Priority |
|:---|:---|:---:|
| `TestLogin_ValidCredentials_ReturnsTokenPair` | Login đúng → access + refresh token | 🔴 High |
| `TestLogin_InvalidPassword_ReturnsUnauthorized` | Sai pass → không leak thông tin | 🔴 High |
| `TestLogin_NonexistentUser_ReturnsUnauthorized` | User không tồn tại → error giống sai pass | 🔴 High |
| `TestLogin_ExceedRateLimit_ReturnsRateLimited` | 5 lần sai liên tiếp → bị block | High |
| `TestRefreshToken_ValidToken_ReturnsNewPair` | Refresh hợp lệ → token mới, token cũ bị revoke | High |
| `TestRefreshToken_RevokedToken_ReturnsError` | Token đã revoke → Unauthorized | High |
| `TestLogout_RevokesTokens` | Logout → cả 2 token bị blacklist | Medium |
| `TestRegister_DuplicateEmail_ReturnsConflict` | Email đã tồn tại → 409 | Medium |
| `TestResetPassword_ExpiredToken_ReturnsError` | Token reset hết hạn → rejected | Medium |
| `TestGoogleLogin_CreatesUserOnFirstLogin` | Google OAuth mới → tạo user | Medium |

**User Service test cases:**

| Test Function | Mô tả | Priority |
|:---|:---|:---:|
| `TestGetAllUsers_FiltersAndPaginates` | Search + filter level/status + phân trang | Medium |
| `TestCreateUser_PublishesUserSyncEvent` | Tạo user → publish Kafka event | High |
| `TestSoftDeleteUser_HidesFromQuery` | Xóa mềm không xóa DB, không tìm thấy | Medium |
| `TestChangePassword_WrongCurrentPassword` | Sai pass hiện tại → reject | 🔴 High |
| `TestUpdateProfile_PropagatesEvent` | Cập nhật profile → event sync | Medium |

---

### P0.3 — `catalog/internal/service` (9.2% → target 55%)

**Files cần tạo:**
- `backend/gomod/services/catalog/internal/service/product_service_test.go`
- `backend/gomod/services/catalog/internal/service/category_service_test.go`
- `backend/gomod/services/catalog/internal/service/variant_service_test.go`

**Product Service test cases:**

| Test Function | Mô tả | Priority |
|:---|:---|:---:|
| `TestCreateProduct_QueuesInitialStock` | Tạo sản phẩm → gọi inventory để init stock | 🔴 High |
| `TestCreateProduct_CompensatesOnInventoryError` | Inventory lỗi → xóa product đã tạo (saga) | 🔴 High |
| `TestGetProductBySlug_RewritesImageURLs` | URL ảnh được rewrite qua CDN | Medium |
| `TestListProducts_FiltersActiveCategoryOnly` | active_only=true → chỉ trả category active | Medium |
| `TestSoftDeleteProduct_RemovesFromSearch` | Xóa → trigger ES index remove | Medium |
| `TestCreateProduct_PublishesOutboxEvent` | Outbox event được persist vào DB | High |
| `TestEnrichStockForDTO_CallsInventoryGRPC` | Enrich stock → gọi inventory contract | Medium |

**Category Service test cases:**

| Test Function | Mô tả | Priority |
|:---|:---|:---:|
| `TestCreateCategory_DuplicateSlugReturnsConflict` | Slug trùng → 409 | Medium |
| `TestGetAllCategories_ActiveOnlyFilter` | active_only=true → lọc đúng | Low |
| `TestSoftDeleteCategory_BlockedIfHasProducts` | Xóa category có product → reject | Medium |
| `TestRestoreCategory_ResetsDeletedAt` | Restore → deleted_at = NULL | Low |

---

## P1 — Gaps Quan Trọng

> Handler và nghiệp vụ còn thiếu ở các service đã có service test.

---

### P1.1 — `payment/internal/handler` (0% → target 60%)

**File cần tạo:** `backend/gomod/services/payment/internal/handler/payment_handler_test.go`

| Test Function | Mô tả |
|:---|:---|
| `TestCreatePaymentIntent_ValidOrder_Returns200` | POST /payments → trả URL thanh toán |
| `TestCreatePaymentIntent_MissingOrder_Returns404` | Order không tồn tại → 404 |
| `TestProcessWebhook_ValidSignature_Returns200` | Webhook hợp lệ → 200, cập nhật status |
| `TestProcessWebhook_InvalidSignature_Returns400` | Webhook giả → 400, không xử lý |
| `TestConfirmCOD_NonCODOrder_Returns422` | COD confirm trên đơn online → 422 |
| `TestGetTransactionsByOrder_Returns200` | Lấy danh sách transaction theo order UUID |

---

### P1.2 — `analytics/internal/handler` (0% → target 50%)

**File cần tạo:** `backend/gomod/services/analytics/internal/handler/analytics_handler_test.go`

| Test Function | Mô tả |
|:---|:---|
| `TestGetOverview_ValidRequest_Returns200` | GET /admin/analytics/overview với date range |
| `TestGetOverview_MissingAuth_Returns401` | Không có token → 401 |
| `TestGetRevenueChart_CachedResponse_Returns200Fast` | Cache hit → response < 10ms |
| `TestGetTopProducts_LimitParam_IsRespected` | limit=5 → chỉ trả 5 sản phẩm |
| `TestGetLowStockProducts_EnrichesWithCatalogInfo` | Enrich product name từ catalog |

---

### P1.3 — `catalog/internal/handler` (0% → target 50%)

**File cần tạo:** `backend/gomod/services/catalog/internal/handler/catalog_handler_test.go`

| Test Function | Mô tả |
|:---|:---|
| `TestListProducts_FilterByCategory_Returns200` | Filter theo category slug |
| `TestGetProductBySlug_NotFound_Returns404` | Slug không tồn tại → 404 |
| `TestCreateProduct_AdminOnly_RejectsCustomer` | Role không đủ → 403 |
| `TestCreateProduct_ValidInput_Returns201` | Tạo product hợp lệ → 201 với body |
| `TestListFAQ_SearchQuery_FiltersResults` | Full-text search FAQ |

---

### P1.4 — `inventory/internal/handler` (0% → target 50%)

**File cần tạo:** `backend/gomod/services/inventory/internal/handler/inventory_handler_test.go`

| Test Function | Mô tả |
|:---|:---|
| `TestAdjustStock_AdminOnly_RejectsCustomer` | Role không đủ → 403 |
| `TestAdjustStock_ValidInput_UpdatesStock` | Điều chỉnh stock hợp lệ → 200 |
| `TestGetStock_MultipleUUIDs_Returns200` | Query nhiều UUID cùng lúc |
| `TestListLowStock_ThresholdFilter` | Low stock với threshold custom |
| `TestListStockMovements_PaginationWorks` | Phân trang movement history |

---

### P1.5 — `order/internal/handler` (4.2% → target 60%)

**File cần tạo:** `backend/gomod/services/order/internal/handler/order_handler_test.go`

| Test Function | Mô tả |
|:---|:---|
| `TestPlaceOrder_ValidCart_Returns201` | Đặt hàng hợp lệ → 201 với order UUID |
| `TestPlaceOrder_EmptyCart_Returns400` | Cart rỗng → 400 |
| `TestCancelOrder_NotOwner_Returns403` | Cancel order của người khác → 403 |
| `TestGetMyOrders_Pagination` | Phân trang đơn hàng của user |
| `TestAdminUpdateOrderStatus_ValidTransition` | Admin cập nhật trạng thái đúng flow |

---

## P2 — Nice-to-Have (Repository + Integration Tests)

> Tầng Repository hiện tại hoàn toàn chưa có test. Recommend dùng **testcontainers-go** với Postgres real instance.

### P2.1 — Repository Integration Tests

**Setup chung:** `backend/gomod/services/testkit/testcontainer_pg.go`

```go
// Shared test helper - spin up postgres container for repo tests
func NewTestPostgres(t *testing.T) *pgxpool.Pool {
    // dùng testcontainers-go
}
```

**Repositories cần test theo thứ tự rủi ro:**

| Repo | Methods cần test | Effort |
|:---|:---|:---:|
| `inventory/repository` | `LockAndDeduct`, `RestoreStock` (race condition) | High |
| `order/repository` | `CreateOrder`, `UpdateStatus`, outbox persistence | High |
| `payment/repository` | `UpdateStatusAtomically` (concurrent webhook) | High |
| `identity/repository` | `FindByEmail`, `CountUsers` pagination | Medium |
| `catalog/repository` | `GetProductBySlug` cache invalidation | Medium |

---

### P2.2 — Service Coverage Boosts

| Service | Current % | Target % | Missing Tests |
|:---|:---:|:---:|:---|
| `notification` | partial | 60% | `BroadcastNotification`, `PurgeStale` |
| `review` | partial | 65% | `AdminReplyReview`, `UpdateReviewStatus` |
| `coupon` | partial | 60% | `ApplyCoupon` validation, expiry check |
| `chat` | partial | 60% | WebSocket message routing |
| `analytics/worker` | 19.7% | 50% | Kafka consumer projection |
| `analytics/service` | 33% | 60% | `GetRevenueChart` date range edge cases |

---

## Implementation Order (Suggested Sprint)

```
Week 1 — P0 (Rủi ro cao nhất)
  Day 1-2: inventory/internal/service (mock repo pattern)
  Day 3-4: identity/internal/service/auth_service_test.go
  Day 5:   identity/internal/service/user_service_test.go

Week 2 — P0 + P1
  Day 1-3: catalog/internal/service (product + category + variant)
  Day 4-5: payment/internal/handler + analytics/internal/handler

Week 3 — P1
  Day 1-2: catalog/internal/handler + inventory/internal/handler
  Day 3-4: order/internal/handler
  Day 5:   cart/internal/handler + review/internal/handler (admin ops)

Week 4 — P2
  Day 1-2: Shared testcontainer setup + inventory repo
  Day 3-4: order repo + payment repo (concurrent tests)
  Day 5:   Fill coverage gaps (notification, coupon, chat)
```

---

## Conventions

### Mock Pattern (sử dụng trong dự án)

```go
// Không dùng mockery/gomock — dùng hand-written struct mock
type mockInventoryRepo struct {
    LockFn   func(ctx context.Context, params dto.DeductParams) error
    mu       sync.Mutex
    calls    []string
}

func (m *mockInventoryRepo) LockAndDeductStockWithRef(
    ctx context.Context, params dto.DeductParams,
) error {
    m.mu.Lock()
    m.calls = append(m.calls, "LockAndDeductStockWithRef")
    m.mu.Unlock()
    return m.LockFn(ctx, params)
}
```

### Test Naming Convention

```
Test<Subject>_<Scenario>_<ExpectedOutcome>
// OK:
TestLogin_ValidCredentials_ReturnsTokenPair
TestAdjustStock_InsufficientQty_ReturnsError
// NOT OK:
TestLoginHappy
TestLogin1
```

### Tabular Tests (preferred)

```go
func TestSomething(t *testing.T) {
    cases := []struct {
        name    string
        input   SomeInput
        wantErr bool
        wantCode ErrorCode
    }{
        {"valid input", validInput, false, 0},
        {"empty email", emptyEmailInput, true, ErrBadRequest},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) { ... })
    }
}
```

### Test File Placement

```
services/<name>/internal/service/<name>_service_test.go  ← unit tests
services/<name>/internal/handler/<name>_handler_test.go  ← handler tests
services/<name>/internal/repository/<name>_repo_test.go  ← integration tests (needs DB)
```

---

## Target Coverage After Plan Completion

| Phase | Services Completed | Avg Coverage Target |
|:---|:---|:---:|
| After P0 | inventory, identity, catalog | 55–65% |
| After P1 | + payment, analytics, order handler | 50–60% |
| After P2 | All services | 60–70% |

> **Note:** `db/sqlc`, `dto`, `routes`, `migrations` được exclude khỏi target — đây là generated/config code, không test trực tiếp.
