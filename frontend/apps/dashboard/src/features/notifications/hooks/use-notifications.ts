import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  deleteNotification,
  fetchAdminNotifications,
  fetchUnreadNotificationCount,
  markAllNotificationsAsRead,
  markNotificationAsRead,
} from '../api'
import type {
  AdminGetNotificationsParams,
  PaginatedNotifications,
  UnreadCountDTO,
} from '../types'

export const notificationKeys = {
  all: ['admin-notifications'] as const,
  lists: () => ['admin-notifications', 'list'] as const,
  list: (params: AdminGetNotificationsParams) => ['admin-notifications', 'list', params] as const,
  unreadCount: ['admin-notifications', 'unread-count'] as const,
}

export function useAdminNotifications(params: AdminGetNotificationsParams = {}) {
  return useQuery({
    queryKey: notificationKeys.list(params),
    queryFn: () => fetchAdminNotifications(params),
    staleTime: 30 * 1000,
  })
}

export function useUnreadNotificationCount() {
  return useQuery({
    queryKey: notificationKeys.unreadCount,
    queryFn: fetchUnreadNotificationCount,
    staleTime: 15 * 1000,
    refetchInterval: 60 * 1000, // Poll fallback every 60s
  })
}

export function useMarkAsRead() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (uuid: string) => markNotificationAsRead(uuid),
    onMutate: async (uuid: string) => {
      // Cancel outgoing queries
      await queryClient.cancelQueries({ queryKey: notificationKeys.all })

      // Snapshot previous states
      const prevUnread = queryClient.getQueryData<UnreadCountDTO>(notificationKeys.unreadCount)

      // Optimistically decrement unread count
      if (prevUnread && prevUnread.unread_count > 0) {
        queryClient.setQueryData<UnreadCountDTO>(notificationKeys.unreadCount, {
          unread_count: prevUnread.unread_count - 1,
        })
      }

      // Optimistically update list queries only
      queryClient.setQueriesData<PaginatedNotifications>(
        { queryKey: notificationKeys.lists() },
        (old) => {
          if (!old || !Array.isArray(old.items)) return old
          return {
            ...old,
            items: old.items.map((item) => {
              const itemId = item.uuid || item.notification_uuid
              return itemId === uuid ? { ...item, is_read: true } : item
            }),
          }
        }
      )

      return { prevUnread }
    },
    onError: (_err, _uuid, context) => {
      if (context?.prevUnread) {
        queryClient.setQueryData(notificationKeys.unreadCount, context.prevUnread)
      }
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: notificationKeys.all })
    },
  })
}

export function useMarkAllAsRead() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: markAllNotificationsAsRead,
    onMutate: async () => {
      await queryClient.cancelQueries({ queryKey: notificationKeys.all })

      const prevUnread = queryClient.getQueryData<UnreadCountDTO>(notificationKeys.unreadCount)

      // Optimistically clear unread count
      queryClient.setQueryData<UnreadCountDTO>(notificationKeys.unreadCount, {
        unread_count: 0,
      })

      // Optimistically mark all visible items as read in lists only
      queryClient.setQueriesData<PaginatedNotifications>(
        { queryKey: notificationKeys.lists() },
        (old) => {
          if (!old || !Array.isArray(old.items)) return old
          return {
            ...old,
            items: old.items.map((item) => ({ ...item, is_read: true })),
          }
        }
      )

      return { prevUnread }
    },
    onError: (_err, _variables, context) => {
      if (context?.prevUnread) {
        queryClient.setQueryData(notificationKeys.unreadCount, context.prevUnread)
      }
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: notificationKeys.all })
    },
  })
}

export function useDeleteNotification() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (uuid: string) => deleteNotification(uuid),
    onMutate: async (uuid: string) => {
      await queryClient.cancelQueries({ queryKey: notificationKeys.all })

      // Optimistically remove item from list queries only
      queryClient.setQueriesData<PaginatedNotifications>(
        { queryKey: notificationKeys.lists() },
        (old) => {
          if (!old || !Array.isArray(old.items)) return old
          const filtered = old.items.filter((item) => {
            const itemId = item.uuid || item.notification_uuid
            return itemId !== uuid
          })
          const currentTotal = old.total ?? old.pagination?.total_records ?? old.items.length
          return {
            ...old,
            items: filtered,
            total: Math.max(currentTotal - 1, 0),
          }
        }
      )
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: notificationKeys.all })
    },
  })
}
