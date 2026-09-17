import { useState } from 'react'
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { Skeleton } from '@/components/ui/skeleton'
import { formatCurrency } from '@/features/orders/order-utils'
import { useRevenueChart } from '../hooks/use-dashboard-analytics'

export function Overview() {
  const [range, setRange] = useState<'7d' | '30d' | '12m'>('30d')

  const queryParams = {
    period: range === '12m' ? ('monthly' as const) : ('daily' as const),
    days: range === '7d' ? 7 : range === '30d' ? 30 : undefined,
    months: range === '12m' ? 12 : undefined,
  }

  const { data: chartData, isLoading, isError } = useRevenueChart(queryParams)

  const formattedData = (chartData || []).map((item) => ({
    name: item.date || '',
    revenue: item.revenue ?? 0,
    orders: item.order_count ?? 0,
  }))

  return (
    <div className='space-y-4'>
      <div className='flex items-center justify-between'>
        <div className='text-xs text-muted-foreground'>
          {range === '12m' ? 'Monthly revenue trend' : 'Daily revenue trend'}
        </div>
        <div className='flex items-center space-x-1 rounded-lg bg-muted p-1 text-xs'>
          <button
            type='button'
            onClick={() => setRange('7d')}
            className={`rounded px-2.5 py-1 font-medium transition-colors ${
              range === '7d'
                ? 'bg-background text-foreground shadow-xs'
                : 'text-muted-foreground hover:text-foreground'
            }`}
          >
            7 Days
          </button>
          <button
            type='button'
            onClick={() => setRange('30d')}
            className={`rounded px-2.5 py-1 font-medium transition-colors ${
              range === '30d'
                ? 'bg-background text-foreground shadow-xs'
                : 'text-muted-foreground hover:text-foreground'
            }`}
          >
            30 Days
          </button>
          <button
            type='button'
            onClick={() => setRange('12m')}
            className={`rounded px-2.5 py-1 font-medium transition-colors ${
              range === '12m'
                ? 'bg-background text-foreground shadow-xs'
                : 'text-muted-foreground hover:text-foreground'
            }`}
          >
            12 Months
          </button>
        </div>
      </div>

      {isLoading ? (
        <div className='space-y-2'>
          <Skeleton className='h-[320px] w-full rounded-xl' />
        </div>
      ) : isError ? (
        <div className='flex h-[320px] items-center justify-center text-sm text-muted-foreground'>
          Failed to load revenue chart data.
        </div>
      ) : formattedData.length === 0 ? (
        <div className='flex h-[320px] items-center justify-center text-sm text-muted-foreground'>
          No revenue data recorded for this period.
        </div>
      ) : (
        <ResponsiveContainer width='100%' height={320}>
          <BarChart data={formattedData} margin={{ top: 10, right: 10, left: -10, bottom: 0 }}>
            <CartesianGrid strokeDasharray='3 3' className='stroke-muted/40' vertical={false} />
            <XAxis
              dataKey='name'
              stroke='#888888'
              fontSize={11}
              tickLine={false}
              axisLine={false}
              tickFormatter={(val: string) => {
                if (!val) return ''
                // If format is YYYY-MM-DD, show MM-DD
                if (val.length === 10) return val.substring(5)
                // If format is YYYY-MM, show Month
                return val
              }}
            />
            <YAxis
              stroke='#888888'
              fontSize={11}
              tickLine={false}
              axisLine={false}
              tickFormatter={(val: number) => {
                if (val >= 1_000_000) return `${(val / 1_000_000).toFixed(0)}M`
                if (val >= 1_000) return `${(val / 1_000).toFixed(0)}k`
                return `${val}`
              }}
            />
            <Tooltip
              content={({ active, payload }) => {
                if (active && payload && payload.length) {
                  const p = payload[0].payload as { name: string; revenue: number; orders: number }
                  return (
                    <div className='rounded-lg border bg-popover p-2.5 shadow-md'>
                      <p className='text-xs font-semibold text-foreground'>{p.name}</p>
                      <p className='mt-1 text-xs text-primary font-medium'>
                        Revenue: <span className='font-bold'>{formatCurrency(p.revenue)}</span>
                      </p>
                      <p className='text-xs text-muted-foreground'>
                        Orders: <span className='font-medium text-foreground'>{p.orders}</span>
                      </p>
                    </div>
                  )
                }
                return null
              }}
            />
            <Bar
              dataKey='revenue'
              fill='currentColor'
              radius={[4, 4, 0, 0]}
              className='fill-primary transition-all hover:opacity-90'
            />
          </BarChart>
        </ResponsiveContainer>
      )}
    </div>
  )
}
