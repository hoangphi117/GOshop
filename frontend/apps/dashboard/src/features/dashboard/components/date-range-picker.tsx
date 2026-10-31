import * as React from 'react'
import {
  endOfDay,
  endOfMonth,
  format,
  startOfDay,
  startOfMonth,
  subDays,
  subMonths,
} from 'date-fns'
import { Calendar as CalendarIcon } from 'lucide-react'
import type { DateRange } from 'react-day-picker'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Calendar } from '@/components/ui/calendar'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'

export interface DateRangePickerValue {
  from?: Date
  to?: Date
  preset?: string
}

interface DateRangePickerProps {
  value: DateRangePickerValue
  onChange: (value: DateRangePickerValue) => void
  className?: string
}

export const PRESETS = [
  {
    key: 'today',
    label: 'Today',
    getRange: () => {
      const now = new Date()
      return { from: startOfDay(now), to: endOfDay(now) }
    },
  },
  {
    key: 'yesterday',
    label: 'Yesterday',
    getRange: () => {
      const y = subDays(new Date(), 1)
      return { from: startOfDay(y), to: endOfDay(y) }
    },
  },
  {
    key: 'last7d',
    label: 'Last 7 Days',
    getRange: () => {
      const now = new Date()
      return { from: startOfDay(subDays(now, 6)), to: endOfDay(now) }
    },
  },
  {
    key: 'last30d',
    label: 'Last 30 Days',
    getRange: () => {
      const now = new Date()
      return { from: startOfDay(subDays(now, 29)), to: endOfDay(now) }
    },
  },
  {
    key: 'thisMonth',
    label: 'This Month',
    getRange: () => {
      const now = new Date()
      return { from: startOfMonth(now), to: endOfDay(now) }
    },
  },
  {
    key: 'lastMonth',
    label: 'Last Month',
    getRange: () => {
      const prev = subMonths(new Date(), 1)
      return { from: startOfMonth(prev), to: endOfMonth(prev) }
    },
  },
] as const

export function DateRangePicker({
  value,
  onChange,
  className,
}: DateRangePickerProps) {
  const [open, setOpen] = React.useState(false)

  const handleSelectRange = (range: DateRange | undefined) => {
    if (!range) {
      onChange({ from: undefined, to: undefined, preset: 'custom' })
      return
    }
    onChange({
      from: range.from ? startOfDay(range.from) : undefined,
      to: range.to ? endOfDay(range.to) : undefined,
      preset: 'custom',
    })
  }

  const handleSelectPreset = (key: string) => {
    const found = PRESETS.find((p) => p.key === key)
    if (found) {
      const { from, to } = found.getRange()
      onChange({ from, to, preset: key })
      setOpen(false)
    }
  }

  const selectedRange: DateRange | undefined = React.useMemo(() => {
    if (!value.from) return undefined
    return {
      from: value.from,
      to: value.to,
    }
  }, [value.from, value.to])

  const label = React.useMemo(() => {
    if (value.from && value.to) {
      return `${format(value.from, 'MMM dd, yyyy')} - ${format(value.to, 'MMM dd, yyyy')}`
    }
    if (value.from) {
      return `${format(value.from, 'MMM dd, yyyy')} - ...`
    }
    return 'Select date range'
  }, [value.from, value.to])

  return (
    <div className={cn('grid gap-2', className)}>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <Button
            id='dashboard-date-range'
            variant='outline'
            size='sm'
            className={cn(
              'h-8 justify-start text-left font-normal text-xs gap-2',
              !value.from && 'text-muted-foreground'
            )}
          >
            <CalendarIcon className='h-3.5 w-3.5 text-muted-foreground' />
            <span>{label}</span>
          </Button>
        </PopoverTrigger>
        <PopoverContent className='w-auto p-0' align='end'>
          <div className='flex flex-col sm:flex-row'>
            {/* Quick Presets */}
            <div className='flex flex-col gap-1 border-b sm:border-b-0 sm:border-r p-3 sm:w-36'>
              <span className='mb-1 px-2 text-[10px] font-semibold tracking-wider text-muted-foreground uppercase'>
                Presets
              </span>
              {PRESETS.map((preset) => (
                <Button
                  key={preset.key}
                  variant={value.preset === preset.key ? 'secondary' : 'ghost'}
                  size='sm'
                  onClick={() => handleSelectPreset(preset.key)}
                  className='h-7 justify-start px-2 text-xs font-normal'
                >
                  {preset.label}
                </Button>
              ))}
            </div>

            {/* Custom DayPicker Calendar */}
            <div className='p-2'>
              <Calendar
                mode='range'
                defaultMonth={value.from}
                selected={selectedRange}
                onSelect={handleSelectRange}
                numberOfMonths={2}
              />
            </div>
          </div>
        </PopoverContent>
      </Popover>
    </div>
  )
}
