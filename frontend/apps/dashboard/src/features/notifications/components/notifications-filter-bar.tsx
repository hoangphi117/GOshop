import { Check, Filter, RefreshCw, Search, Trash2, X } from 'lucide-react'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

interface NotificationsFilterBarProps {
  search: string
  onSearchChange: (search: string) => void
  selectedChannel: string
  onChannelChange: (channel: string) => void
  selectedStatus: string
  onStatusChange: (status: string) => void
  selectedCount: number
  onMarkSelectedAsRead: () => void
  onDeleteSelected: () => void
  onClearSelection: () => void
  isMarkingRead?: boolean
  isDeleting?: boolean
  onRefresh: () => void
  isRefreshing?: boolean
}

export function NotificationsFilterBar({
  search,
  onSearchChange,
  selectedChannel,
  onChannelChange,
  selectedStatus,
  onStatusChange,
  selectedCount,
  onMarkSelectedAsRead,
  onDeleteSelected,
  onClearSelection,
  isMarkingRead,
  isDeleting,
  onRefresh,
  isRefreshing,
}: NotificationsFilterBarProps) {
  return (
    <div className='space-y-3'>
      {/* Search and Filter Row */}
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <div className='flex flex-1 flex-wrap items-center gap-2.5 sm:flex-nowrap'>
          <div className='relative w-full sm:w-72'>
            <Search className='absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground' />
            <Input
              value={search}
              onChange={(e) => onSearchChange(e.target.value)}
              placeholder='Search notifications...'
              className='h-9 pl-9 text-xs'
            />
            {search && (
              <button
                type='button'
                onClick={() => onSearchChange('')}
                className='absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground'
              >
                <X className='size-3.5' />
              </button>
            )}
          </div>

          <Select value={selectedChannel} onValueChange={onChannelChange}>
            <SelectTrigger className='h-9 w-full text-xs sm:w-44'>
              <div className='flex items-center gap-1.5'>
                <Filter className='size-3.5 text-muted-foreground' />
                <SelectValue placeholder='All Channels' />
              </div>
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='all'>All Channels</SelectItem>
              <SelectItem value='order'>Orders</SelectItem>
              <SelectItem value='return'>Returns & RMA</SelectItem>
              <SelectItem value='stock'>Stock Alerts</SelectItem>
              <SelectItem value='review'>Customer Reviews</SelectItem>
            </SelectContent>
          </Select>

          <Select value={selectedStatus} onValueChange={onStatusChange}>
            <SelectTrigger className='h-9 w-full text-xs sm:w-36'>
              <SelectValue placeholder='All Status' />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='all'>All Status</SelectItem>
              <SelectItem value='unread'>Unread Only</SelectItem>
              <SelectItem value='read'>Read Only</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={onRefresh}
          className='h-9 gap-1.5 text-xs font-semibold'
          title='Refresh notifications list'
        >
          <RefreshCw className={`size-3.5 ${isRefreshing ? 'animate-spin' : ''}`} />
          <span>Refresh</span>
        </Button>
      </div>

      {/* Bulk Action Toolbar */}
      {selectedCount > 0 && (
        <div className='flex flex-wrap items-center justify-between gap-2 rounded-xl border border-primary/20 bg-primary/5 px-4 py-2 text-xs'>
          <div className='flex items-center gap-2'>
            <span className='font-bold text-foreground'>
              {selectedCount} item{selectedCount > 1 ? 's' : ''} selected
            </span>
            <button
              type='button'
              onClick={onClearSelection}
              className='cursor-pointer text-xs text-muted-foreground underline-offset-4 hover:text-foreground hover:underline'
            >
              Deselect all
            </button>
          </div>

          <div className='flex items-center gap-2'>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={onMarkSelectedAsRead}
              disabled={isMarkingRead}
              className='h-8 cursor-pointer gap-1.5 text-xs'
            >
              <Check className='size-3.5 text-primary' />
              <span>Mark as Read</span>
            </Button>

            <Button
              type='button'
              variant='destructive'
              size='sm'
              onClick={onDeleteSelected}
              disabled={isDeleting}
              className='h-8 cursor-pointer gap-1.5 text-xs font-semibold'
            >
              <Trash2 className='size-3.5' />
              <span>Delete</span>
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
