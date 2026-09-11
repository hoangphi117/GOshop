import { create } from 'zustand'
import { authFetch } from '@/lib/api'

export interface NotificationItem {
  uuid: string
  type: string
  title: string
  body: string
  data?: Record<string, any>
  is_read: boolean
  read_at?: string
  created_at: string
}

interface NotificationState {
  notifications: NotificationItem[]
  unreadCount: number
  isLoading: boolean
  isMarkingAllRead: boolean
  fetchNotifications: () => Promise<void>
  fetchUnreadCount: () => Promise<void>
  markAsRead: (uuid: string) => Promise<void>
  markAllAsRead: () => Promise<void>
  deleteNotification: (uuid: string) => Promise<void>
  addNotification: (item: NotificationItem) => void
  reset: () => void
}

export const useNotificationStore = create<NotificationState>((set, get) => ({
  notifications: [],
  unreadCount: 0,
  isLoading: false,
  isMarkingAllRead: false,

  fetchNotifications: async () => {
    set({ isLoading: true })
    try {
      const baseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
      const res = await authFetch(`${baseUrl}/api/v1/notifications/me?page=1&limit=30`)
      if (res.ok) {
        const json = await res.json()
        const items: NotificationItem[] = Array.isArray(json?.data?.items)
          ? json.data.items
          : Array.isArray(json?.data)
          ? json.data
          : []
        set({
          notifications: items,
          unreadCount: items.filter((n) => !n.is_read).length,
        })
      }
    } catch {
      // Ignore network errors silently
    } finally {
      set({ isLoading: false })
    }
  },

  fetchUnreadCount: async () => {
    try {
      const baseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
      const res = await authFetch(`${baseUrl}/api/v1/notifications/me/unread`)
      if (res.ok) {
        const json = await res.json()
        const count =
          typeof json?.data?.unread_count === 'number'
            ? json.data.unread_count
            : 0
        set({ unreadCount: count })
      }
    } catch {
      // Ignore network errors silently
    }
  },

  markAsRead: async (uuid: string) => {
    const prev = get().notifications
    const target = prev.find((n) => n.uuid === uuid)
    if (!target || target.is_read) return

    set({
      notifications: prev.map((n) =>
        n.uuid === uuid ? { ...n, is_read: true, read_at: new Date().toISOString() } : n
      ),
      unreadCount: Math.max(0, get().unreadCount - 1),
    })

    try {
      const baseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
      await authFetch(`${baseUrl}/api/v1/notifications/${uuid}/read`, {
        method: 'PATCH',
      })
    } catch {
      set({ notifications: prev })
    }
  },

  markAllAsRead: async () => {
    const prev = get().notifications
    set({
      isMarkingAllRead: true,
      notifications: prev.map((n) => ({
        ...n,
        is_read: true,
        read_at: new Date().toISOString(),
      })),
      unreadCount: 0,
    })

    try {
      const baseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
      await authFetch(`${baseUrl}/api/v1/notifications/read-all`, {
        method: 'PATCH',
      })
    } catch {
      set({ notifications: prev })
    } finally {
      set({ isMarkingAllRead: false })
    }
  },

  deleteNotification: async (uuid: string) => {
    const prev = get().notifications
    const target = prev.find((n) => n.uuid === uuid)
    const wasUnread = target && !target.is_read

    set({
      notifications: prev.filter((n) => n.uuid !== uuid),
      unreadCount: wasUnread
        ? Math.max(0, get().unreadCount - 1)
        : get().unreadCount,
    })

    try {
      const baseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
      await authFetch(`${baseUrl}/api/v1/notifications/${uuid}`, {
        method: 'DELETE',
      })
    } catch {
      set({ notifications: prev })
    }
  },

  addNotification: (item: NotificationItem) => {
    const prev = get().notifications
    if (prev.some((n) => n.uuid === item.uuid)) return
    set({
      notifications: [item, ...prev],
      unreadCount: !item.is_read ? get().unreadCount + 1 : get().unreadCount,
    })
  },

  reset: () =>
    set({
      notifications: [],
      unreadCount: 0,
      isLoading: false,
      isMarkingAllRead: false,
    }),
}))
