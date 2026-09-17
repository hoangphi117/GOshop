import {
  Clock,
  DollarSign,
  ShoppingBag,
  TrendingDown,
  TrendingUp,
  Users,
} from 'lucide-react'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ConfigDrawer } from '@/components/config-drawer'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { TopNav } from '@/components/layout/top-nav'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { formatCurrency } from '@/features/orders/order-utils'
import { Analytics } from './components/analytics'
import { Overview } from './components/overview'
import { RecentSales } from './components/recent-sales'
import { useAnalyticsOverview } from './hooks/use-dashboard-analytics'

export function Dashboard() {
  const { data: stats, isLoading } = useAnalyticsOverview()

  return (
    <>
      {/* ===== Top Heading ===== */}
      <Header>
        <TopNav links={topNav} className='me-auto' />
        <Search />
        <ThemeSwitch />
        <ConfigDrawer />
        <ProfileDropdown />
      </Header>

      {/* ===== Main ===== */}
      <Main>
        <div className='mb-2 flex items-center justify-between space-y-2'>
          <div>
            <h1 className='text-2xl font-bold tracking-tight'>Analytics Dashboard</h1>
            <p className='text-xs text-muted-foreground'>
              Real-time business performance & revenue metrics (rolling 30 days)
            </p>
          </div>
        </div>

        <Tabs orientation='vertical' defaultValue='overview' className='space-y-4'>
          <div className='w-full overflow-x-auto pb-2'>
            <TabsList>
              <TabsTrigger value='overview'>Overview</TabsTrigger>
              <TabsTrigger value='analytics'>Analytics & Top Products</TabsTrigger>
            </TabsList>
          </div>

          <TabsContent value='overview' className='space-y-4'>
            {/* ===== KPI Summary Cards ===== */}
            <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-4'>
              {/* Total Revenue */}
              <Card className='relative overflow-hidden'>
                <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
                  <CardTitle className='text-sm font-medium'>Total Revenue</CardTitle>
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
                        <span className='text-muted-foreground'>vs previous 30d</span>
                      </div>
                    </>
                  )}
                </CardContent>
              </Card>

              {/* Total Orders */}
              <Card className='relative overflow-hidden'>
                <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
                  <CardTitle className='text-sm font-medium'>Total Orders</CardTitle>
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
                        <span className='text-muted-foreground'>vs previous 30d</span>
                      </div>
                    </>
                  )}
                </CardContent>
              </Card>

              {/* New Customers */}
              <Card className='relative overflow-hidden'>
                <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
                  <CardTitle className='text-sm font-medium'>New Customers</CardTitle>
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
                        <span className='text-muted-foreground'>vs previous 30d</span>
                      </div>
                    </>
                  )}
                </CardContent>
              </Card>

              {/* Pending Orders */}
              <Card className='relative overflow-hidden'>
                <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
                  <CardTitle className='text-sm font-medium'>Pending Orders</CardTitle>
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
                      <p className='mt-1 text-xs text-amber-600 dark:text-amber-400 font-medium'>
                        {(stats?.pending_orders ?? 0) > 0 ? 'Awaiting order fulfillment' : 'All orders processed'}
                      </p>
                    </>
                  )}
                </CardContent>
              </Card>
            </div>

            {/* ===== Charts & Recent Sales Grid ===== */}
            <div className='grid grid-cols-1 gap-4 lg:grid-cols-7'>
              <Card className='col-span-1 lg:col-span-4'>
                <CardHeader>
                  <CardTitle>Revenue Analytics</CardTitle>
                  <CardDescription>
                    Historical sales revenue and order volume distribution
                  </CardDescription>
                </CardHeader>
                <CardContent className='ps-2'>
                  <Overview />
                </CardContent>
              </Card>

              <Card className='col-span-1 lg:col-span-3'>
                <CardHeader>
                  <CardTitle>Recent Orders</CardTitle>
                  <CardDescription>
                    {stats?.total_orders ? `You had ${stats.total_orders} total orders in the last 30 days.` : 'Latest customer transactions.'}
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <RecentSales />
                </CardContent>
              </Card>
            </div>
          </TabsContent>

          <TabsContent value='analytics' className='space-y-4'>
            <Analytics />
          </TabsContent>
        </Tabs>
      </Main>
    </>
  )
}

function renderGrowthBadge(growth: number) {
  const isPositive = growth >= 0
  return (
    <span
      className={`inline-flex items-center gap-0.5 font-semibold ${
        isPositive ? 'text-emerald-600 dark:text-emerald-400' : 'text-rose-600 dark:text-rose-400'
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

const topNav = [
  {
    title: 'Overview',
    href: 'dashboard/overview',
    isActive: true,
    disabled: false,
  },
  {
    title: 'Customers',
    href: 'dashboard/customers',
    isActive: false,
    disabled: true,
  },
  {
    title: 'Products',
    href: 'dashboard/products',
    isActive: false,
    disabled: true,
  },
  {
    title: 'Settings',
    href: 'dashboard/settings',
    isActive: false,
    disabled: true,
  },
]
