import {
  AlertTriangle,
  Bell,
  Check,
  Laptop,
  MessageSquare,
  Package,
  RotateCcw,
  RotateCcw as ResetIcon,
  Volume2,
  VolumeX,
} from 'lucide-react'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Switch } from '@/components/ui/switch'
import { Button } from '@/components/ui/button'
import { useNotificationSettingsStore } from '../stores/notification-settings-store'

interface NotificationSettingsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

const CHANNELS = [
  {
    id: 'order',
    label: 'Orders',
    description: 'New orders and status transitions',
    icon: Package,
    color: 'text-blue-500',
  },
  {
    id: 'return',
    label: 'Returns & Refunds',
    description: 'Return requests and RMA updates',
    icon: RotateCcw,
    color: 'text-amber-500',
  },
  {
    id: 'stock',
    label: 'Inventory & Stock',
    description: 'Low stock thresholds and replenishment alerts',
    icon: AlertTriangle,
    color: 'text-rose-500',
  },
  {
    id: 'review',
    label: 'Customer Reviews',
    description: 'New ratings and product feedback',
    icon: MessageSquare,
    color: 'text-cyan-500',
  },
]

export function NotificationSettingsDialog({
  open,
  onOpenChange,
}: NotificationSettingsDialogProps) {
  const {
    soundEnabled,
    desktopPushEnabled,
    mutedTypes,
    setSoundEnabled,
    setDesktopPushEnabled,
    toggleMutedType,
    resetSettings,
  } = useNotificationSettingsStore()

  const handlePushToggle = async (checked: boolean) => {
    if (checked && 'Notification' in window) {
      if (Notification.permission !== 'granted') {
        const res = await Notification.requestPermission()
        if (res !== 'granted') {
          setDesktopPushEnabled(false)
          return
        }
      }
    }
    setDesktopPushEnabled(checked)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-w-md sm:max-w-lg'>
        <DialogHeader>
          <div className='flex items-center gap-2'>
            <div className='flex size-8 items-center justify-center rounded-lg bg-primary/10 text-primary'>
              <Bell className='size-4' />
            </div>
            <div>
              <DialogTitle className='text-base font-bold'>
                Notification Preferences
              </DialogTitle>
              <DialogDescription className='text-xs text-muted-foreground'>
                Customize sound alerts, browser notifications, and muted channels.
              </DialogDescription>
            </div>
          </div>
        </DialogHeader>

        <div className='space-y-4 py-2 text-sm'>
          {/* Audio Chime Setting */}
          <div className='flex items-center justify-between rounded-xl border border-border/70 bg-card p-3.5 shadow-2xs'>
            <div className='flex items-start gap-3'>
              <div className='mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-lg bg-muted text-foreground'>
                {soundEnabled ? (
                  <Volume2 className='size-4 text-primary' />
                ) : (
                  <VolumeX className='size-4 text-muted-foreground' />
                )}
              </div>
              <div className='space-y-0.5'>
                <p className='font-semibold text-foreground text-xs'>
                  Chime Sound Effect
                </p>
                <p className='text-[11px] text-muted-foreground'>
                  Play an audio chime whenever a real-time notification arrives.
                </p>
              </div>
            </div>
            <Switch
              checked={soundEnabled}
              onCheckedChange={setSoundEnabled}
              aria-label='Toggle sound effect'
            />
          </div>

          {/* Desktop Push Setting */}
          <div className='flex items-center justify-between rounded-xl border border-border/70 bg-card p-3.5 shadow-2xs'>
            <div className='flex items-start gap-3'>
              <div className='mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-lg bg-muted text-foreground'>
                <Laptop className='size-4 text-primary' />
              </div>
              <div className='space-y-0.5'>
                <p className='font-semibold text-foreground text-xs'>
                  Desktop Push Notifications
                </p>
                <p className='text-[11px] text-muted-foreground'>
                  Display OS native banners when the dashboard tab is in the background.
                </p>
              </div>
            </div>
            <Switch
              checked={desktopPushEnabled}
              onCheckedChange={handlePushToggle}
              aria-label='Toggle desktop push'
            />
          </div>

          {/* Channel Filters */}
          <div className='space-y-2 pt-1'>
            <h4 className='text-xs font-bold text-foreground tracking-tight'>
              Active Notification Channels
            </h4>
            <p className='text-[11px] text-muted-foreground'>
              Turn off channels to mute sound and banner toasts for specific activity types.
            </p>

            <div className='divide-y divide-border/50 rounded-xl border border-border/70 bg-card'>
              {CHANNELS.map((channel) => {
                const Icon = channel.icon
                const isMuted = mutedTypes.includes(channel.id)
                const isEnabled = !isMuted

                return (
                  <div
                    key={channel.id}
                    className='flex items-center justify-between p-3 transition-colors hover:bg-muted/30'
                  >
                    <div className='flex items-center gap-3'>
                      <div className='flex size-7 shrink-0 items-center justify-center rounded-md bg-muted/60'>
                        <Icon className={`size-3.5 ${channel.color}`} />
                      </div>
                      <div>
                        <p className='text-xs font-semibold text-foreground'>
                          {channel.label}
                        </p>
                        <p className='text-[10px] text-muted-foreground'>
                          {channel.description}
                        </p>
                      </div>
                    </div>
                    <Switch
                      checked={isEnabled}
                      onCheckedChange={() => toggleMutedType(channel.id)}
                      aria-label={`Toggle ${channel.label}`}
                    />
                  </div>
                )
              })}
            </div>
          </div>
        </div>

        <DialogFooter className='flex items-center justify-between gap-2 pt-2'>
          <Button
            type='button'
            variant='ghost'
            size='sm'
            onClick={resetSettings}
            className='h-8 cursor-pointer gap-1.5 text-xs text-muted-foreground hover:text-foreground'
          >
            <ResetIcon className='size-3' />
            <span>Reset Defaults</span>
          </Button>

          <Button
            type='button'
            size='sm'
            onClick={() => onOpenChange(false)}
            className='h-8 cursor-pointer text-xs font-semibold'
          >
            <Check className='size-3.5' />
            <span>Done</span>
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
