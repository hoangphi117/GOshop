import {
  AlertTriangle,
  ArrowRight,
  CheckCircle2,
  CreditCard,
  PackageCheck,
  ShoppingCart,
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
import { useCartFunnel } from '../hooks/use-dashboard-analytics'

interface CartFunnelCardProps {
  fromDate?: string
  toDate?: string
}

export function CartFunnelCard({ fromDate, toDate }: CartFunnelCardProps = {}) {
  const queryParams = fromDate && toDate ? { from_date: fromDate, to_date: toDate } : {}
  const { data, isLoading, isError } = useCartFunnel(queryParams)

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <Skeleton className='h-6 w-64' />
          <Skeleton className='h-4 w-96' />
        </CardHeader>
        <CardContent className='space-y-6'>
          <div className='grid grid-cols-2 gap-4 sm:grid-cols-4'>
            {Array.from({ length: 4 }).map((_, i) => (
              <Skeleton key={i} className='h-24 w-full rounded-xl' />
            ))}
          </div>
          <Skeleton className='h-36 w-full rounded-xl' />
        </CardContent>
      </Card>
    )
  }

  if (isError || !data) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Cart Abandonment & Conversion Funnel</CardTitle>
          <CardDescription>Failed to load funnel statistics.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className='py-6 text-center text-sm text-destructive'>
            Could not fetch cart funnel data. Please try again.
          </div>
        </CardContent>
      </Card>
    )
  }

  const totalCarts = data.total_carts ?? 0
  const ordersCreated = data.orders_created ?? 0
  const ordersPaid = data.orders_paid ?? 0
  const abandonedCarts = data.abandoned_carts ?? 0
  const abandonmentRate = data.abandonment_rate ?? 0
  const cartToOrderRate = data.cart_to_order_rate ?? 0
  const orderToPaidRate = data.order_to_paid_rate ?? 0
  const potentialLostRevenue = data.potential_lost_revenue ?? 0

  // Calculate drop-off percentages
  const cartDropoff = Math.max(0, Math.round((100 - cartToOrderRate) * 10) / 10)
  const paymentDropoff = Math.max(0, Math.round((100 - orderToPaidRate) * 10) / 10)

  return (
    <Card className='overflow-hidden'>
      <CardHeader className='pb-4'>
        <div className='flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between'>
          <div>
            <CardTitle className='flex items-center gap-2'>
              <ShoppingCart className='h-5 w-5 text-primary' />
              Cart Abandonment & Conversion Funnel
            </CardTitle>
            <CardDescription>
              End-to-end customer drop-off analysis from cart creation to checkout and payment completion
            </CardDescription>
          </div>
          <Badge variant='outline' className='w-fit text-xs font-normal'>
            {fromDate && toDate ? `${fromDate} to ${toDate}` : 'Active Period'}
          </Badge>
        </div>
      </CardHeader>

      <CardContent className='space-y-6'>
        {/* ===== KPI Summary Cards ===== */}
        <div className='grid grid-cols-2 gap-3 sm:grid-cols-4'>
          {/* Abandonment Rate */}
          <div className='rounded-xl border bg-card p-3.5 shadow-2xs'>
            <div className='flex items-center justify-between'>
              <span className='text-xs font-medium text-muted-foreground'>Abandonment Rate</span>
              <TrendingDown className={`h-4 w-4 ${abandonmentRate > 50 ? 'text-rose-500' : 'text-amber-500'}`} />
            </div>
            <div className='mt-2 text-2xl font-bold tracking-tight text-foreground'>
              {abandonmentRate.toFixed(1)}%
            </div>
            <div className='mt-1 text-[11px] text-muted-foreground'>
              {abandonedCarts} abandoned carts
            </div>
          </div>

          {/* Potential Lost Revenue */}
          <div className='rounded-xl border bg-card p-3.5 shadow-2xs'>
            <div className='flex items-center justify-between'>
              <span className='text-xs font-medium text-muted-foreground'>Potential Lost</span>
              <AlertTriangle className='h-4 w-4 text-amber-500' />
            </div>
            <div className='mt-2 text-xl font-bold tracking-tight text-amber-600 dark:text-amber-400'>
              {formatCurrency(potentialLostRevenue)}
            </div>
            <div className='mt-1 text-[11px] text-muted-foreground'>
              In inactive carts (&gt;24h)
            </div>

          </div>

          {/* Cart to Order Conversion */}
          <div className='rounded-xl border bg-card p-3.5 shadow-2xs'>
            <div className='flex items-center justify-between'>
              <span className='text-xs font-medium text-muted-foreground'>Cart-to-Order</span>
              <PackageCheck className='h-4 w-4 text-blue-500' />
            </div>
            <div className='mt-2 text-2xl font-bold tracking-tight text-foreground'>
              {cartToOrderRate.toFixed(1)}%
            </div>
            <div className='mt-1 text-[11px] text-muted-foreground'>
              {ordersCreated} of {totalCarts} carts
            </div>
          </div>

          {/* Order to Paid Conversion */}
          <div className='rounded-xl border bg-card p-3.5 shadow-2xs'>
            <div className='flex items-center justify-between'>
              <span className='text-xs font-medium text-muted-foreground'>Order-to-Paid</span>
              <CheckCircle2 className='h-4 w-4 text-emerald-500' />
            </div>
            <div className='mt-2 text-2xl font-bold tracking-tight text-emerald-600 dark:text-emerald-400'>
              {orderToPaidRate.toFixed(1)}%
            </div>
            <div className='mt-1 text-[11px] text-muted-foreground'>
              {ordersPaid} of {ordersCreated} placed
            </div>
          </div>
        </div>

        {/* ===== Visual Funnel Pipeline ===== */}
        <div className='rounded-xl border bg-muted/30 p-4 sm:p-5'>
          <h4 className='mb-4 text-xs font-semibold uppercase tracking-wider text-muted-foreground'>
            Conversion Funnel Stages
          </h4>

          <div className='grid grid-cols-1 gap-3 md:grid-cols-5 md:items-center'>
            {/* Stage 1: Active Carts */}
            <div className='rounded-lg border bg-card p-3.5 shadow-2xs'>
              <div className='flex items-center gap-2'>
                <div className='flex h-8 w-8 items-center justify-center rounded-lg bg-primary/10 text-primary'>
                  <ShoppingCart className='h-4 w-4' />
                </div>
                <div>
                  <div className='text-xs text-muted-foreground'>Stage 1</div>
                  <div className='text-sm font-semibold text-foreground'>Active Carts</div>
                </div>
              </div>
              <div className='mt-3 flex items-baseline justify-between'>
                <span className='text-xl font-bold text-foreground'>{totalCarts}</span>
                <Badge variant='secondary' className='text-[10px]'>100%</Badge>
              </div>
              <div className='mt-2 h-1.5 w-full rounded-full bg-muted overflow-hidden'>
                <div className='h-full bg-primary rounded-full w-full' />
              </div>
            </div>

            {/* Drop-off connector 1 */}
            <div className='flex flex-col items-center justify-center py-1 text-center md:py-0'>
              <div className='flex items-center gap-1 text-[11px] font-medium text-rose-500'>
                <TrendingDown className='h-3 w-3' />
                <span>{cartDropoff}% drop</span>
              </div>
              <ArrowRight className='hidden h-4 w-4 text-muted-foreground/60 md:block' />
            </div>

            {/* Stage 2: Orders Placed */}
            <div className='rounded-lg border bg-card p-3.5 shadow-2xs'>
              <div className='flex items-center gap-2'>
                <div className='flex h-8 w-8 items-center justify-center rounded-lg bg-blue-500/10 text-blue-500'>
                  <PackageCheck className='h-4 w-4' />
                </div>
                <div>
                  <div className='text-xs text-muted-foreground'>Stage 2</div>
                  <div className='text-sm font-semibold text-foreground'>Orders Placed</div>
                </div>
              </div>
              <div className='mt-3 flex items-baseline justify-between'>
                <span className='text-xl font-bold text-foreground'>{ordersCreated}</span>
                <Badge variant='secondary' className='text-[10px]'>{cartToOrderRate.toFixed(1)}%</Badge>
              </div>
              <div className='mt-2 h-1.5 w-full rounded-full bg-muted overflow-hidden'>
                <div
                  className='h-full bg-blue-500 rounded-full transition-all'
                  style={{ width: `${Math.min(100, Math.max(0, cartToOrderRate))}%` }}
                />
              </div>
            </div>

            {/* Drop-off connector 2 */}
            <div className='flex flex-col items-center justify-center py-1 text-center md:py-0'>
              <div className='flex items-center gap-1 text-[11px] font-medium text-rose-500'>
                <TrendingDown className='h-3 w-3' />
                <span>{paymentDropoff}% drop</span>
              </div>
              <ArrowRight className='hidden h-4 w-4 text-muted-foreground/60 md:block' />
            </div>

            {/* Stage 3: Orders Paid */}
            <div className='rounded-lg border bg-card p-3.5 shadow-2xs'>
              <div className='flex items-center gap-2'>
                <div className='flex h-8 w-8 items-center justify-center rounded-lg bg-emerald-500/10 text-emerald-500'>
                  <CreditCard className='h-4 w-4' />
                </div>
                <div>
                  <div className='text-xs text-muted-foreground'>Stage 3</div>
                  <div className='text-sm font-semibold text-foreground'>Orders Paid</div>
                </div>
              </div>
              <div className='mt-3 flex items-baseline justify-between'>
                <span className='text-xl font-bold text-emerald-600 dark:text-emerald-400'>{ordersPaid}</span>
                <Badge variant='secondary' className='text-[10px] text-emerald-600 dark:text-emerald-400'>
                  {totalCarts > 0 ? ((ordersPaid / totalCarts) * 100).toFixed(1) : 0}% net
                </Badge>
              </div>
              <div className='mt-2 h-1.5 w-full rounded-full bg-muted overflow-hidden'>
                <div
                  className='h-full bg-emerald-500 rounded-full transition-all'
                  style={{
                    width: `${totalCarts > 0 ? Math.min(100, Math.max(0, (ordersPaid / totalCarts) * 100)) : 0}%`,
                  }}
                />
              </div>
            </div>
          </div>
        </div>

        {/* ===== Abandonment Recovery Alert Banner ===== */}
        {abandonedCarts > 0 && (
          <div className='flex items-start gap-3 rounded-xl border border-amber-500/20 bg-amber-500/10 p-4 text-xs text-amber-900 dark:text-amber-200'>
            <AlertTriangle className='h-5 w-5 shrink-0 text-amber-600 dark:text-amber-400' />
            <div className='space-y-1'>
              <div className='font-semibold text-amber-800 dark:text-amber-300'>
                {abandonedCarts} abandoned carts detected with {formatCurrency(potentialLostRevenue)} at risk
              </div>
              <div className='text-amber-700/90 dark:text-amber-300/80'>
                These carts have had no checkout activity for over 24 hours. Automated reminder emails or limited-time promotional incentives could help convert up to 15-20% of these carts into confirmed orders.
              </div>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
