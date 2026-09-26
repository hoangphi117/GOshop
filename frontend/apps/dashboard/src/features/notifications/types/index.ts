export type NotificationType = 'order' | 'return' | 'stock' | 'review' | 'system' | string

export interface NotificationDTO {
  uuid: string
  notification_uuid?: string
  user_uuid?: string
  type: NotificationType
  title: string
  body: string
  data?: Record<string, unknown>
  is_read: boolean
  read_at?: string | null
  created_at: string
}

export interface UnreadCountDTO {
  unread_count: number
}

export interface AdminGetNotificationsParams {
  page?: number
  limit?: number
  type?: string
  is_read?: boolean
}

export interface PaginationMeta {
  page: number
  limit: number
  total_records: number
  total_pages: number
  has_next?: boolean
  has_prev?: boolean
}

export interface PaginatedNotifications {
  items: NotificationDTO[]
  pagination?: PaginationMeta
  total?: number
}

export type NotificationFilterTab = 'all' | 'order' | 'return' | 'stock' | 'review' | 'unread'
