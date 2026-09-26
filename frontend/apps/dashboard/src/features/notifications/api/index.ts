import { fetchWithAuth } from '@/lib/api'
import type {
  AdminGetNotificationsParams,
  NotificationDTO,
  PaginatedNotifications,
  UnreadCountDTO,
} from '../types'

interface ApiResponse<T> {
  code?: number
  status?: string
  message: string
  data: T
  pagination?: import('../types').PaginationMeta
}

export async function fetchAdminNotifications(
  params: AdminGetNotificationsParams = {}
): Promise<PaginatedNotifications> {
  const searchParams = new URLSearchParams()
  if (params.page) searchParams.set('page', String(params.page))
  if (params.limit) searchParams.set('limit', String(params.limit))
  if (params.type && params.type !== 'all') searchParams.set('type', params.type)
  if (params.is_read !== undefined) searchParams.set('is_read', String(params.is_read))

  const qs = searchParams.toString()
  const path = `/api/v1/admin/notifications${qs ? `?${qs}` : ''}`

  const res = await fetchWithAuth(path)
  if (!res.ok) {
    throw new Error('Failed to fetch admin notifications')
  }

  const json: ApiResponse<NotificationDTO[] | { items?: NotificationDTO[] }> = await res.json()

  let rawItems: NotificationDTO[] = []
  if (Array.isArray(json.data)) {
    rawItems = json.data
  } else if (json.data && Array.isArray((json.data as { items?: NotificationDTO[] }).items)) {
    rawItems = (json.data as { items: NotificationDTO[] }).items || []
  }

  const items: NotificationDTO[] = rawItems.map((item) => {
    const id = item.uuid || item.notification_uuid || ''
    return {
      ...item,
      uuid: id,
      notification_uuid: id,
    }
  })

  return {
    items,
    pagination: json.pagination,
    total: json.pagination?.total_records ?? items.length,
  }
}

export async function fetchUnreadNotificationCount(): Promise<UnreadCountDTO> {
  const res = await fetchWithAuth('/api/v1/admin/notifications/unread')
  if (!res.ok) {
    throw new Error('Failed to fetch unread notification count')
  }

  const json: ApiResponse<UnreadCountDTO> = await res.json()
  return json.data
}

export async function markNotificationAsRead(uuid: string): Promise<NotificationDTO> {
  const res = await fetchWithAuth(`/api/v1/admin/notifications/${uuid}/read`, {
    method: 'PATCH',
  })
  if (!res.ok) {
    throw new Error('Failed to mark notification as read')
  }

  const json: ApiResponse<NotificationDTO> = await res.json()
  return json.data
}

export async function markAllNotificationsAsRead(): Promise<void> {
  const res = await fetchWithAuth('/api/v1/admin/notifications/read-all', {
    method: 'PATCH',
  })
  if (!res.ok) {
    throw new Error('Failed to mark all notifications as read')
  }
}

export async function deleteNotification(uuid: string): Promise<void> {
  const res = await fetchWithAuth(`/api/v1/admin/notifications/${uuid}`, {
    method: 'DELETE',
  })
  if (!res.ok) {
    throw new Error('Failed to delete notification')
  }
}
