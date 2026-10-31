import { Badge } from '@/components/ui/badge'

/** Format a number as VND currency */
export function formatCurrency(val?: number) {
  return new Intl.NumberFormat('vi-VN', {
    style: 'currency',
    currency: 'VND',
  }).format(val ?? 0)
}

/** Format a date string to Asia/Ho_Chi_Minh timezone */
export function formatDateTime(dateStr?: string) {
  if (!dateStr) return '—'
  try {
    if (/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/.test(dateStr)) return dateStr
    const d = new Date(dateStr)
    if (isNaN(d.getTime())) return dateStr
    return new Intl.DateTimeFormat('vi-VN', {
      timeZone: 'Asia/Ho_Chi_Minh',
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    }).format(d)
  } catch {
    return dateStr
  }
}

/** Render a colored Badge for order status */
export function renderOrderStatusBadge(status?: string) {
  const s = status?.toLowerCase()
  if (s === 'delivered')
    return (
      <Badge
        variant='outline'
        className='bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
      >
        Delivered
      </Badge>
    )
  if (s === 'shipped')
    return (
      <Badge
        variant='outline'
        className='bg-indigo-500/10 text-indigo-600 dark:text-indigo-400 border-indigo-500/20'
      >
        Shipped
      </Badge>
    )
  if (s === 'confirmed')
    return (
      <Badge
        variant='outline'
        className='bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20'
      >
        Confirmed
      </Badge>
    )
  if (s === 'pending')
    return (
      <Badge
        variant='outline'
        className='bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20'
      >
        Pending
      </Badge>
    )
  if (s === 'cancelled') return <Badge variant='destructive'>Cancelled</Badge>
  if (s === 'returned')
    return (
      <Badge
        variant='outline'
        className='bg-rose-500/10 text-rose-600 dark:text-rose-400 border-rose-500/20'
      >
        Returned
      </Badge>
    )
  return <Badge variant='secondary'>{status || 'Unknown'}</Badge>
}

/** Render a colored Badge for payment status */
export function renderPaymentStatusBadge(status?: string) {
  const ps = status?.toLowerCase()
  if (ps === 'paid')
    return (
      <Badge
        variant='outline'
        className='bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
      >
        Paid
      </Badge>
    )
  if (ps === 'unpaid')
    return (
      <Badge
        variant='outline'
        className='bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20'
      >
        Unpaid
      </Badge>
    )
  if (ps === 'refunded' || ps === 'partially refunded')
    return (
      <Badge
        variant='outline'
        className='bg-destructive/10 text-destructive border-destructive/20'
      >
        {status}
      </Badge>
    )
  return <Badge variant='secondary'>{status || 'Unknown'}</Badge>
}
