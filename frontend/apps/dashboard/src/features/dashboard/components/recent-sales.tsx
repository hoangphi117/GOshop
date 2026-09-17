import { Link } from '@tanstack/react-router'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { formatCurrency } from '@/features/orders/order-utils'
import { useRecentOrders } from '../hooks/use-dashboard-analytics'

export function RecentSales() {
  const { data: orders, isLoading, isError } = useRecentOrders({ limit: 5 })

  if (isLoading) {
    return (
      <div className='space-y-4'>
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className='flex items-center gap-4'>
            <Skeleton className='h-9 w-9 rounded-full' />
            <div className='flex-1 space-y-1'>
              <Skeleton className='h-4 w-28' />
              <Skeleton className='h-3 w-40' />
            </div>
            <Skeleton className='h-4 w-16' />
          </div>
        ))}
      </div>
    )
  }

  if (isError) {
    return (
      <div className='py-6 text-center text-sm text-muted-foreground'>
        Failed to load recent orders.
      </div>
    )
  }

  if (!orders || orders.length === 0) {
    return (
      <div className='py-6 text-center text-sm text-muted-foreground'>
        No recent orders recorded yet.
      </div>
    )
  }

  return (
    <div className='space-y-4'>
      {orders.map((order) => {
        const initials = getInitials(order.customer_name || 'Customer')
        return (
          <div key={order.uuid} className='flex items-center justify-between gap-3'>
            <div className='flex items-center gap-3 min-w-0'>
              <Avatar className='h-9 w-9 shrink-0'>
                <AvatarFallback className='bg-primary/10 text-xs font-semibold text-primary'>
                  {initials}
                </AvatarFallback>
              </Avatar>
              <div className='space-y-0.5 truncate'>
                <div className='flex items-center gap-2'>
                  <Link
                    to='/orders/$uuid'
                    params={{ uuid: order.uuid || '' }}
                    className='truncate text-sm font-medium hover:underline'
                  >
                    {order.customer_name || 'Anonymous Customer'}
                  </Link>
                  {renderStatusBadge(order.status ?? 0)}
                </div>
                <div className='truncate text-xs text-muted-foreground'>
                  <span className='font-mono'>{order.order_number}</span> · {order.customer_email}
                </div>
              </div>
            </div>
            <div className='text-right shrink-0'>
              <div className='text-sm font-semibold text-foreground'>
                +{formatCurrency(order.total_amount)}
              </div>
              <div className='text-[10px] text-muted-foreground'>
                {order.created_at ? order.created_at.substring(5, 16) : ''}
              </div>
            </div>
          </div>
        )
      })}
    </div>
  )
}

function getInitials(name: string): string {
  const parts = name.trim().split(/\s+/)
  if (parts.length === 1) return parts[0].substring(0, 2).toUpperCase()
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
}

function renderStatusBadge(status: number) {
  switch (status) {
    case 1:
      return (
        <Badge variant='outline' className='bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20 text-[10px] px-1.5 py-0'>
          Pending
        </Badge>
      )
    case 2:
      return (
        <Badge variant='outline' className='bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20 text-[10px] px-1.5 py-0'>
          Confirmed
        </Badge>
      )
    case 3:
      return (
        <Badge variant='outline' className='bg-indigo-500/10 text-indigo-600 dark:text-indigo-400 border-indigo-500/20 text-[10px] px-1.5 py-0'>
          Shipped
        </Badge>
      )
    case 4:
      return (
        <Badge variant='outline' className='bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20 text-[10px] px-1.5 py-0'>
          Delivered
        </Badge>
      )
    case 5:
      return (
        <Badge variant='outline' className='bg-rose-500/10 text-rose-600 dark:text-rose-400 border-rose-500/20 text-[10px] px-1.5 py-0'>
          Cancelled
        </Badge>
      )
    case 6:
      return (
        <Badge variant='outline' className='bg-orange-500/10 text-orange-600 dark:text-orange-400 border-orange-500/20 text-[10px] px-1.5 py-0'>
          Returned
        </Badge>
      )
    default:
      return null
  }
}
