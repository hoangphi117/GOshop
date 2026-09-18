import { useState } from 'react'
import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip } from 'recharts'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { formatCurrency } from '@/features/orders/order-utils'
import { useOrderStatusDistribution } from '../hooks/use-dashboard-analytics'

const STATUS_COLORS: Record<number, { hex: string; bg: string; text: string }> = {
  1: { hex: '#f59e0b', bg: 'bg-amber-500/10', text: 'text-amber-600 dark:text-amber-400' }, // Pending
  2: { hex: '#3b82f6', bg: 'bg-blue-500/10', text: 'text-blue-600 dark:text-blue-400' },    // Confirmed
  3: { hex: '#6366f1', bg: 'bg-indigo-500/10', text: 'text-indigo-600 dark:text-indigo-400' },// Shipped
  4: { hex: '#10b981', bg: 'bg-emerald-500/10', text: 'text-emerald-600 dark:text-emerald-400' },// Delivered
  5: { hex: '#f43f5e', bg: 'bg-rose-500/10', text: 'text-rose-600 dark:text-rose-400' },    // Cancelled
  6: { hex: '#f97316', bg: 'bg-orange-500/10', text: 'text-orange-600 dark:text-orange-400' },// Returned
}

export function OrderStatusChart() {
  const [days, setDays] = useState<number>(30)
  const { data, isLoading, isError } = useOrderStatusDistribution({ days })

  const items = data?.items || []
  const totalOrders = data?.total_orders ?? 0

  const chartData = items
    .filter((item) => (item.count ?? 0) > 0)
    .map((item) => ({
      name: item.status_name || `Status ${item.status}`,
      value: item.count ?? 0,
      totalAmount: item.total_amount ?? 0,
      percentage: item.percentage ?? 0,
      status: item.status ?? 0,
      color: STATUS_COLORS[item.status ?? 0]?.hex || '#94a3b8',
    }))

  return (
    <div className='space-y-4'>
      <div className='flex items-center justify-between'>
        <div className='text-xs text-muted-foreground'>
          Distribution across all fulfillment stages
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
        <div className='flex flex-col items-center justify-center space-y-3 py-6'>
          <Skeleton className='h-48 w-48 rounded-full' />
          <Skeleton className='h-4 w-40' />
        </div>
      ) : isError ? (
        <div className='flex h-[260px] items-center justify-center text-sm text-muted-foreground'>
          Failed to load order status distribution.
        </div>
      ) : totalOrders === 0 || chartData.length === 0 ? (
        <div className='flex h-[260px] items-center justify-center text-sm text-muted-foreground'>
          No orders found in the selected period.
        </div>
      ) : (
        <div className='grid grid-cols-1 items-center gap-4 sm:grid-cols-12'>
          {/* Donut Chart */}
          <div className='relative flex h-[240px] items-center justify-center sm:col-span-5'>
            <ResponsiveContainer width='100%' height='100%'>
              <PieChart>
                <Pie
                  data={chartData}
                  cx='50%'
                  cy='50%'
                  innerRadius={55}
                  outerRadius={85}
                  paddingAngle={3}
                  dataKey='value'
                >
                  {chartData.map((entry, index) => (
                    <Cell key={`cell-${index}`} fill={entry.color} stroke='transparent' />
                  ))}
                </Pie>
                <Tooltip
                  content={({ active, payload }) => {
                    if (active && payload && payload.length) {
                      const p = payload[0].payload as {
                        name: string
                        value: number
                        totalAmount: number
                        percentage: number
                        color: string
                      }
                      return (
                        <div className='rounded-lg border bg-popover p-2.5 shadow-md'>
                          <div className='flex items-center gap-1.5'>
                            <span
                              className='h-2.5 w-2.5 rounded-full'
                              style={{ backgroundColor: p.color }}
                            />
                            <p className='text-xs font-semibold text-foreground'>{p.name}</p>
                          </div>
                          <p className='mt-1 text-xs text-foreground'>
                            Orders: <span className='font-bold'>{p.value}</span> ({p.percentage}%)
                          </p>
                          <p className='text-xs text-muted-foreground'>
                            Value: <span className='font-medium text-foreground'>{formatCurrency(p.totalAmount)}</span>
                          </p>
                        </div>
                      )
                    }
                    return null
                  }}
                />
              </PieChart>
            </ResponsiveContainer>
            <div className='pointer-events-none absolute inset-0 flex flex-col items-center justify-center text-center'>
              <span className='text-2xl font-bold tracking-tight text-foreground'>{totalOrders}</span>
              <span className='text-[11px] text-muted-foreground uppercase font-medium'>Orders</span>
            </div>
          </div>

          {/* Breakdown List */}
          <div className='space-y-2.5 sm:col-span-7'>
            {items.map((item) => {
              const statusMeta = STATUS_COLORS[item.status ?? 0] || {
                hex: '#94a3b8',
                bg: 'bg-muted',
                text: 'text-muted-foreground',
              }
              return (
                <div
                  key={item.status}
                  className='flex items-center justify-between rounded-lg border bg-card/50 p-2 text-xs transition-colors hover:bg-accent/40'
                >
                  <div className='flex items-center gap-2 min-w-0'>
                    <span
                      className='h-2.5 w-2.5 shrink-0 rounded-full'
                      style={{ backgroundColor: statusMeta.hex }}
                    />
                    <span className='font-medium text-foreground truncate'>
                      {item.status_name}
                    </span>
                    <Badge variant='outline' className={`text-[10px] px-1.5 py-0 ${statusMeta.bg} ${statusMeta.text}`}>
                      {item.percentage}%
                    </Badge>
                  </div>
                  <div className='text-right shrink-0'>
                    <div className='font-bold text-foreground'>
                      {item.count} <span className='text-[10px] font-normal text-muted-foreground'>orders</span>
                    </div>
                    <div className='text-[10px] text-muted-foreground'>
                      {formatCurrency(item.total_amount)}
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
