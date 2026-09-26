import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { Bell, CheckCheck, ExternalLink, RefreshCw, Settings, X } from 'lucide-react'
import { cn } from '@/lib/utils'
import {
  useAdminNotifications,
  useMarkAllAsRead,
  useUnreadNotificationCount,
} from '../hooks/use-notifications'
import type {
  AdminGetNotificationsParams,
  NotificationFilterTab,
} from '../types'
import { NotificationItem } from './notification-item'
import { NotificationSettingsDialog } from './notification-settings-dialog'

interface NotificationDropdownProps {
  onClose: () => void
}

const TABS: { id: NotificationFilterTab; label: string }[] = [
  { id: 'all', label: 'All' },
  { id: 'unread', label: 'Unread' },
  { id: 'order', label: 'Orders' },
  { id: 'return', label: 'Returns' },
  { id: 'stock', label: 'Stock' },
  { id: 'review', label: 'Reviews' },
]

export function NotificationDropdown({ onClose }: NotificationDropdownProps) {
  const [activeTab, setActiveTab] = useState<NotificationFilterTab>('all')
  const [settingsOpen, setSettingsOpen] = useState(false)
  const navigate = useNavigate()
  const markAllAsRead = useMarkAllAsRead()

  const { data: unreadData, refetch: refetchUnread } =
    useUnreadNotificationCount()
  const unreadCount = unreadData?.unread_count ?? 0

  const queryParams: AdminGetNotificationsParams = {
    limit: 20,
    ...(activeTab === 'unread'
      ? { is_read: false }
      : activeTab === 'all'
        ? {}
        : { type: activeTab }),
  }

  const { data, isLoading, isFetching, refetch } =
    useAdminNotifications(queryParams)
  const notifications = data?.items || []

  const handleRefresh = () => {
    refetch()
    refetchUnread()
  }

  const getEmptyState = () => {
    switch (activeTab) {
      case 'unread':
        return {
          title: 'No unread notifications',
          subtitle: 'You have read all received updates.',
        }
      case 'order':
        return {
          title: 'No order alerts',
          subtitle: 'New orders and status updates will appear here.',
        }
      case 'return':
        return {
          title: 'No return requests',
          subtitle: 'Return and refund requests will appear here.',
        }
      case 'stock':
        return {
          title: 'Stock levels normal',
          subtitle: 'Low inventory and stockout warnings will appear here.',
        }
      case 'review':
        return {
          title: 'No new reviews',
          subtitle: 'Customer ratings and product reviews will appear here.',
        }
      default:
        return {
          title: 'No notifications yet',
          subtitle:
            'Important admin updates and activity alerts will appear here.',
        }
    }
  }

  const emptyState = getEmptyState()

  const hasUnread = unreadCount > 0 || notifications.some((n) => !n.is_read)

  return (
    <div className='flex w-full flex-col'>
      {/* Header */}
      <div className='flex items-center justify-between border-b border-border/70 bg-muted/40 p-4'>
        <div className='flex items-center gap-2.5'>
          <div className='flex size-8 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary'>
            <Bell className='size-4' />
          </div>
          <div>
            <h3 className='text-sm font-extrabold tracking-tight text-foreground'>
              Notifications
            </h3>
            <p className='text-[11px] font-medium text-muted-foreground'>
              {unreadCount > 0
                ? `${unreadCount} unread update${unreadCount > 1 ? 's' : ''}`
                : 'All caught up'}
            </p>
          </div>
        </div>

        <div className='flex items-center gap-1.5'>
          {hasUnread && (
            <button
              type='button'
              onClick={() => markAllAsRead.mutate()}
              disabled={markAllAsRead.isPending}
              className='flex cursor-pointer items-center gap-1 rounded-xl border border-border/80 bg-background px-2.5 py-1.5 text-[11px] font-semibold text-foreground shadow-2xs transition-colors hover:bg-muted disabled:opacity-50'
              title='Mark all as read'
            >
              <CheckCheck className='size-3 text-primary' />
              <span>Mark all read</span>
            </button>
          )}

          <button
            type='button'
            onClick={handleRefresh}
            className='cursor-pointer rounded-xl p-1.5 text-muted-foreground transition-colors hover:bg-muted/80 hover:text-foreground'
            title='Refresh'
          >
            <RefreshCw
              className={cn(
                'size-3.5',
                (isLoading || isFetching) && 'animate-spin'
              )}
            />
          </button>

          <button
            type='button'
            onClick={() => setSettingsOpen(true)}
            className='cursor-pointer rounded-xl p-1.5 text-muted-foreground transition-colors hover:bg-muted/80 hover:text-foreground'
            title='Notification preferences'
          >
            <Settings className='size-3.5' />
          </button>

          <button
            type='button'
            onClick={onClose}
            className='cursor-pointer rounded-xl p-1.5 text-muted-foreground transition-colors hover:bg-muted/80 hover:text-foreground'
            title='Close'
          >
            <X className='size-3.5' />
          </button>
        </div>
      </div>

      {/* Pill Filter Tabs */}
      <div className='flex flex-wrap items-center gap-1.5 border-b border-border/60 bg-card px-4 py-2.5'>
        {TABS.map((tab) => {
          const isActive = activeTab === tab.id
          return (
            <button
              key={tab.id}
              type='button'
              onClick={() => setActiveTab(tab.id)}
              className={cn(
                'flex cursor-pointer items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-semibold whitespace-nowrap transition-all',
                isActive
                  ? 'bg-primary text-primary-foreground shadow-2xs'
                  : 'text-muted-foreground hover:bg-muted/70 hover:text-foreground'
              )}
            >
              <span>{tab.label}</span>
              {tab.id === 'unread' && unreadCount > 0 ? (
                <span
                  className={cn(
                    'py-0.2 rounded-full px-1.5 text-[10px] font-bold',
                    isActive
                      ? 'bg-primary-foreground text-primary'
                      : 'bg-rose-100 text-rose-700 dark:bg-rose-950 dark:text-rose-300'
                  )}
                >
                  {unreadCount > 99 ? '99+' : unreadCount}
                </span>
              ) : null}
            </button>
          )
        })}
      </div>

      {/* Scrollable Notification List */}
      <div className='max-h-[360px] divide-y divide-border/40 overflow-y-auto'>
        {isLoading && notifications.length === 0 ? (
          <div className='space-y-2 py-12 text-center text-muted-foreground'>
            <RefreshCw className='mx-auto size-6 animate-spin text-primary' />
            <p className='text-xs font-medium'>Loading notifications...</p>
          </div>
        ) : notifications.length === 0 ? (
          <div className='space-y-2 px-6 py-12 text-center'>
            <p className='text-xs font-bold text-foreground'>
              {emptyState.title}
            </p>
            <p className='mx-auto max-w-[240px] text-[11px] leading-relaxed text-muted-foreground'>
              {emptyState.subtitle}
            </p>
          </div>
        ) : (
          notifications.map((notification) => (
            <NotificationItem
              key={notification.uuid || notification.notification_uuid}
              notification={notification}
              onClosePopover={onClose}
            />
          ))
        )}
      </div>

      {/* Footer */}
      <div className='border-t border-border/70 bg-muted/40 p-3 text-center'>
        <button
          type='button'
          onClick={() => {
            onClose()
            navigate({ to: '/orders' })
          }}
          className='inline-flex cursor-pointer items-center gap-1.5 text-xs font-bold text-primary transition-colors hover:underline'
        >
          <span>View Order Activity</span>
          <ExternalLink className='size-3' />
        </button>
      </div>

      <NotificationSettingsDialog
        open={settingsOpen}
        onOpenChange={setSettingsOpen}
      />
    </div>
  )
}
