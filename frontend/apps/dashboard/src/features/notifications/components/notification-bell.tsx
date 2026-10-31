import { useState } from 'react'
import { Bell } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { cn } from '@/lib/utils'
import { useNotificationSocket } from '../hooks/use-notification-socket'
import { useUnreadNotificationCount } from '../hooks/use-notifications'
import { NotificationDropdown } from './notification-dropdown'

export function NotificationBell() {
  const [open, setOpen] = useState(false)

  // Establish real-time WebSocket connection
  useNotificationSocket()

  // Query unread count
  const { data } = useUnreadNotificationCount()
  const unreadCount = data?.unread_count ?? 0

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant='ghost'
          size='icon'
          className={cn(
            'relative size-9 rounded-full transition-colors cursor-pointer',
            open && 'bg-primary/10 text-primary ring-2 ring-primary/30'
          )}
          aria-label='Open notifications'
        >
          <Bell className='size-4 text-foreground' />

          {unreadCount > 0 ? (
            <span className='absolute -top-1 -right-1 min-w-[18px] h-[18px] px-1 bg-rose-500 text-white text-[10px] font-bold rounded-full flex items-center justify-center shadow-xs ring-2 ring-background animate-in zoom-in-50'>
              {unreadCount > 99 ? '99+' : unreadCount}
            </span>
          ) : null}
        </Button>
      </PopoverTrigger>

      <PopoverContent
        align='end'
        sideOffset={8}
        className='w-[360px] sm:w-[420px] max-w-[calc(100vw-2rem)] p-0 shadow-2xl border border-border/80 rounded-2xl sm:rounded-3xl overflow-hidden bg-card'
      >
        <NotificationDropdown onClose={() => setOpen(false)} />
      </PopoverContent>
    </Popover>
  )
}

