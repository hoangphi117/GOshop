import { useMemo, useState } from 'react'
import { endOfDay, format, startOfDay, subDays } from 'date-fns'
import { useQueryClient } from '@tanstack/react-query'
import {
  Clock,
  DollarSign,
  RefreshCw,
  ShoppingBag,
  TrendingDown,
  TrendingUp,
  Users,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { DashboardHeader } from '@/components/layout/dashboard-header'
import { Main } from '@/components/layout/main'
import { formatCurrency } from '@/features/orders/order-utils'
import { DashboardExportMenu } from './components/dashboard-export-menu'
import {
  DateRangePicker,
  type DateRangePickerValue,
} from './components/date-range-picker'
import { LowStockAlertCard } from './components/low-stock-alert-card'
import { OrderStatusChart } from './components/order-status-chart'
import { Overview } from './components/overview'
import { PaymentMethodChart } from './components/payment-method-chart'
import { RecentSales } from './components/recent-sales'
import {
  useAnalyticsOverview,
  useOrderStatusDistribution,
  usePaymentMethodBreakdown,
  useRevenueChart,
  useTopProducts,
} from './hooks/use-dashboard-analytics'

export function Dashboard() {
  const queryClient = useQueryClient()
  const [dateRange, setDateRange] = useState<DateRangePickerValue>(() => {
    const now = new Date()
    return {
      from: startOfDay(subDays(now, 29)),
      to: endOfDay(now),
      preset: 'last30d',
    }
  })

  const queryParams = useMemo(() => {
    if (!dateRange.from || !dateRange.to) return {}
    return {
      from_date: format(dateRange.from, 'yyyy-MM-dd'),
      to_date: format(dateRange.to, 'yyyy-MM-dd'),
    }
  }, [dateRange.from, dateRange.to])

  const dateRangeLabel = useMemo(() => {
    if (dateRange.from && dateRange.to) {
      return `${format(dateRange.from, 'MMM dd, yyyy')} - ${format(dateRange.to, 'MMM dd, yyyy')}`
    }
    return 'Last 30 Days'
  }, [dateRange.from, dateRange.to])

  const {
    data: stats,
    isLoading,
    isFetching,
  } = useAnalyticsOverview(queryParams)
  const { data: revenuePoints } = useRevenueChart(queryParams)
  const { data: topProducts } = useTopProducts({ ...queryParams, limit: 10 })
  const { data: orderStatusData } = useOrderStatusDistribution(queryParams)
  const { data: paymentMethodsData } = usePaymentMethodBreakdown(queryParams)

  const [lastUpdated, setLastUpdated] = useState<string>('Just now')

  const handleRefresh = async () => {
    await queryClient.invalidateQueries({
      predicate: (query) =>
        Array.isArray(query.queryKey) &&
        typeof query.queryKey[0] === 'string' &&
        query.queryKey[0].startsWith('analytics'),
    })
    const now = new Date()
    setLastUpdated(
      now.toLocaleTimeString('en-US', {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
      })
    )
  }

  return (
    <>
      {/* ===== Top Heading ===== */}
      <DashboardHeader fixed={false} />

      {/* ===== Main ===== */}
      <Main>
        <div className='mb-4 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between'>
          <div>
            <h1 className='text-2xl font-bold tracking-tight'>
              Analytics Dashboard
            </h1>
            <p className='text-xs text-muted-foreground'>
              Real-time business performance & revenue metrics ({dateRangeLabel}
              )
            </p>
          </div>
          <div className='flex flex-wrap items-center gap-2'>
            <DateRangePicker value={dateRange} onChange={setDateRange} />
            <DashboardExportMenu
              stats={stats}
              revenuePoints={revenuePoints}
              topProducts={topProducts}
              orderStatus={orderStatusData?.items}
              paymentMethods={paymentMethodsData?.items}
              dateRangeLabel={dateRangeLabel}
            />
            <span className='hidden text-xs text-muted-foreground sm:inline-block'>
              Last synced:{' '}
              <span className='font-medium text-foreground'>{lastUpdated}</span>
            </span>
            <Button
              variant='outline'
              size='sm'
              onClick={handleRefresh}
              disabled={isFetching}
              className='h-8 gap-1.5 text-xs'
            >
              <RefreshCw
                className={`h-3.5 w-3.5 ${isFetching ? 'animate-spin' : ''}`}
              />
              <span>Refresh</span>
            </Button>
          </div>
        </div>

        <div className='space-y-4'>
          {/* ===== KPI Summary Cards ===== */}
          <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-4'>
            {/* Total Revenue */}
            <Card className='relative overflow-hidden'>
              <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
                <CardTitle className='text-sm font-medium'>
                  Total Revenue
                </CardTitle>
                <div className='rounded-full bg-primary/10 p-2 text-primary'>
                  <DollarSign className='h-4 w-4' />
                </div>
              </CardHeader>
              <CardContent>
                {isLoading ? (
                  <div className='space-y-1.5'>
                    <Skeleton className='h-7 w-28' />
                    <Skeleton className='h-3 w-36' />
                  </div>
                ) : (
                  <>
                    <div className='text-2xl font-bold text-foreground'>
                      {formatCurrency(stats?.total_revenue)}
                    </div>
                    <div className='mt-1 flex items-center gap-1.5 text-xs'>
                      {renderGrowthBadge(stats?.revenue_growth ?? 0)}
                      <span className='text-muted-foreground'>
                        vs previous period
                      </span>
                    </div>
                  </>
                )}
              </CardContent>
            </Card>

            {/* Total Orders */}
            <Card className='relative overflow-hidden'>
              <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
                <CardTitle className='text-sm font-medium'>
                  Total Orders
                </CardTitle>
                <div className='rounded-full bg-blue-500/10 p-2 text-blue-600 dark:text-blue-400'>
                  <ShoppingBag className='h-4 w-4' />
                </div>
              </CardHeader>
              <CardContent>
                {isLoading ? (
                  <div className='space-y-1.5'>
                    <Skeleton className='h-7 w-20' />
                    <Skeleton className='h-3 w-36' />
                  </div>
                ) : (
                  <>
                    <div className='text-2xl font-bold text-foreground'>
                      {stats?.total_orders ?? 0}
                    </div>
                    <div className='mt-1 flex items-center gap-1.5 text-xs'>
                      {renderGrowthBadge(stats?.order_growth ?? 0)}
                      <span className='text-muted-foreground'>
                        vs previous period
                      </span>
                    </div>
                  </>
                )}
              </CardContent>
            </Card>

            {/* New Customers */}
            <Card className='relative overflow-hidden'>
              <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
                <CardTitle className='text-sm font-medium'>
                  New Customers
                </CardTitle>
                <div className='rounded-full bg-emerald-500/10 p-2 text-emerald-600 dark:text-emerald-400'>
                  <Users className='h-4 w-4' />
                </div>
              </CardHeader>
              <CardContent>
                {isLoading ? (
                  <div className='space-y-1.5'>
                    <Skeleton className='h-7 w-20' />
                    <Skeleton className='h-3 w-36' />
                  </div>
                ) : (
                  <>
                    <div className='text-2xl font-bold text-foreground'>
                      +{stats?.new_customers ?? 0}
                    </div>
                    <div className='mt-1 flex items-center gap-1.5 text-xs'>
                      {renderGrowthBadge(stats?.customer_growth ?? 0)}
                      <span className='text-muted-foreground'>
                        vs previous period
                      </span>
                    </div>
                  </>
                )}
              </CardContent>
            </Card>

            {/* Pending Orders */}
            <Card className='relative overflow-hidden'>
              <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
                <CardTitle className='text-sm font-medium'>
                  Pending Orders
                </CardTitle>
                <div className='rounded-full bg-amber-500/10 p-2 text-amber-600 dark:text-amber-400'>
                  <Clock className='h-4 w-4' />
                </div>
              </CardHeader>
              <CardContent>
                {isLoading ? (
                  <div className='space-y-1.5'>
                    <Skeleton className='h-7 w-16' />
                    <Skeleton className='h-3 w-28' />
                  </div>
                ) : (
                  <>
                    <div className='text-2xl font-bold text-foreground'>
                      {stats?.pending_orders ?? 0}
                    </div>
                    <p className='mt-1 text-xs font-medium text-amber-600 dark:text-amber-400'>
                      {(stats?.pending_orders ?? 0) > 0
                        ? 'Awaiting order fulfillment'
                        : 'All orders processed'}
                    </p>
                  </>
                )}
              </CardContent>
            </Card>
          </div>

          {/* ===== Row 1: Revenue Trends & Recent Orders ===== */}
          <div className='grid grid-cols-1 gap-4 lg:grid-cols-7'>
            <Card className='col-span-1 lg:col-span-4'>
              <CardHeader>
                <CardTitle>Revenue Analytics</CardTitle>
                <CardDescription>
                  Historical sales revenue and order volume distribution
                </CardDescription>
              </CardHeader>
              <CardContent className='ps-2'>
                <Overview
                  fromDate={queryParams.from_date}
                  toDate={queryParams.to_date}
                />
              </CardContent>
            </Card>

            <Card className='col-span-1 lg:col-span-3'>
              <CardHeader>
                <CardTitle>Recent Orders</CardTitle>
                <CardDescription>
                  {stats?.total_orders
                    ? `You had ${stats.total_orders} total orders in this period.`
                    : 'Latest customer transactions.'}
                </CardDescription>
              </CardHeader>
              <CardContent>
                <RecentSales />
              </CardContent>
            </Card>
          </div>

          {/* ===== Row 2: Order Fulfillment & Payment Gateways ===== */}
          <div className='grid grid-cols-1 gap-4 lg:grid-cols-7'>
            <Card className='col-span-1 lg:col-span-4'>
              <CardHeader>
                <CardTitle>Order Status Distribution</CardTitle>
                <CardDescription>
                  Breakdown of order volumes across pending, shipped, delivered,
                  and cancelled states
                </CardDescription>
              </CardHeader>
              <CardContent>
                <OrderStatusChart
                  fromDate={queryParams.from_date}
                  toDate={queryParams.to_date}
                />
              </CardContent>
            </Card>

            <Card className='col-span-1 lg:col-span-3'>
              <CardHeader>
                <CardTitle>Payment Methods</CardTitle>
                <CardDescription>
                  Revenue contribution across COD, Bank Transfer, and E-Wallets
                </CardDescription>
              </CardHeader>
              <CardContent>
                <PaymentMethodChart
                  fromDate={queryParams.from_date}
                  toDate={queryParams.to_date}
                />
              </CardContent>
            </Card>
          </div>

          {/* ===== Row 3: Inventory Alerts ===== */}
          <div className='grid grid-cols-1 gap-4 lg:grid-cols-7'>
            <Card className='col-span-1 lg:col-span-7'>
              <CardHeader>
                <CardTitle>Inventory Depletion Alerts</CardTitle>
                <CardDescription>
                  Active warehouse alerts for low-stock and out-of-stock items
                </CardDescription>
              </CardHeader>
              <CardContent>
                <LowStockAlertCard />
              </CardContent>
            </Card>
          </div>
        </div>
      </Main>
    </>
  )
}

function renderGrowthBadge(growth: number) {
  const isPositive = growth >= 0
  return (
    <span
      className={`inline-flex items-center gap-0.5 font-semibold ${
        isPositive
          ? 'text-emerald-600 dark:text-emerald-400'
          : 'text-rose-600 dark:text-rose-400'
      }`}
    >
      {isPositive ? (
        <TrendingUp className='h-3.5 w-3.5' />
      ) : (
        <TrendingDown className='h-3.5 w-3.5' />
      )}
      {isPositive ? `+${growth.toFixed(1)}%` : `${growth.toFixed(1)}%`}
    </span>
  )
}
