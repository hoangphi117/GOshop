import {
  BadgeAlert,
  DollarSign,
  Package,
  RotateCcw,
  TrendingDown,
} from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { formatCurrency } from '@/features/orders/order-utils'
import { useReturnAnalytics } from '../hooks/use-dashboard-analytics'

interface ReturnsAnalyticsCardProps {
  fromDate?: string
  toDate?: string
}

export function ReturnsAnalyticsCard({ fromDate, toDate }: ReturnsAnalyticsCardProps = {}) {
  const queryParams = fromDate && toDate
    ? { from_date: fromDate, to_date: toDate }
    : { days: 30 }
  const { data, isLoading, isError } = useReturnAnalytics(queryParams)

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <Skeleton className='h-6 w-64' />
          <Skeleton className='h-4 w-96' />
        </CardHeader>
        <CardContent className='space-y-6'>
          <div className='grid grid-cols-2 gap-3 sm:grid-cols-4'>
            {Array.from({ length: 4 }).map((_, i) => (
              <Skeleton key={i} className='h-24 w-full rounded-xl' />
            ))}
          </div>
          <div className='grid grid-cols-1 gap-4 lg:grid-cols-2'>
            <Skeleton className='h-48 w-full rounded-xl' />
            <Skeleton className='h-48 w-full rounded-xl' />
          </div>
        </CardContent>
      </Card>
    )
  }

  if (isError || !data) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Return & Refund Financial Analytics</CardTitle>
          <CardDescription>Failed to load return analytics data.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className='py-6 text-center text-sm text-destructive'>
            Could not fetch return analytics data. Please try again.
          </div>
        </CardContent>
      </Card>
    )
  }

  const returnRate = data.return_rate ?? 0
  const totalReturns = data.total_returns ?? 0
  const totalRefundedAmount = data.total_refunded_amount ?? 0
  const returnedOrdersCount = data.returned_orders_count ?? 0
  const totalPaidOrders = data.total_paid_orders ?? 0
  const reasonsBreakdown = data.reasons_breakdown || []
  const topReturnedProducts = data.top_returned_products || []

  // Dynamic status styling based on return rate
  const getRateBadge = (rate: number) => {
    if (rate <= 5) {
      return (
        <Badge variant='outline' className='border-emerald-500/40 text-emerald-600 dark:text-emerald-400'>
          Healthy (&le;5%)
        </Badge>
      )
    }
    if (rate <= 15) {
      return (
        <Badge variant='outline' className='border-amber-500/40 text-amber-600 dark:text-amber-400'>
          Moderate (5-15%)
        </Badge>
      )
    }
    return (
      <Badge variant='outline' className='border-rose-500/40 text-rose-600 dark:text-rose-400'>
        Elevated (&gt;15%)
      </Badge>
    )
  }

  return (
    <Card className='overflow-hidden'>
      <CardHeader className='pb-4'>
        <div className='flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between'>
          <div>
            <CardTitle className='flex items-center gap-2'>
              <RotateCcw className='h-5 w-5 text-primary' />
              Return & Refund Financial Analytics
            </CardTitle>
            <CardDescription>
              Financial impact of returns, item defect patterns, and reason breakdowns
            </CardDescription>
          </div>
          <div className='flex items-center gap-2'>
            {getRateBadge(returnRate)}
            <Badge variant='outline' className='text-xs font-normal'>
              {fromDate && toDate ? `${fromDate} to ${toDate}` : 'Last 30 Days'}
            </Badge>
          </div>
        </div>
      </CardHeader>

      <CardContent className='space-y-6'>
        {/* ===== KPI Summary Cards ===== */}
        <div className='grid grid-cols-2 gap-3 sm:grid-cols-4'>
          {/* Return Rate */}
          <div className='rounded-xl border bg-card p-3.5 shadow-2xs'>
            <div className='flex items-center justify-between'>
              <span className='text-xs font-medium text-muted-foreground'>Return Rate</span>
              <TrendingDown className={`h-4 w-4 ${returnRate > 10 ? 'text-rose-500' : 'text-emerald-500'}`} />
            </div>
            <div className='mt-2 text-2xl font-bold tracking-tight text-foreground'>
              {returnRate.toFixed(1)}%
            </div>
            <div className='mt-1 text-[11px] text-muted-foreground'>
              {returnedOrdersCount} of {totalPaidOrders} orders returned
            </div>
          </div>

          {/* Total Refunded Amount */}
          <div className='rounded-xl border bg-card p-3.5 shadow-2xs'>
            <div className='flex items-center justify-between'>
              <span className='text-xs font-medium text-muted-foreground'>Total Refunded</span>
              <DollarSign className='h-4 w-4 text-rose-500' />
            </div>
            <div className='mt-2 text-xl font-bold tracking-tight text-rose-600 dark:text-rose-400'>
              {formatCurrency(totalRefundedAmount)}
            </div>
            <div className='mt-1 text-[11px] text-muted-foreground'>
              Direct revenue loss from refunds
            </div>
          </div>

          {/* Total Return Cases */}
          <div className='rounded-xl border bg-card p-3.5 shadow-2xs'>
            <div className='flex items-center justify-between'>
              <span className='text-xs font-medium text-muted-foreground'>Return Requests</span>
              <RotateCcw className='h-4 w-4 text-blue-500' />
            </div>
            <div className='mt-2 text-2xl font-bold tracking-tight text-foreground'>
              {totalReturns}
            </div>
            <div className='mt-1 text-[11px] text-muted-foreground'>
              Total return cases initiated
            </div>
          </div>

          {/* Returned Orders */}
          <div className='rounded-xl border bg-card p-3.5 shadow-2xs'>
            <div className='flex items-center justify-between'>
              <span className='text-xs font-medium text-muted-foreground'>Returned Orders</span>
              <BadgeAlert className='h-4 w-4 text-amber-500' />
            </div>
            <div className='mt-2 text-2xl font-bold tracking-tight text-foreground'>
              {returnedOrdersCount}
            </div>
            <div className='mt-1 text-[11px] text-muted-foreground'>
              Orders with completed returns
            </div>
          </div>
        </div>

        {/* ===== Breakdown & Top Products Grid ===== */}
        <div className='grid grid-cols-1 gap-4 lg:grid-cols-2'>
          {/* Reasons Breakdown */}
          <div className='rounded-xl border bg-card p-4'>
            <div className='mb-3 flex items-center justify-between'>
              <h4 className='text-sm font-semibold text-foreground'>Return Reasons Breakdown</h4>
              <span className='text-xs text-muted-foreground'>{reasonsBreakdown.length} categories</span>
            </div>

            {reasonsBreakdown.length === 0 ? (
              <div className='py-8 text-center text-xs text-muted-foreground'>
                No return reasons recorded in this timeframe.
              </div>
            ) : (
              <div className='space-y-3'>
                {reasonsBreakdown.map((item) => {
                  const percentage = Math.min(100, Math.max(0, item.percentage ?? 0))
                  const reasonLabel = (item.reason || 'Other')
                    .replace(/_/g, ' ')
                    .replace(/\b\w/g, (char) => char.toUpperCase())

                  return (
                    <div key={item.reason || 'unknown'} className='space-y-1.5'>
                      <div className='flex items-center justify-between text-xs'>
                        <span className='font-medium text-foreground'>{reasonLabel}</span>
                        <div className='flex items-center gap-2'>
                          <span className='text-muted-foreground'>{item.count} cases</span>
                          <span className='font-semibold text-foreground'>{percentage.toFixed(1)}%</span>
                        </div>
                      </div>
                      <div className='h-2 w-full overflow-hidden rounded-full bg-muted'>
                        <div
                          className='h-full rounded-full bg-primary transition-all duration-300'
                          style={{ width: `${percentage}%` }}
                        />
                      </div>
                    </div>
                  )
                })}
              </div>
            )}
          </div>

          {/* Top Returned Products */}
          <div className='rounded-xl border bg-card p-4'>
            <div className='mb-3 flex items-center justify-between'>
              <h4 className='text-sm font-semibold text-foreground'>Top Returned Products</h4>
              <span className='text-xs text-muted-foreground'>Highest return impact</span>
            </div>

            {topReturnedProducts.length === 0 ? (
              <div className='py-8 text-center text-xs text-muted-foreground'>
                No product returns recorded in this timeframe.
              </div>
            ) : (
              <div className='divide-y divide-border'>
                {topReturnedProducts.slice(0, 5).map((prod, index) => (
                  <div key={prod.uuid || index} className='flex items-center gap-3 py-2.5 first:pt-0 last:pb-0'>
                    <div className='relative flex h-11 w-11 shrink-0 items-center justify-center overflow-hidden rounded-lg border bg-muted/40'>
                      {prod.image_url ? (
                        <img
                          src={prod.image_url}
                          alt={prod.name || 'Product'}
                          className='h-full w-full object-cover'
                        />
                      ) : (
                        <Package className='h-5 w-5 text-muted-foreground' />
                      )}
                      <span className='absolute bottom-0 right-0 rounded-tl bg-background/80 px-1 text-[9px] font-bold'>
                        #{index + 1}
                      </span>
                    </div>

                    <div className='min-w-0 flex-1 space-y-0.5'>
                      <p className='truncate text-xs font-semibold text-foreground'>
                        {prod.name || 'Unnamed Product'}
                      </p>
                      <div className='flex items-center gap-2 text-[11px] text-muted-foreground'>
                        <span>{prod.return_case_count ?? 0} requests</span>
                        <span>•</span>
                        <span>{prod.units_returned ?? 0} units</span>
                      </div>
                    </div>

                    <div className='text-right'>
                      <div className='text-xs font-bold text-rose-600 dark:text-rose-400'>
                        {formatCurrency(prod.returned_value ?? 0)}
                      </div>
                      <div className='text-[10px] text-muted-foreground'>Refunded</div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
