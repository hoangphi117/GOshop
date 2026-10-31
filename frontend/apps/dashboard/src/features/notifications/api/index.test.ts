import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  deleteNotification,
  fetchAdminNotifications,
  fetchUnreadNotificationCount,
  markAllNotificationsAsRead,
  markNotificationAsRead,
} from './index'

const mockFetchWithAuth = vi.fn()

vi.mock('@/lib/api', () => ({
  fetchWithAuth: (...args: unknown[]) => mockFetchWithAuth(...args),
}))

describe('Admin Notifications API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetchAdminNotifications builds query string and returns data from backend array', async () => {
    const mockItems = [
      {
        uuid: 'notif-1',
        notification_uuid: 'notif-1',
        user_uuid: 'user-1',
        type: 'order',
        title: 'Order Placed',
        body: 'Order #123 has been created',
        is_read: false,
        created_at: '2026-09-26T12:00:00Z',
      },
    ]

    mockFetchWithAuth.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        status: 'success',
        message: 'OK',
        data: mockItems,
        pagination: {
          page: 1,
          limit: 10,
          total_records: 1,
          total_pages: 1,
        },
      }),
    })

    const result = await fetchAdminNotifications({ page: 1, limit: 10, type: 'order', is_read: false })

    expect(mockFetchWithAuth).toHaveBeenCalledWith(
      '/api/v1/admin/notifications?page=1&limit=10&type=order&is_read=false'
    )
    expect(result.items).toEqual(mockItems)
    expect(result.total).toBe(1)
  })

  it('fetchUnreadNotificationCount returns unread count', async () => {
    mockFetchWithAuth.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ code: 200, message: 'OK', data: { unread_count: 5 } }),
    })

    const result = await fetchUnreadNotificationCount()
    expect(mockFetchWithAuth).toHaveBeenCalledWith('/api/v1/admin/notifications/unread')
    expect(result).toEqual({ unread_count: 5 })
  })

  it('markNotificationAsRead calls PATCH endpoint', async () => {
    const mockNotification = {
      notification_uuid: 'notif-123',
      user_uuid: 'user-1',
      type: 'order',
      title: 'Order Placed',
      body: 'Body',
      is_read: true,
      created_at: '2026-09-26T12:00:00Z',
    }

    mockFetchWithAuth.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ code: 200, message: 'OK', data: mockNotification }),
    })

    const result = await markNotificationAsRead('notif-123')
    expect(mockFetchWithAuth).toHaveBeenCalledWith('/api/v1/admin/notifications/notif-123/read', {
      method: 'PATCH',
    })
    expect(result).toEqual(mockNotification)
  })

  it('markAllNotificationsAsRead calls PATCH read-all endpoint', async () => {
    mockFetchWithAuth.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ code: 200, message: 'OK' }),
    })

    await markAllNotificationsAsRead()
    expect(mockFetchWithAuth).toHaveBeenCalledWith('/api/v1/admin/notifications/read-all', {
      method: 'PATCH',
    })
  })

  it('deleteNotification calls DELETE endpoint', async () => {
    mockFetchWithAuth.mockResolvedValueOnce({
      ok: true,
    })

    await deleteNotification('notif-456')
    expect(mockFetchWithAuth).toHaveBeenCalledWith('/api/v1/admin/notifications/notif-456', {
      method: 'DELETE',
    })
  })
})
