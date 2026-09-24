import {
  ArrowUpRight,
  Coins,
  Percent,
  Sparkles,
  Ticket,
  TrendingUp,
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
import { useCouponPerformance } from '../hooks/use-dashboard-analytics'

interface CouponPerformanceCardProps {
  fromDate?: string
  toDate?: string
}

export function CouponPerformanceCard({ fromDate, toDate }: CouponPerformanceCardProps = {}) {
  const queryParams = fromDate && toDate
    ? { from_date: fromDate, to_date: toDate, limit: 10 }
    : { days: 30, limit: 10 }
  const { data, isLoading, isError } = useCouponPerformance(queryParams)

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
          <Skeleton className='h-48 w-full rounded-xl' />
        </CardContent>
      </Card>
    )
  }

  if (isError || !data) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Coupon ROI & Campaign Performance</CardTitle>
          <CardDescription>Failed to load promotional campaign statistics.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className='py-6 text-center text-sm text-destructive'>
            Could not fetch coupon performance analytics. Please try again.
          </div>
        </CardContent>
      </Card>
    )
  }

  const totalCouponsUsed = data.total_coupons_used ?? 0
  const totalDiscountGiven = data.total_discount_given ?? 0
  const totalRevenueGenerated = data.total_revenue_generated ?? 0
  const overallRoiMultiplier = data.overall_roi_multiplier ?? 0
  const campaigns = data.campaigns || []

  const getRoiBadge = (roi: number) => {
    if (roi >= 4) {
      return (
        <Badge variant='outline' className='border-emerald-500/40 text-emerald-600 dark:text-emerald-400'>
          Excellent (&ge;4.0x)
        </Badge>
      )
    }
    if (roi >= 1.5) {
      return (
        <Badge variant='outline' className='border-blue-500/40 text-blue-600 dark:text-blue-400'>
          Positive (1.5x - 4.0x)
        </Badge>
      )
    }
    if (roi >= 1.0) {
      return (
        <Badge variant='outline' className='border-amber-500/40 text-amber-600 dark:text-amber-400'>
          Break-even (1.0x - 1.5x)
        </Badge>
      )
    }
    return (
      <Badge variant='outline' className='border-rose-500/40 text-rose-600 dark:text-rose-400'>
        Negative (&lt;1.0x)
      </Badge>
    )
  }

  return (
    <Card className='overflow-hidden'>
      <CardHeader className='pb-4'>
        <div className='flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between'>
          <div>
            <CardTitle className='flex items-center gap-2'>
              <Ticket className='h-5 w-5 text-primary' />
              Coupon ROI & Campaign Performance
            </CardTitle>
            <CardDescription>
              Promotional code effectiveness, revenue generation efficiency, and campaign ROI
            </CardDescription>
          </div>
          <div className='flex items-center gap-2'>
            {getRoiBadge(overallRoiMultiplier)}
            <Badge variant='outline' className='text-xs font-normal'>
              {fromDate && toDate ? `${fromDate} to ${toDate}` : 'Last 30 Days'}
            </Badge>
          </div>
        </div>
      </CardHeader>

      <CardContent className='space-y-6'>
        {/* ===== KPI Summary Cards ===== */}
        <div className='grid grid-cols-2 gap-3 sm:grid-cols-4'>
          {/* Overall ROI Multiplier */}
          <div className='rounded-xl border bg-card p-3.5 shadow-2xs'>
            <div className='flex items-center justify-between'>
              <span className='text-xs font-medium text-muted-foreground'>Return on Discount (ROI)</span>
              <Sparkles className='h-4 w-4 text-emerald-500' />
            </div>
            <div className='mt-2 flex items-baseline gap-1 text-2xl font-bold tracking-tight text-emerald-600 dark:text-emerald-400'>
              {overallRoiMultiplier.toFixed(2)}x
            </div>
            <div className='mt-1 text-[11px] text-muted-foreground'>
              Revenue generated per $1 discount
            </div>
          </div>

          {/* Revenue Generated */}
          <div className='rounded-xl border bg-card p-3.5 shadow-2xs'>
            <div className='flex items-center justify-between'>
              <span className='text-xs font-medium text-muted-foreground'>Attributed Revenue</span>
              <TrendingUp className='h-4 w-4 text-primary' />
            </div>
            <div className='mt-2 text-xl font-bold tracking-tight text-foreground'>
              {formatCurrency(totalRevenueGenerated)}
            </div>
            <div className='mt-1 text-[11px] text-muted-foreground'>
              From orders using promo codes
            </div>
          </div>

          {/* Total Discounts Given */}
          <div className='rounded-xl border bg-card p-3.5 shadow-2xs'>
            <div className='flex items-center justify-between'>
              <span className='text-xs font-medium text-muted-foreground'>Total Discounts</span>
              <Coins className='h-4 w-4 text-amber-500' />
            </div>
            <div className='mt-2 text-xl font-bold tracking-tight text-amber-600 dark:text-amber-400'>
              {formatCurrency(totalDiscountGiven)}
            </div>
            <div className='mt-1 text-[11px] text-muted-foreground'>
              Direct margin investment
            </div>
          </div>

          {/* Total Usages */}
          <div className='rounded-xl border bg-card p-3.5 shadow-2xs'>
            <div className='flex items-center justify-between'>
              <span className='text-xs font-medium text-muted-foreground'>Coupons Redeemed</span>
              <Ticket className='h-4 w-4 text-blue-500' />
            </div>
            <div className='mt-2 text-2xl font-bold tracking-tight text-foreground'>
              {totalCouponsUsed.toLocaleString()}
            </div>
            <div className='mt-1 text-[11px] text-muted-foreground'>
              Across {campaigns.length} campaigns
            </div>
          </div>
        </div>

        {/* ===== Campaign Performance Breakdown Table ===== */}
        <div className='rounded-xl border bg-card'>
          <div className='border-b p-4'>
            <h4 className='text-sm font-semibold text-foreground'>Campaign Efficiency Rankings</h4>
            <p className='text-xs text-muted-foreground'>Ranked by total revenue generated and discount multiplier</p>
          </div>

          {campaigns.length === 0 ? (
            <div className='py-8 text-center text-xs text-muted-foreground'>
              No coupon campaigns redeemed in this timeframe.
            </div>
          ) : (
            <div className='divide-y divide-border'>
              {campaigns.map((camp, idx) => {
                const roi = camp.roi_multiplier ?? 0

                return (
                  <div key={camp.coupon_uuid || camp.coupon_code || idx} className='flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between'>
                    <div className='flex items-center gap-3'>
                      <div className='flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border bg-muted/40 font-mono text-xs font-bold text-foreground'>
                        #{idx + 1}
                      </div>

                      <div className='space-y-0.5'>
                        <div className='flex items-center gap-2'>
                          <span className='font-mono text-sm font-bold text-primary tracking-wide'>
                            {camp.coupon_code}
                          </span>
                          <span className='text-xs text-muted-foreground'>•</span>
                          <span className='text-xs font-medium text-foreground truncate max-w-[200px]'>
                            {camp.coupon_name || 'Campaign'}
                          </span>
                        </div>
                        <div className='flex items-center gap-2 text-xs text-muted-foreground'>
                          <Badge variant='secondary' className='text-[10px] px-1.5 py-0 font-normal'>
                            {camp.coupon_type === 1 ? (
                              <span className='flex items-center gap-1'>
                                <Percent className='h-2.5 w-2.5' /> {camp.discount_value}% OFF
                              </span>
                            ) : (
                              <span>{formatCurrency(camp.discount_value ?? 0)} OFF</span>
                            )}
                          </Badge>
                          <span>{camp.usage_count?.toLocaleString()} uses</span>
                        </div>
                      </div>
                    </div>

                    <div className='flex items-center gap-5 sm:text-right'>
                      <div>
                        <div className='text-xs font-semibold text-amber-600 dark:text-amber-400'>
                          {formatCurrency(camp.total_discount ?? 0)}
                        </div>
                        <div className='text-[10px] text-muted-foreground'>Discount Given</div>
                      </div>

                      <div>
                        <div className='text-xs font-bold text-foreground'>
                          {formatCurrency(camp.total_revenue ?? 0)}
                        </div>
                        <div className='text-[10px] text-muted-foreground'>Revenue</div>
                      </div>

                      <div className='min-w-20 text-right'>
                        <div className='flex items-center justify-end gap-1'>
                          <span className={`text-sm font-bold ${roi >= 3 ? 'text-emerald-600 dark:text-emerald-400' : roi >= 1.5 ? 'text-blue-600 dark:text-blue-400' : 'text-amber-600 dark:text-amber-400'}`}>
                            {roi.toFixed(1)}x
                          </span>
                          <ArrowUpRight className='h-3.5 w-3.5 text-muted-foreground' />
                        </div>
                        <div className='text-[10px] text-muted-foreground'>ROI Multiplier</div>
                      </div>
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
