# Admin Notification Enhancements (Tasks 2, 3, 4, 5) Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a production-grade enterprise notification ecosystem including a dedicated management page, native browser push alerts, user sound/category preferences, and automated data retention.

**Architecture:** 
- **Frontend (Dashboard):** Dedicated TanStack Router page (`/notifications`) with advanced filtering and bulk mutations; Zustand-persisted notification preferences (`sound`, `push`, `channels`); Browser `Notification` API integration on background tab events.
- **Backend (Worker & DB):** Scheduled 24h cron goroutine in `backend/cmd/worker/main.go` using SQLC index-optimized cleanup query to purge read notifications older than 30 days without impacting API transaction throughput.

**Tech Stack:** React 19, TanStack Router, TanStack Query, Tailwind CSS, Web Notification API, Go 1.23, SQLC, PostgreSQL, Zerolog.

---

### Task 1: Notification Preferences Store & Settings Modal (Enhancement 4)

**Files:**
- Create: `frontend/apps/dashboard/src/features/notifications/stores/notification-settings-store.ts`
- Create: `frontend/apps/dashboard/src/features/notifications/components/notification-settings-dialog.tsx`
- Modify: `frontend/apps/dashboard/src/features/notifications/hooks/use-notification-socket.ts`
- Modify: `frontend/apps/dashboard/src/features/notifications/components/notification-dropdown.tsx`

**Step 1: Create the Zustand settings store with local storage persistence**
- State: `soundEnabled` (boolean, default `true`), `desktopPushEnabled` (boolean, default `false`), `mutedTypes` (string[], default `[]`).
- Actions: `setSoundEnabled`, `setDesktopPushEnabled`, `toggleMutedType`.

**Step 2: Build the settings dialog component**
- Accessible dialog allowing admins to toggle sound effects, opt into desktop notifications, and check/uncheck notification channels (Orders, Returns, Stock, Reviews).

**Step 3: Connect settings store into WebSocket listener**
- In `use-notification-socket.ts`: Check `soundEnabled` before triggering audio chime; check `mutedTypes.includes(item.type)` before showing toasts or firing sound.

**Step 4: Add settings button trigger to NotificationDropdown**
- Add `<Settings className="size-3.5" />` icon button next to refresh/close buttons in the dropdown header.

**Step 5: Verify build & commit**
- Run `pnpm --filter dashboard build`
- Git commit: `feat(dashboard): add notification preferences store and settings dialog`

---

### Task 2: Native Desktop Web Push Notifications (Enhancement 3)

**Files:**
- Create: `frontend/apps/dashboard/src/features/notifications/hooks/use-web-push.ts`
- Modify: `frontend/apps/dashboard/src/features/notifications/hooks/use-notification-socket.ts`

**Step 1: Create the `useWebPush` hook**
- Check browser `Notification` support (`'Notification' in window`).
- Expose `requestPermission(): Promise<boolean>`.
- Expose `sendNotification(title: string, options: NotificationOptions): void`.
- Attach `onclick` listener to focus the dashboard tab and navigate to the entity if applicable.

**Step 2: Integrate into `use-notification-socket.ts`**
- When a new WebSocket notification event arrives:
  - If `document.visibilityState === 'hidden'` and `desktopPushEnabled === true` and `Notification.permission === 'granted'`:
    - Display OS native notification with title, body, and icon.

**Step 3: Test and verify**
- Test with mock event when tab is unfocused.
- Run `pnpm --filter dashboard build`.

**Step 4: Commit**
- Git commit: `feat(dashboard): integrate native browser desktop notifications`

---

### Task 3: Dedicated Notification Management Page (`/notifications`) (Enhancement 2)

**Files:**
- Create: `frontend/apps/dashboard/src/routes/_authenticated/notifications/index.tsx`
- Create: `frontend/apps/dashboard/src/features/notifications/pages/notifications-page.tsx`
- Create: `frontend/apps/dashboard/src/features/notifications/components/notifications-table.tsx`
- Create: `frontend/apps/dashboard/src/features/notifications/components/notifications-filter-bar.tsx`
- Modify: `frontend/apps/dashboard/src/features/notifications/components/notification-dropdown.tsx`

**Step 1: Create route file**
- Define `Route = createFileRoute('/_authenticated/notifications/')({ component: NotificationsPage })`.

**Step 2: Build `NotificationsFilterBar`**
- Search input (debounced 300ms).
- Type select dropdown (`All`, `Order`, `Return`, `Stock`, `Review`).
- Read status filter (`All`, `Unread`, `Read`).
- Bulk action toolbar (shows when items are checked: Mark Selected as Read, Delete Selected).

**Step 3: Build `NotificationsTable` with bulk selection & pagination**
- Checkbox column for batch operations.
- Type icon + badge column.
- Title and body with preview.
- Timestamp with exact date + relative time tooltip.
- Action buttons per row (Mark read/unread, Delete, View Detail link).
- Pagination footer (rows per page, current page, Next / Previous buttons).

**Step 4: Connect dropdown footer link**
- Update dropdown footer link from "View Order Activity" to "View All Notifications" linking to `/notifications`.

**Step 5: Verify build & commit**
- Run `pnpm --filter dashboard build`.
- Git commit: `feat(dashboard): implement full-featured notification management page`

---

### Task 4: Backend Data Retention & Auto-Cleanup Worker (Enhancement 5)

**Files:**
- Modify: `backend/internal/db/queries/notifications.sql`
- Generate SQLC: `backend/internal/db/sqlc/notifications.sql.go`
- Modify: `backend/internal/repository/interfaces.go`
- Modify: `backend/internal/repository/notification_repository.go`
- Modify: `backend/internal/service/v1/interfaces.go`
- Modify: `backend/internal/service/v1/notification_service.go`
- Modify: `backend/cmd/worker/main.go`
- Test: `backend/internal/worker/cleanup_test.go`

**Step 1: Add cleanup query in `notifications.sql`**
```sql
-- name: DeleteOldReadNotifications :execrows
DELETE FROM notifications
WHERE is_read = TRUE 
  AND created_at < $1;
```

**Step 2: Run sqlc generate or implement repo method**
- Add `DeleteOldRead(ctx context.Context, cutoff time.Time) (int64, error)` to repository.

**Step 3: Add `StartRetentionCleanup` in `backend/cmd/worker/main.go`**
- Initialize a `time.NewTicker(24 * time.Hour)` goroutine.
- Calculate cutoff: `time.Now().AddDate(0, 0, -30)` (30 days ago).
- Execute cleanup in background and log deleted count:
  `log.Info().Int64("deleted_count", count).Msg("Stale read notifications purged successfully")`.
- Ensure clean shutdown on worker SIGTERM via context cancellation.

**Step 4: Run Go tests**
- Run `go test ./internal/worker/...` and verify clean execution.

**Step 5: Commit**
- Git commit: `feat(worker): add automated 30-day retention cleanup for read notifications`

---

## Execution Handoff

Plan complete and saved to `docs/plans/2026-09-26-admin-notification-enhancements.md`. Two execution options:

1. **Subagent-Driven (this session)** - Implement task-by-task with immediate test & build checks.
2. **Parallel Session (separate)** - Open new session with executing-plans.

Which approach would you like to take?
