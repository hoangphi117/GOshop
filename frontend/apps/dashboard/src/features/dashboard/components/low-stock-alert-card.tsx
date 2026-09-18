import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import { AlertCircle, AlertTriangle, ArrowRight, Package, ShieldCheck } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { formatCurrency } from '@/features/orders/order-utils'
import { useLowStockAlerts } from '../hooks/use-dashboard-analytics'

export function LowStockAlertCard() {
  const [threshold, setThreshold] = useState<number>(5)
  const { data, isLoading, isError } = useLowStockAlerts({ threshold, limit: 6 })

  const items = data?.items || []
  const totalLow = data?.total_low ?? 0

  return (
    <div className='space-y-4'>
      <div className='flex items-center justify-between'>
        <div className='text-xs text-muted-foreground'>
          Items requiring inventory replenishment
        </div>
        <div className='flex items-center space-x-1 rounded-lg bg-muted p-1 text-xs'>
          <button
            type='button'
            onClick={() => setThreshold(5)}
            className={`rounded px-2.5 py-1 font-medium transition-colors ${
              threshold === 5
                ? 'bg-background text-foreground shadow-xs'
                : 'text-muted-foreground hover:text-foreground'
            }`}
          >
            ≤ 5 items
          </button>
          <button
            type='button'
            onClick={() => setThreshold(10)}
            className={`rounded px-2.5 py-1 font-medium transition-colors ${
              threshold === 10
                ? 'bg-background text-foreground shadow-xs'
                : 'text-muted-foreground hover:text-foreground'
            }`}
          >
            ≤ 10 items
          </button>
        </div>
      </div>

      {isLoading ? (
        <div className='space-y-3 py-2'>
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className='flex items-center gap-3 rounded-lg border p-3'>
              <Skeleton className='h-10 w-10 rounded-md' />
              <div className='flex-1 space-y-1.5'>
                <Skeleton className='h-4 w-32' />
                <Skeleton className='h-3 w-20' />
              </div>
              <Skeleton className='h-5 w-16 rounded-full' />
            </div>
          ))}
        </div>
      ) : isError ? (
        <div className='flex h-[200px] items-center justify-center text-sm text-muted-foreground'>
          Failed to load low stock inventory alerts.
        </div>
      ) : items.length === 0 ? (
        <div className='flex h-[200px] flex-col items-center justify-center space-y-2 text-center'>
          <div className='rounded-full bg-emerald-500/10 p-3 text-emerald-600 dark:text-emerald-400'>
            <ShieldCheck className='h-6 w-6' />
          </div>
          <p className='text-sm font-semibold text-foreground'>Healthy Inventory</p>
          <p className='text-xs text-muted-foreground max-w-[240px]'>
            No products are below the warning threshold ({threshold} items).
          </p>
        </div>
      ) : (
        <div className='space-y-2.5'>
          {items.map((prod) => {
            const isCritical = (prod.stock ?? 0) <= 0
            return (
              <div
                key={prod.uuid}
                className={`flex items-center justify-between rounded-xl border p-3 transition-all hover:shadow-xs ${
                  isCritical
                    ? 'border-rose-500/30 bg-rose-500/5 dark:bg-rose-500/10'
                    : 'border-amber-500/30 bg-amber-500/5 dark:bg-amber-500/10'
                }`}
              >
                <div className='flex items-center gap-3 min-w-0'>
                  <div className='relative flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-lg bg-muted border'>
                    {prod.image_url ? (
                      <img
                        src={prod.image_url}
                        alt={prod.name}
                        className='h-full w-full object-cover'
                      />
                    ) : (
                      <Package className='h-5 w-5 text-muted-foreground' />
                    )}
                  </div>
                  <div className='min-w-0 space-y-0.5 truncate'>
                    <p className='truncate text-sm font-semibold text-foreground'>
                      {prod.name}
                    </p>
                    <div className='flex items-center gap-2 text-xs text-muted-foreground'>
                      <Badge variant='outline' className='text-[10px] px-1.5 py-0'>
                        {prod.category_name || 'General'}
                      </Badge>
                      <span>{formatCurrency(prod.price)}</span>
                    </div>
                  </div>
                </div>

                <div className='flex items-center gap-3 shrink-0'>
                  {isCritical ? (
                    <Badge
                      variant='outline'
                      className='bg-rose-500/15 text-rose-600 dark:text-rose-400 border-rose-500/30 text-xs font-semibold gap-1'
                    >
                      <AlertCircle className='h-3 w-3' />
                      Out of stock
                    </Badge>
                  ) : (
                    <Badge
                      variant='outline'
                      className='bg-amber-500/15 text-amber-600 dark:text-amber-400 border-amber-500/30 text-xs font-semibold gap-1'
                    >
                      <AlertTriangle className='h-3 w-3' />
                      {prod.stock} left
                    </Badge>
                  )}
                </div>
              </div>
            )
          })}

          <div className='pt-2'>
            <Button
              variant='outline'
              size='sm'
              className='w-full justify-center gap-1.5 text-xs'
              asChild
            >
              <Link to='/products'>
                <span>View Full Inventory ({totalLow} low stock alerts)</span>
                <ArrowRight className='h-3.5 w-3.5' />
              </Link>
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
