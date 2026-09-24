import { Package, TrendingUp } from 'lucide-react'
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
import { useRevenueChart, useTopProducts } from '../hooks/use-dashboard-analytics'
import { AnalyticsChart } from './analytics-chart'
import { CartFunnelCard } from './cart-funnel-card'

interface AnalyticsProps {
  fromDate?: string
  toDate?: string
}

export function Analytics({ fromDate, toDate }: AnalyticsProps = {}) {
  const { data: topProducts, isLoading: isProductsLoading } = useTopProducts(
    fromDate && toDate
      ? { from_date: fromDate, to_date: toDate, limit: 6 }
      : { days: 30, limit: 6 }
  )
  const { data: monthlyChart, isLoading: isMonthlyLoading } = useRevenueChart({ period: 'monthly', months: 12 })

  const totalMonthlyRevenue = (monthlyChart || []).reduce((acc, curr) => acc + (curr.revenue ?? 0), 0)
  const totalMonthlyOrders = (monthlyChart || []).reduce((acc, curr) => acc + (curr.order_count ?? 0), 0)

  return (
    <div className='space-y-4'>
      {/* ===== Cart Abandonment & Conversion Funnel ===== */}
      <CartFunnelCard fromDate={fromDate} toDate={toDate} />

      {/* ===== 12-Month Performance Chart ===== */}
      <Card>

        <CardHeader className='flex flex-row items-center justify-between pb-2'>
          <div>
            <CardTitle>Revenue & Traffic Performance</CardTitle>
            <CardDescription>Monthly overview for the last 12 months</CardDescription>
          </div>
          <div className='flex items-center gap-4 text-right'>
            <div>
              <div className='text-xs text-muted-foreground'>12M Revenue</div>
              <div className='text-base font-bold text-primary'>{formatCurrency(totalMonthlyRevenue)}</div>
            </div>
            <div>
              <div className='text-xs text-muted-foreground'>12M Orders</div>
              <div className='text-base font-bold text-foreground'>{totalMonthlyOrders}</div>
            </div>
          </div>
        </CardHeader>
        <CardContent className='px-6'>
          {isMonthlyLoading ? (
            <Skeleton className='h-[300px] w-full rounded-xl' />
          ) : (
            <AnalyticsChart />
          )}
        </CardContent>
      </Card>

      {/* ===== Top Selling Products Grid ===== */}
      <div className='grid grid-cols-1 gap-4 lg:grid-cols-7'>
        <Card className='col-span-1 lg:col-span-7'>
          <CardHeader>
            <div className='flex items-center justify-between'>
              <div>
                <CardTitle className='flex items-center gap-2'>
                  <TrendingUp className='h-5 w-5 text-primary' />
                  Top Best-Selling Products {fromDate && toDate ? `(${fromDate} - ${toDate})` : '(Last 30 Days)'}
                </CardTitle>
                <CardDescription>
                  Ranked by total quantity sold and revenue generated
                </CardDescription>
              </div>
            </div>
          </CardHeader>
          <CardContent>
            {isProductsLoading ? (
              <div className='grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3'>
                {Array.from({ length: 6 }).map((_, i) => (
                  <div key={i} className='flex items-center gap-3 rounded-lg border p-3'>
                    <Skeleton className='h-12 w-12 rounded-md' />
                    <div className='flex-1 space-y-1.5'>
                      <Skeleton className='h-4 w-32' />
                      <Skeleton className='h-3 w-20' />
                    </div>
                  </div>
                ))}
              </div>
            ) : !topProducts || topProducts.length === 0 ? (
              <div className='py-8 text-center text-sm text-muted-foreground'>
                No products sold in the last 30 days.
              </div>
            ) : (
              <div className='grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3'>
                {topProducts.map((prod, index) => (
                  <div
                    key={prod.uuid}
                    className='group relative flex items-center gap-3.5 rounded-xl border bg-card p-3.5 shadow-xs transition-all hover:border-primary/40 hover:shadow-md'
                  >
                    <div className='relative flex h-14 w-14 shrink-0 items-center justify-center overflow-hidden rounded-lg bg-muted/50 border'>
                      {prod.image_url ? (
                        <img
                          src={prod.image_url}
                          alt={prod.name}
                          className='h-full w-full object-cover transition-transform group-hover:scale-105'
                        />
                      ) : (
                        <Package className='h-6 w-6 text-muted-foreground' />
                      )}
                      <span className='absolute top-0 left-0 rounded-br bg-primary px-1.5 py-0.5 text-[10px] font-bold text-primary-foreground'>
                        #{index + 1}
                      </span>
                    </div>
                    <div className='min-w-0 flex-1 space-y-1'>
                      <p className='truncate text-sm font-semibold text-foreground group-hover:text-primary'>
                        {prod.name}
                      </p>
                      <div className='flex items-center gap-2 text-xs text-muted-foreground'>
                        <Badge variant='secondary' className='text-[10px] px-1.5 py-0 font-normal'>
                          {prod.category_name || 'General'}
                        </Badge>
                        <span>{prod.total_units_sold} sold</span>
                      </div>
                      <p className='text-xs font-bold text-foreground'>
                        {formatCurrency(prod.total_revenue)}
                      </p>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
