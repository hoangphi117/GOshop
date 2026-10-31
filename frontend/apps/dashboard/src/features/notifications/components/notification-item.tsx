import { formatDistanceToNow } from 'date-fns'
import {
  AlertTriangle,
  Bell,
  Check,
  MessageSquare,
  Package,
  RotateCcw,
  Trash2,
} from 'lucide-react'
import { useNavigate } from '@tanstack/react-router'
import { cn } from '@/lib/utils'
import { useDeleteNotification, useMarkAsRead } from '../hooks/use-notifications'
import type { NotificationDTO } from '../types'

interface NotificationItemProps {
  notification: NotificationDTO
  onClosePopover?: () => void
}

export function NotificationItem({ notification, onClosePopover }: NotificationItemProps) {
  const navigate = useNavigate()
  const markAsRead = useMarkAsRead()
  const deleteMutation = useDeleteNotification()

  const getVisual = (type: string) => {
    const t = (type || '').toLowerCase()
    if (t.includes('order')) {
      return {
        icon: <Package className='size-4 text-blue-600 dark:text-blue-400' />,
        bg: 'bg-blue-500/10 border-blue-500/20',
      }
    }
    if (t.includes('return')) {
      return {
        icon: <RotateCcw className='size-4 text-amber-600 dark:text-amber-400' />,
        bg: 'bg-amber-500/10 border-amber-500/20',
      }
    }
    if (t.includes('stock') || t.includes('inventory')) {
      return {
        icon: <AlertTriangle className='size-4 text-rose-600 dark:text-rose-400' />,
        bg: 'bg-rose-500/10 border-rose-500/20',
      }
    }
    if (t.includes('review')) {
      return {
        icon: <MessageSquare className='size-4 text-cyan-600 dark:text-cyan-400' />,
        bg: 'bg-cyan-500/10 border-cyan-500/20',
      }
    }
    return {
      icon: <Bell className='size-4 text-primary' />,
      bg: 'bg-primary/10 border-primary/20',
    }
  }

  const notifId = notification.uuid || notification.notification_uuid || ''

  const handleClick = () => {
    if (!notification.is_read && notifId) {
      markAsRead.mutate(notifId)
    }

    onClosePopover?.()

    const data = notification.data
    if (data?.order_uuid && typeof data.order_uuid === 'string') {
      navigate({ to: '/orders/$uuid', params: { uuid: data.order_uuid } })
      return
    }

    const t = (notification.type || '').toLowerCase()
    if (t.includes('order')) {
      navigate({ to: '/orders' })
    } else if (t.includes('return')) {
      navigate({ to: '/returns' })
    } else if (t.includes('stock') || t.includes('inventory')) {
      navigate({ to: '/inventory' })
    } else if (t.includes('review')) {
      navigate({ to: '/reviews' })
    }
  }

  const handleMarkRead = (e: React.MouseEvent) => {
    e.stopPropagation()
    if (notifId) {
      markAsRead.mutate(notifId)
    }
  }

  const handleDelete = (e: React.MouseEvent) => {
    e.stopPropagation()
    if (notifId) {
      deleteMutation.mutate(notifId)
    }
  }

  const timeAgo = (() => {
    try {
      return formatDistanceToNow(new Date(notification.created_at), { addSuffix: true })
    } catch {
      return 'just now'
    }
  })()

  const visual = getVisual(notification.type)
  const orderNumber = notification.data?.order_number as string | undefined

  return (
    <div
      onClick={handleClick}
      className={cn(
        'group relative flex cursor-pointer items-start gap-3 p-3.5 transition-all text-left border-b border-border/40 last:border-b-0 hover:bg-muted/50',
        !notification.is_read ? 'bg-primary/5 hover:bg-primary/10' : 'bg-card'
      )}
    >
      {/* Squircle visual icon badge */}
      <div
        className={cn(
          'size-9 rounded-2xl flex items-center justify-center shrink-0 border transition-transform group-hover:scale-105',
          visual.bg
        )}
      >
        {visual.icon}
      </div>

      {/* Content */}
      <div className='min-w-0 flex-1 space-y-0.5'>
        <div className='flex items-center justify-between gap-1'>
          <h4
            className={cn(
              'truncate text-xs text-foreground',
              !notification.is_read ? 'font-bold' : 'font-medium text-foreground/80'
            )}
          >
            {notification.title}
          </h4>
          <span className='shrink-0 text-[10px] text-muted-foreground font-medium'>{timeAgo}</span>
        </div>

        <p className='line-clamp-2 text-[11px] text-muted-foreground leading-relaxed'>
          {notification.body}
        </p>

        {orderNumber ? (
          <div className='pt-1 flex items-center gap-1.5 text-[10px] text-muted-foreground font-mono'>
            <span className='px-1.5 py-0.5 rounded bg-muted text-foreground/80 font-bold'>
              #{orderNumber}
            </span>
          </div>
        ) : null}
      </div>

      {/* Right indicators & quick actions */}
      <div className='flex flex-col items-end gap-1.5 shrink-0 pt-0.5'>
        {!notification.is_read ? (
          <span className='size-2 rounded-full bg-primary shadow-xs' />
        ) : (
          <span className='size-2' />
        )}

        <div className='opacity-0 group-hover:opacity-100 flex items-center gap-1 transition-opacity'>
          {!notification.is_read ? (
            <button
              type='button'
              onClick={handleMarkRead}
              className='p-1 rounded-lg hover:bg-primary/10 text-primary transition-colors cursor-pointer'
              title='Mark as read'
            >
              <Check className='size-3' />
            </button>
          ) : null}

          <button
            type='button'
            onClick={handleDelete}
            className='p-1 rounded-lg hover:bg-rose-500/10 text-rose-500 transition-colors cursor-pointer'
            title='Delete notification'
          >
            <Trash2 className='size-3' />
          </button>
        </div>
      </div>
    </div>
  )
}
