import {
  FolderTree,
  Layers,
  Package,
  ShoppingBag,
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
import { useCategorySales } from '../hooks/use-dashboard-analytics'

interface CategorySalesCardProps {
  fromDate?: string
  toDate?: string
}

const CATEGORY_COLORS = [
  'bg-primary',
  'bg-indigo-500',
  'bg-emerald-500',
  'bg-amber-500',
  'bg-sky-500',
  'bg-rose-500',
  'bg-purple-500',
  'bg-teal-500',
]

export function CategorySalesCard({ fromDate, toDate }: CategorySalesCardProps = {}) {
  const queryParams = fromDate && toDate
    ? { from_date: fromDate, to_date: toDate }
    : { days: 30 }
  const { data, isLoading, isError } = useCategorySales(queryParams)

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
          <CardTitle>Category Sales Breakdown</CardTitle>
          <CardDescription>Failed to load category sales data.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className='py-6 text-center text-sm text-destructive'>
            Could not fetch category sales breakdown. Please try again.
          </div>
        </CardContent>
      </Card>
    )
  }

  const totalRevenue = data.total_revenue ?? 0
  const totalUnitsSold = data.total_units_sold ?? 0
  const totalOrders = data.total_orders ?? 0
  const categories = data.categories || []
  const topCategory = categories.length > 0 ? categories[0] : null

  return (
    <Card className='overflow-hidden'>
      <CardHeader className='pb-4'>
        <div className='flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between'>
          <div>
            <CardTitle className='flex items-center gap-2'>
              <FolderTree className='h-5 w-5 text-primary' />
              Category Sales Breakdown & Revenue Contribution
            </CardTitle>
            <CardDescription>
              Gross revenue share, volume sold, and order distribution across merchandise categories
            </CardDescription>
          </div>
          <Badge variant='outline' className='w-fit text-xs font-normal'>
            {fromDate && toDate ? `${fromDate} to ${toDate}` : 'Last 30 Days'}
          </Badge>
        </div>
      </CardHeader>

      <CardContent className='space-y-6'>
        {/* ===== KPI Summary Cards ===== */}
        <div className='grid grid-cols-2 gap-3 sm:grid-cols-4'>
          {/* Total Revenue */}
          <div className='rounded-xl border bg-card p-3.5 shadow-2xs'>
            <div className='flex items-center justify-between'>
              <span className='text-xs font-medium text-muted-foreground'>Category Revenue</span>
              <TrendingUp className='h-4 w-4 text-primary' />
            </div>
            <div className='mt-2 text-xl font-bold tracking-tight text-primary'>
              {formatCurrency(totalRevenue)}
            </div>
            <div className='mt-1 text-[11px] text-muted-foreground'>
              Across all categories
            </div>
          </div>

          {/* Total Units Sold */}
          <div className='rounded-xl border bg-card p-3.5 shadow-2xs'>
            <div className='flex items-center justify-between'>
              <span className='text-xs font-medium text-muted-foreground'>Units Sold</span>
              <Package className='h-4 w-4 text-indigo-500' />
            </div>
            <div className='mt-2 text-2xl font-bold tracking-tight text-foreground'>
              {totalUnitsSold.toLocaleString()}
            </div>
            <div className='mt-1 text-[11px] text-muted-foreground'>
              Total items purchased
            </div>
          </div>

          {/* Orders Count */}
          <div className='rounded-xl border bg-card p-3.5 shadow-2xs'>
            <div className='flex items-center justify-between'>
              <span className='text-xs font-medium text-muted-foreground'>Orders Placed</span>
              <ShoppingBag className='h-4 w-4 text-emerald-500' />
            </div>
            <div className='mt-2 text-2xl font-bold tracking-tight text-foreground'>
              {totalOrders.toLocaleString()}
            </div>
            <div className='mt-1 text-[11px] text-muted-foreground'>
              Orders containing items
            </div>
          </div>

          {/* Top Category Share */}
          <div className='rounded-xl border bg-card p-3.5 shadow-2xs'>
            <div className='flex items-center justify-between'>
              <span className='text-xs font-medium text-muted-foreground'>Leading Category</span>
              <Layers className='h-4 w-4 text-amber-500' />
            </div>
            <div className='mt-2 truncate text-xl font-bold tracking-tight text-foreground'>
              {topCategory?.category_name || 'N/A'}
            </div>
            <div className='mt-1 text-[11px] text-muted-foreground'>
              {topCategory ? `${(topCategory.percentage ?? 0).toFixed(1)}% of total revenue` : 'No sales recorded'}
            </div>
          </div>
        </div>

        {/* ===== Visual Proportional Revenue Distribution Bar ===== */}
        {categories.length > 0 && (
          <div className='space-y-2 rounded-xl border bg-muted/20 p-4'>
            <div className='flex items-center justify-between text-xs font-medium text-muted-foreground'>
              <span>Revenue Distribution Share</span>
              <span>100% Total Contribution</span>
            </div>
            <div className='flex h-3 w-full overflow-hidden rounded-full bg-muted'>
              {categories.map((cat, idx) => {
                const percentage = Math.min(100, Math.max(0, cat.percentage ?? 0))
                if (percentage <= 0) return null
                const colorClass = CATEGORY_COLORS[idx % CATEGORY_COLORS.length]
                return (
                  <div
                    key={cat.category_id || idx}
                    className={`h-full ${colorClass} transition-all duration-300 first:rounded-l-full last:rounded-r-full`}
                    style={{ width: `${percentage}%` }}
                    title={`${cat.category_name}: ${percentage.toFixed(1)}%`}
                  />
                )
              })}
            </div>
          </div>
        )}

        {/* ===== Category Breakdown Table / Cards ===== */}
        <div className='rounded-xl border bg-card'>
          <div className='border-b p-4'>
            <h4 className='text-sm font-semibold text-foreground'>Revenue & Volume by Category</h4>
            <p className='text-xs text-muted-foreground'>Sorted by highest gross revenue contribution</p>
          </div>

          {categories.length === 0 ? (
            <div className='py-8 text-center text-xs text-muted-foreground'>
              No category sales recorded in this timeframe.
            </div>
          ) : (
            <div className='divide-y divide-border'>
              {categories.map((cat, idx) => {
                const percentage = Math.min(100, Math.max(0, cat.percentage ?? 0))
                const colorClass = CATEGORY_COLORS[idx % CATEGORY_COLORS.length]

                return (
                  <div key={cat.category_id || idx} className='flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between'>
                    <div className='flex items-center gap-3'>
                      <div className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-xs font-bold text-white ${colorClass}`}>
                        #{idx + 1}
                      </div>
                      <div className='space-y-0.5'>
                        <div className='flex items-center gap-2'>
                          <span className='text-sm font-semibold text-foreground'>{cat.category_name}</span>
                          <Badge variant='secondary' className='text-[10px] px-1.5 py-0 font-normal'>
                            {cat.category_slug}
                          </Badge>
                        </div>
                        <div className='flex items-center gap-3 text-xs text-muted-foreground'>
                          <span>{cat.units_sold?.toLocaleString()} units sold</span>
                          <span>•</span>
                          <span>{cat.order_count?.toLocaleString()} orders</span>
                        </div>
                      </div>
                    </div>

                    <div className='flex items-center gap-4 sm:text-right'>
                      <div className='w-28 space-y-1 sm:w-36'>
                        <div className='flex items-center justify-between text-xs sm:justify-end sm:gap-2'>
                          <span className='font-semibold text-foreground'>{percentage.toFixed(1)}%</span>
                          <span className='text-[11px] text-muted-foreground'>share</span>
                        </div>
                        <div className='h-1.5 w-full overflow-hidden rounded-full bg-muted'>
                          <div
                            className={`h-full rounded-full ${colorClass}`}
                            style={{ width: `${percentage}%` }}
                          />
                        </div>
                      </div>

                      <div className='min-w-24 text-right'>
                        <div className='text-sm font-bold text-foreground'>
                          {formatCurrency(cat.gross_revenue ?? 0)}
                        </div>
                        <div className='text-[10px] text-muted-foreground'>Gross Revenue</div>
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
