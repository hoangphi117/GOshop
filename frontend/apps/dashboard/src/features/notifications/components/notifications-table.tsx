import { useNavigate } from '@tanstack/react-router'
import { format, formatDistanceToNow } from 'date-fns'
import {
  AlertTriangle,
  Bell,
  Check,
  ChevronLeft,
  ChevronRight,
  ExternalLink,
  MessageSquare,
  Package,
  RotateCcw,
  Trash2,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { cn } from '@/lib/utils'
import type { NotificationDTO } from '../types'

interface NotificationsTableProps {
  notifications: NotificationDTO[]
  isLoading: boolean
  page: number
  pageSize: number
  totalRecords: number
  onPageChange: (newPage: number) => void
  onPageSizeChange: (newSize: number) => void
  selectedIds: string[]
  onToggleSelect: (id: string) => void
  onSelectAll: (ids: string[]) => void
  onDeselectAll: () => void
  onMarkAsRead: (id: string) => void
  onDeleteNotification: (id: string) => void
}

export function NotificationsTable({
  notifications,
  isLoading,
  page,
  pageSize,
  totalRecords,
  onPageChange,
  onPageSizeChange,
  selectedIds,
  onToggleSelect,
  onSelectAll,
  onDeselectAll,
  onMarkAsRead,
  onDeleteNotification,
}: NotificationsTableProps) {
  const navigate = useNavigate()

  const currentPageIds = notifications
    .map((n) => n.uuid || n.notification_uuid || '')
    .filter(Boolean)

  const isAllSelected =
    currentPageIds.length > 0 &&
    currentPageIds.every((id) => selectedIds.includes(id))

  const handleSelectAllToggle = () => {
    if (isAllSelected) {
      onDeselectAll()
    } else {
      onSelectAll(currentPageIds)
    }
  }

  const getVisual = (type: string) => {
    const t = (type || '').toLowerCase()
    if (t.includes('order')) {
      return {
        label: 'Order',
        icon: Package,
        badgeClass: 'bg-blue-500/10 text-blue-600 border-blue-500/20 dark:text-blue-400',
      }
    }
    if (t.includes('return')) {
      return {
        label: 'Return',
        icon: RotateCcw,
        badgeClass: 'bg-amber-500/10 text-amber-600 border-amber-500/20 dark:text-amber-400',
      }
    }
    if (t.includes('stock') || t.includes('inventory')) {
      return {
        label: 'Stock Alert',
        icon: AlertTriangle,
        badgeClass: 'bg-rose-500/10 text-rose-600 border-rose-500/20 dark:text-rose-400',
      }
    }
    if (t.includes('review')) {
      return {
        label: 'Review',
        icon: MessageSquare,
        badgeClass: 'bg-cyan-500/10 text-cyan-600 border-cyan-500/20 dark:text-cyan-400',
      }
    }
    return {
      label: 'System',
      icon: Bell,
      badgeClass: 'bg-primary/10 text-primary border-primary/20',
    }
  }

  const handleNavigateTarget = (notification: NotificationDTO) => {
    const id = notification.uuid || notification.notification_uuid || ''
    if (!notification.is_read && id) {
      onMarkAsRead(id)
    }

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

  const totalPages = Math.ceil(totalRecords / pageSize) || 1

  return (
    <div className='space-y-4'>
      <div className='overflow-hidden rounded-xl border border-border/70 bg-card shadow-2xs'>
        <Table>
          <TableHeader>
            <TableRow className='hover:bg-transparent'>
              <TableHead className='w-12 text-center'>
                <Checkbox
                  checked={isAllSelected}
                  onCheckedChange={handleSelectAllToggle}
                  aria-label='Select all notifications on this page'
                />
              </TableHead>
              <TableHead className='w-36'>Channel</TableHead>
              <TableHead>Notification</TableHead>
              <TableHead className='w-40'>Received</TableHead>
              <TableHead className='w-24 text-center'>Status</TableHead>
              <TableHead className='w-32 text-right'>Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading && notifications.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} className='h-48 text-center text-muted-foreground'>
                  <div className='flex flex-col items-center justify-center gap-2'>
                    <div className='size-6 animate-spin rounded-full border-2 border-primary border-t-transparent' />
                    <p className='text-xs font-medium'>Loading notifications...</p>
                  </div>
                </TableCell>
              </TableRow>
            ) : notifications.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} className='h-48 text-center'>
                  <div className='flex flex-col items-center justify-center gap-1.5 py-6'>
                    <Bell className='size-8 text-muted-foreground/40' />
                    <p className='text-sm font-bold text-foreground'>No notifications found</p>
                    <p className='text-xs text-muted-foreground'>
                      No alerts match your current filter settings.
                    </p>
                  </div>
                </TableCell>
              </TableRow>
            ) : (
              notifications.map((item) => {
                const id = item.uuid || item.notification_uuid || ''
                const isSelected = selectedIds.includes(id)
                const visual = getVisual(item.type)
                const Icon = visual.icon

                let dateFormatted: string
                let timeAgo = ''
                try {
                  const d = new Date(item.created_at)
                  dateFormatted = format(d, 'MMM dd, yyyy HH:mm')
                  timeAgo = formatDistanceToNow(d, { addSuffix: true })
                } catch {
                  dateFormatted = item.created_at
                }

                return (
                  <TableRow
                    key={id}
                    className={cn(
                      'transition-colors',
                      !item.is_read && 'bg-primary/5 dark:bg-primary/5',
                      isSelected && 'bg-muted/80'
                    )}
                  >
                    <TableCell className='text-center'>
                      <Checkbox
                        checked={isSelected}
                        onCheckedChange={() => onToggleSelect(id)}
                        aria-label={`Select notification ${item.title}`}
                      />
                    </TableCell>

                    <TableCell>
                      <div className='flex items-center gap-2'>
                        <div
                          className={cn(
                            'flex size-7 shrink-0 items-center justify-center rounded-lg border',
                            visual.badgeClass
                          )}
                        >
                          <Icon className='size-3.5' />
                        </div>
                        <span className='text-xs font-semibold text-foreground'>
                          {visual.label}
                        </span>
                      </div>
                    </TableCell>

                    <TableCell>
                      <div
                        onClick={() => handleNavigateTarget(item)}
                        className='group cursor-pointer space-y-0.5'
                      >
                        <div className='flex items-center gap-1.5'>
                          <p
                            className={cn(
                              'text-xs transition-colors group-hover:text-primary',
                              !item.is_read
                                ? 'font-bold text-foreground'
                                : 'font-medium text-foreground/80'
                            )}
                          >
                            {item.title}
                          </p>
                          <ExternalLink className='size-3 opacity-0 transition-opacity group-hover:opacity-100 text-primary' />
                        </div>
                        <p className='line-clamp-1 text-[11px] text-muted-foreground'>
                          {item.body}
                        </p>
                      </div>
                    </TableCell>

                    <TableCell>
                      <div className='space-y-0.5 text-xs'>
                        <p className='font-medium text-foreground'>{timeAgo}</p>
                        <p className='text-[10px] text-muted-foreground'>{dateFormatted}</p>
                      </div>
                    </TableCell>

                    <TableCell className='text-center'>
                      {!item.is_read ? (
                        <span className='inline-flex items-center gap-1 rounded-full bg-rose-500/10 px-2 py-0.5 text-[10px] font-bold text-rose-600 dark:text-rose-400'>
                          <span className='size-1.5 rounded-full bg-rose-500 animate-pulse' />
                          Unread
                        </span>
                      ) : (
                        <span className='inline-flex items-center rounded-full bg-muted px-2 py-0.5 text-[10px] font-medium text-muted-foreground'>
                          Read
                        </span>
                      )}
                    </TableCell>

                    <TableCell className='text-right'>
                      <div className='flex items-center justify-end gap-1'>
                        {!item.is_read && (
                          <Button
                            type='button'
                            variant='ghost'
                            size='sm'
                            onClick={() => onMarkAsRead(id)}
                            className='size-7 p-0 text-muted-foreground hover:text-foreground'
                            title='Mark as read'
                          >
                            <Check className='size-3.5 text-primary' />
                          </Button>
                        )}

                        <Button
                          type='button'
                          variant='ghost'
                          size='sm'
                          onClick={() => onDeleteNotification(id)}
                          className='size-7 p-0 text-muted-foreground hover:text-destructive'
                          title='Delete notification'
                        >
                          <Trash2 className='size-3.5' />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                )
              })
            )}
          </TableBody>
        </Table>
      </div>

      {/* Pagination Bar */}
      <div className='flex flex-wrap items-center justify-between gap-3 text-xs text-muted-foreground'>
        <div className='flex items-center gap-2'>
          <span>Rows per page:</span>
          <Select
            value={String(pageSize)}
            onValueChange={(val) => onPageSizeChange(Number(val))}
          >
            <SelectTrigger className='h-8 w-16 text-xs'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='10'>10</SelectItem>
              <SelectItem value='25'>25</SelectItem>
              <SelectItem value='50'>50</SelectItem>
            </SelectContent>
          </Select>
          <span>
            Showing {Math.min((page - 1) * pageSize + 1, totalRecords)} to{' '}
            {Math.min(page * pageSize, totalRecords)} of {totalRecords} records
          </span>
        </div>

        <div className='flex items-center gap-1.5'>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => onPageChange(page - 1)}
            disabled={page <= 1 || isLoading}
            className='h-8 gap-1 px-2.5 text-xs font-medium'
          >
            <ChevronLeft className='size-3.5' />
            <span>Previous</span>
          </Button>

          <span className='px-2 font-medium text-foreground'>
            Page {page} of {totalPages}
          </span>

          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => onPageChange(page + 1)}
            disabled={page >= totalPages || isLoading}
            className='h-8 gap-1 px-2.5 text-xs font-medium'
          >
            <span>Next</span>
            <ChevronRight className='size-3.5' />
          </Button>
        </div>
      </div>
    </div>
  )
}
