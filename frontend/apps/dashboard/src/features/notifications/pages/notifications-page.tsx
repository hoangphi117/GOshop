import { useMemo, useState } from 'react'
import {
  Bell,
  CheckCheck,
  Package,
  RotateCcw,
  Settings,
  Sparkles,
} from 'lucide-react'
import { toast } from 'sonner'
import { DashboardHeader } from '@/components/layout/dashboard-header'
import { Main } from '@/components/layout/main'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  useAdminNotifications,
  useDeleteNotification,
  useMarkAllAsRead,
  useMarkAsRead,
  useUnreadNotificationCount,
} from '../hooks/use-notifications'
import { NotificationsFilterBar } from '../components/notifications-filter-bar'
import { NotificationsTable } from '../components/notifications-table'
import { NotificationSettingsDialog } from '../components/notification-settings-dialog'

export function NotificationsPage() {
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [search, setSearch] = useState('')
  const [selectedChannel, setSelectedChannel] = useState('all')
  const [selectedStatus, setSelectedStatus] = useState('all')
  const [selectedIds, setSelectedIds] = useState<string[]>([])
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [bulkDeleteOpen, setBulkDeleteOpen] = useState(false)
  const [singleDeleteId, setSingleDeleteId] = useState<string | null>(null)

  // Query params
  const isReadParam = useMemo(() => {
    if (selectedStatus === 'unread') return false
    if (selectedStatus === 'read') return true
    return undefined
  }, [selectedStatus])

  const channelParam = selectedChannel !== 'all' ? selectedChannel : undefined

  const {
    data,
    isLoading,
    isFetching,
    refetch,
  } = useAdminNotifications({
    page,
    limit: pageSize,
    type: channelParam,
    is_read: isReadParam,
  })

  const { data: unreadData } = useUnreadNotificationCount()
  const unreadCount = unreadData?.unread_count ?? 0

  const markAsRead = useMarkAsRead()
  const markAllAsRead = useMarkAllAsRead()
  const deleteMutation = useDeleteNotification()

  // Filter in memory by search text (title or body)
  const filteredItems = useMemo(() => {
    const items = data?.items || []
    if (!search.trim()) return items
    const query = search.toLowerCase()
    return items.filter(
      (item) =>
        (item.title || '').toLowerCase().includes(query) ||
        (item.body || '').toLowerCase().includes(query)
    )
  }, [data?.items, search])

  const totalRecords = data?.total ?? filteredItems.length

  // Handlers
  const handleToggleSelect = (id: string) => {
    setSelectedIds((prev) =>
      prev.includes(id) ? prev.filter((item) => item !== id) : [...prev, id]
    )
  }

  const handleSelectAll = (ids: string[]) => {
    setSelectedIds((prev) => Array.from(new Set([...prev, ...ids])))
  }

  const handleDeselectAll = () => {
    setSelectedIds([])
  }

  const handleMarkSelectedAsRead = async () => {
    if (selectedIds.length === 0) return
    try {
      await Promise.all(selectedIds.map((id) => markAsRead.mutateAsync(id)))
      toast.success(`Marked ${selectedIds.length} notifications as read`)
      setSelectedIds([])
    } catch {
      toast.error('Failed to mark some notifications as read')
    }
  }

  const handleConfirmBulkDelete = async () => {
    if (selectedIds.length === 0) return
    try {
      await Promise.all(selectedIds.map((id) => deleteMutation.mutateAsync(id)))
      toast.success(`Deleted ${selectedIds.length} notifications`)
      setSelectedIds([])
      setBulkDeleteOpen(false)
    } catch {
      toast.error('Failed to delete some notifications')
    }
  }

  const handleConfirmSingleDelete = async () => {
    if (!singleDeleteId) return
    try {
      await deleteMutation.mutateAsync(singleDeleteId)
      toast.success('Notification deleted')
      setSelectedIds((prev) => prev.filter((id) => id !== singleDeleteId))
      setSingleDeleteId(null)
    } catch {
      toast.error('Failed to delete notification')
    }
  }

  return (
    <>
      <DashboardHeader fixed />

      <Main className='flex flex-1 flex-col gap-6 p-4 sm:p-6'>
        {/* Page Heading & Actions */}
        <div className='flex flex-wrap items-center justify-between gap-4'>
          <div className='space-y-1'>
            <div className='flex items-center gap-2.5'>
              <div className='flex size-9 items-center justify-center rounded-xl bg-primary/10 text-primary'>
                <Bell className='size-5' />
              </div>
              <h1 className='text-2xl font-black tracking-tight text-foreground'>
                Notifications
              </h1>
            </div>
            <p className='text-xs text-muted-foreground'>
              Monitor real-time system alerts, order updates, stock levels, and customer interactions.
            </p>
          </div>

          <div className='flex items-center gap-2'>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={() => setSettingsOpen(true)}
              className='h-9 gap-1.5 text-xs font-semibold'
            >
              <Settings className='size-3.5 text-muted-foreground' />
              <span>Preferences</span>
            </Button>

            {unreadCount > 0 && (
              <Button
                type='button'
                size='sm'
                onClick={() => markAllAsRead.mutate()}
                disabled={markAllAsRead.isPending}
                className='h-9 gap-1.5 text-xs font-bold'
              >
                <CheckCheck className='size-3.5' />
                <span>Mark All Read</span>
              </Button>
            )}
          </div>
        </div>

        {/* Quick KPI Cards */}
        <div className='grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4'>
          <div className='flex items-center gap-3.5 rounded-xl border border-border/70 bg-card p-4 shadow-2xs'>
            <div className='flex size-10 items-center justify-center rounded-xl bg-primary/10 text-primary'>
              <Bell className='size-5' />
            </div>
            <div>
              <p className='text-xs font-medium text-muted-foreground'>Total Records</p>
              <p className='text-xl font-black text-foreground'>{totalRecords}</p>
            </div>
          </div>

          <div className='flex items-center gap-3.5 rounded-xl border border-border/70 bg-card p-4 shadow-2xs'>
            <div className='flex size-10 items-center justify-center rounded-xl bg-rose-500/10 text-rose-600 dark:text-rose-400'>
              <Sparkles className='size-5' />
            </div>
            <div>
              <p className='text-xs font-medium text-muted-foreground'>Unread Alerts</p>
              <p className='text-xl font-black text-foreground'>{unreadCount}</p>
            </div>
          </div>

          <div className='flex items-center gap-3.5 rounded-xl border border-border/70 bg-card p-4 shadow-2xs'>
            <div className='flex size-10 items-center justify-center rounded-xl bg-blue-500/10 text-blue-600 dark:text-blue-400'>
              <Package className='size-5' />
            </div>
            <div>
              <p className='text-xs font-medium text-muted-foreground'>Order Channel</p>
              <p className='text-xs font-bold text-foreground'>Real-time Active</p>
            </div>
          </div>

          <div className='flex items-center gap-3.5 rounded-xl border border-border/70 bg-card p-4 shadow-2xs'>
            <div className='flex size-10 items-center justify-center rounded-xl bg-amber-500/10 text-amber-600 dark:text-amber-400'>
              <RotateCcw className='size-5' />
            </div>
            <div>
              <p className='text-xs font-medium text-muted-foreground'>Returns & Stock</p>
              <p className='text-xs font-bold text-foreground'>Auto Monitored</p>
            </div>
          </div>
        </div>

        {/* Filter Bar */}
        <NotificationsFilterBar
          search={search}
          onSearchChange={setSearch}
          selectedChannel={selectedChannel}
          onChannelChange={(ch) => {
            setSelectedChannel(ch)
            setPage(1)
          }}
          selectedStatus={selectedStatus}
          onStatusChange={(st) => {
            setSelectedStatus(st)
            setPage(1)
          }}
          selectedCount={selectedIds.length}
          onMarkSelectedAsRead={handleMarkSelectedAsRead}
          onDeleteSelected={() => setBulkDeleteOpen(true)}
          onClearSelection={handleDeselectAll}
          isMarkingRead={markAsRead.isPending}
          isDeleting={deleteMutation.isPending}
          onRefresh={() => refetch()}
          isRefreshing={isFetching}
        />

        {/* Table View */}
        <NotificationsTable
          notifications={filteredItems}
          isLoading={isLoading}
          page={page}
          pageSize={pageSize}
          totalRecords={totalRecords}
          onPageChange={setPage}
          onPageSizeChange={(newSize) => {
            setPageSize(newSize)
            setPage(1)
          }}
          selectedIds={selectedIds}
          onToggleSelect={handleToggleSelect}
          onSelectAll={handleSelectAll}
          onDeselectAll={handleDeselectAll}
          onMarkAsRead={(id) => markAsRead.mutate(id)}
          onDeleteNotification={(id) => setSingleDeleteId(id)}
        />
      </Main>

      {/* Preferences Dialog */}
      <NotificationSettingsDialog
        open={settingsOpen}
        onOpenChange={setSettingsOpen}
      />

      {/* Bulk Delete Confirm */}
      <ConfirmDialog
        open={bulkDeleteOpen}
        onOpenChange={setBulkDeleteOpen}
        title='Delete Selected Notifications'
        desc={`Are you sure you want to permanently delete ${selectedIds.length} notification(s)? This action cannot be undone.`}
        confirmText='Delete'
        destructive
        handleConfirm={handleConfirmBulkDelete}
        isLoading={deleteMutation.isPending}
      />

      {/* Single Delete Confirm */}
      <ConfirmDialog
        open={Boolean(singleDeleteId)}
        onOpenChange={(open) => !open && setSingleDeleteId(null)}
        title='Delete Notification'
        desc='Are you sure you want to permanently delete this notification record?'
        confirmText='Delete'
        destructive
        handleConfirm={handleConfirmSingleDelete}
        isLoading={deleteMutation.isPending}
      />
    </>
  )
}
