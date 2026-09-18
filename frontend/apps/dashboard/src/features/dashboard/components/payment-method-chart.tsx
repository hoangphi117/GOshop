import { useState } from 'react'
import { CreditCard, DollarSign, Wallet } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { formatCurrency } from '@/features/orders/order-utils'
import { usePaymentMethodBreakdown } from '../hooks/use-dashboard-analytics'

const METHOD_ICONS: Record<number, React.ReactNode> = {
  1: <DollarSign className='h-4 w-4 text-amber-500' />,
  2: <CreditCard className='h-4 w-4 text-blue-500' />,
  3: <Wallet className='h-4 w-4 text-purple-500' />,
}

const METHOD_COLORS: Record<number, string> = {
  1: 'bg-amber-500',
  2: 'bg-blue-500',
  3: 'bg-purple-500',
}

export function PaymentMethodChart() {
  const [days, setDays] = useState<number>(30)
  const { data, isLoading, isError } = usePaymentMethodBreakdown({ days })

  const items = data?.items || []
  const totalRevenue = data?.total_revenue ?? 0
  const totalOrders = data?.total_orders ?? 0

  return (
    <div className='space-y-4'>
      <div className='flex items-center justify-between'>
        <div className='text-xs text-muted-foreground'>
          Net revenue & order volume by payment gateway
        </div>
        <div className='flex items-center space-x-1 rounded-lg bg-muted p-1 text-xs'>
          <button
            type='button'
            onClick={() => setDays(7)}
            className={`rounded px-2.5 py-1 font-medium transition-colors ${
              days === 7
                ? 'bg-background text-foreground shadow-xs'
                : 'text-muted-foreground hover:text-foreground'
            }`}
          >
            7 Days
          </button>
          <button
            type='button'
            onClick={() => setDays(30)}
            className={`rounded px-2.5 py-1 font-medium transition-colors ${
              days === 30
                ? 'bg-background text-foreground shadow-xs'
                : 'text-muted-foreground hover:text-foreground'
            }`}
          >
            30 Days
          </button>
          <button
            type='button'
            onClick={() => setDays(90)}
            className={`rounded px-2.5 py-1 font-medium transition-colors ${
              days === 90
                ? 'bg-background text-foreground shadow-xs'
                : 'text-muted-foreground hover:text-foreground'
            }`}
          >
            90 Days
          </button>
        </div>
      </div>

      {isLoading ? (
        <div className='space-y-3 py-4'>
          <Skeleton className='h-16 w-full rounded-xl' />
          <Skeleton className='h-16 w-full rounded-xl' />
          <Skeleton className='h-16 w-full rounded-xl' />
        </div>
      ) : isError ? (
        <div className='flex h-[220px] items-center justify-center text-sm text-muted-foreground'>
          Failed to load payment method metrics.
        </div>
      ) : items.length === 0 ? (
        <div className='flex h-[220px] items-center justify-center text-sm text-muted-foreground'>
          No payment transactions found in the selected period.
        </div>
      ) : (
        <div className='space-y-3.5'>
          {/* Top Summary Banner */}
          <div className='flex items-center justify-between rounded-xl border bg-muted/40 px-4 py-2.5 text-xs'>
            <div>
              <span className='text-muted-foreground'>Total Paid Revenue:</span>{' '}
              <span className='font-bold text-foreground'>{formatCurrency(totalRevenue)}</span>
            </div>
            <div>
              <span className='text-muted-foreground'>Total Paid Orders:</span>{' '}
              <span className='font-bold text-foreground'>{totalOrders}</span>
            </div>
          </div>

          {/* Channels list */}
          <div className='space-y-3'>
            {items.map((item) => {
              const icon = METHOD_ICONS[item.payment_method ?? 0] || <CreditCard className='h-4 w-4 text-primary' />
              const colorClass = METHOD_COLORS[item.payment_method ?? 0] || 'bg-primary'

              return (
                <div
                  key={item.payment_method}
                  className='rounded-xl border bg-card p-3.5 shadow-xs transition-all hover:border-primary/30'
                >
                  <div className='flex items-center justify-between'>
                    <div className='flex items-center gap-2.5'>
                      <div className='rounded-lg bg-muted p-1.5'>{icon}</div>
                      <div>
                        <div className='flex items-center gap-2'>
                          <span className='text-sm font-semibold text-foreground'>
                            {item.method_name}
                          </span>
                          <Badge variant='secondary' className='text-[10px] px-1.5 py-0'>
                            {item.order_count} orders
                          </Badge>
                        </div>
                        <div className='text-xs text-muted-foreground'>
                          Revenue share: <span className='font-semibold text-foreground'>{item.percentage}%</span>
                        </div>
                      </div>
                    </div>
                    <div className='text-right'>
                      <div className='text-sm font-bold text-foreground'>
                        {formatCurrency(item.net_revenue)}
                      </div>
                    </div>
                  </div>

                  {/* Visual progress bar */}
                  <div className='mt-2.5'>
                    <div className='h-2 w-full overflow-hidden rounded-full bg-muted'>
                      <div
                        className={`h-full transition-all ${colorClass}`}
                        style={{ width: `${Math.max(item.percentage ?? 0, 2)}%` }}
                      />
                    </div>
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}
