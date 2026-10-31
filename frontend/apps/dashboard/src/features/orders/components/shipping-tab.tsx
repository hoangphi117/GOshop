import { useEffect, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import {
  Truck,
  RefreshCw,
  Copy,
  Check,
  Calendar,
  AlertTriangle,
  Package,
  Loader2,
} from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useOrderTimeline, useShippingMutations } from '../hooks/use-shipping'
import { ShippingTimeline } from './shipping-timeline'
import { CreateShippingForm } from './create-shipping-form'
import { PrintBillDialog } from './print-bill-dialog'
import { CancelShippingDialog } from './cancel-shipping-dialog'
import { formatDateTime } from '../order-utils'
import type { Order } from '../types'

interface ShippingTabProps {
  order: Order
}

export function ShippingTab({ order }: ShippingTabProps) {
  const queryClient = useQueryClient()
  const [copied, setCopied] = useState(false)
  const [isDispatched, setIsDispatched] = useState(false)

  const { data: timelineData, isLoading: isLoadingTimeline } = useOrderTimeline(
    order.uuid
  )
  const { syncTracking } = useShippingMutations()

  const isShippingOrDelivered =
    order.status?.toLowerCase() === 'shipped' ||
    order.status?.toLowerCase() === 'delivered' ||
    order.status?.toLowerCase() === 'in_transit' ||
    order.status?.toLowerCase() === 'intransit'

  const hasShippingCode = Boolean(
    order.shipping_code && order.shipping_code.trim() !== ''
  )

  const hasActiveShipment =
    hasShippingCode || isShippingOrDelivered || isDispatched

  const isCancelled = order.status?.toLowerCase() === 'cancelled'
  const isReturned = order.status?.toLowerCase() === 'returned'

  // Auto-poll order details while dispatch is processing and code is not yet returned
  useEffect(() => {
    if (isDispatched && !hasShippingCode) {
      const interval = setInterval(() => {
        queryClient.invalidateQueries({
          queryKey: ['orderDetails', order.uuid],
        })
        queryClient.invalidateQueries({
          queryKey: ['orderTimeline', order.uuid],
        })
      }, 2500)
      return () => clearInterval(interval)
    }
  }, [isDispatched, hasShippingCode, order.uuid, queryClient])

  const handleCopyTracking = (code: string) => {
    navigator.clipboard.writeText(code)
    setCopied(true)
    toast.success('Tracking code copied to clipboard')
    setTimeout(() => setCopied(false), 2000)
  }

  const handleSyncTracking = async () => {
    try {
      await syncTracking.mutateAsync({ uuid: order.uuid })
      toast.success('Tracking status synced successfully with carrier')
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : 'Failed to sync tracking'
      )
    }
  }

  return (
    <div className='space-y-6'>
      {/* 1. Shipment Status Card */}
      {hasShippingCode ? (
        <div className='space-y-4 rounded-lg border bg-card p-4 shadow-xs'>
          <div className='flex flex-wrap items-center justify-between gap-3 border-b pb-3'>
            <div className='flex items-center gap-3'>
              <div className='flex size-10 items-center justify-center rounded-lg border bg-primary/10 text-primary'>
                <Truck className='size-5' />
              </div>
              <div>
                <div className='flex items-center gap-2'>
                  <span className='text-sm font-bold text-foreground'>
                    {order.shipping_provider?.toUpperCase() === 'GHN'
                      ? 'Giao Hàng Nhanh (GHN)'
                      : order.shipping_provider || 'Carrier Delivery'}
                  </span>
                  <Badge
                    variant='outline'
                    className='border-blue-200 bg-blue-500/10 text-xs text-blue-600 dark:border-blue-900/50 dark:text-blue-400'
                  >
                    Active Shipment
                  </Badge>
                </div>
                <div className='flex items-center gap-2 pt-1'>
                  <span className='text-xs text-muted-foreground'>
                    Tracking Code:
                  </span>
                  <code className='rounded bg-muted px-1.5 py-0.5 font-mono text-xs font-semibold text-foreground'>
                    {order.shipping_code}
                  </code>
                  <Button
                    size='icon'
                    variant='ghost'
                    className='size-6 text-muted-foreground hover:text-foreground'
                    onClick={() =>
                      handleCopyTracking(order.shipping_code as string)
                    }
                    title='Copy tracking code'
                  >
                    {copied ? (
                      <Check className='size-3 text-emerald-500' />
                    ) : (
                      <Copy className='size-3' />
                    )}
                  </Button>
                </div>
              </div>
            </div>

            {/* Quick action buttons */}
            <div className='flex flex-wrap items-center gap-2'>
              <Button
                size='sm'
                variant='outline'
                className='h-8 gap-1.5 text-xs'
                onClick={handleSyncTracking}
                disabled={syncTracking.isPending}
              >
                {syncTracking.isPending ? (
                  <Loader2 className='size-3.5 animate-spin' />
                ) : (
                  <RefreshCw className='size-3.5' />
                )}
                Sync Tracking
              </Button>

              <PrintBillDialog
                orderUuid={order.uuid}
                shippingCode={order.shipping_code as string}
              />

              <CancelShippingDialog
                orderUuid={order.uuid}
                shippingCode={order.shipping_code as string}
              />
            </div>
          </div>

          {/* Delivery Details Grid */}
          <div className='grid grid-cols-1 gap-3 text-xs sm:grid-cols-3'>
            {order.expected_delivery && (
              <div className='space-y-1 rounded-md border bg-muted/20 p-2.5'>
                <div className='flex items-center gap-1.5 text-[11px] font-semibold text-muted-foreground uppercase'>
                  <Calendar className='size-3' /> Expected Delivery
                </div>
                <p className='font-medium text-foreground'>
                  {formatDateTime(order.expected_delivery)}
                </p>
              </div>
            )}
            <div className='space-y-1 rounded-md border bg-muted/20 p-2.5'>
              <div className='flex items-center gap-1.5 text-[11px] font-semibold text-muted-foreground uppercase'>
                <Truck className='size-3' /> Delivery Carrier
              </div>
              <p className='font-medium text-foreground capitalize'>
                {order.shipping_provider || 'Standard'}
              </p>
            </div>
            <div className='space-y-1 rounded-md border bg-muted/20 p-2.5'>
              <div className='flex items-center gap-1.5 text-[11px] font-semibold text-muted-foreground uppercase'>
                <Package className='size-3' /> Shipping Status
              </div>
              <p className='font-medium text-foreground capitalize'>
                {order.status}
              </p>
            </div>
          </div>
        </div>
      ) : hasActiveShipment ? (
        <div className='space-y-3 rounded-lg border bg-card p-4 shadow-xs'>
          <div className='flex flex-wrap items-center justify-between gap-3'>
            <div className='flex items-center gap-3'>
              <div className='flex size-10 items-center justify-center rounded-lg border bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'>
                <Truck className='size-5' />
              </div>
              <div>
                <div className='flex items-center gap-2'>
                  <span className='text-sm font-bold text-foreground'>
                    {order.shipping_provider?.toUpperCase() === 'GHN'
                      ? 'Giao Hàng Nhanh (GHN)'
                      : order.shipping_provider || 'Carrier Delivery'}
                  </span>
                  <Badge
                    variant='outline'
                    className='border-emerald-200 bg-emerald-500/10 text-xs text-emerald-600 dark:border-emerald-900/50 dark:text-emerald-400'
                  >
                    Dispatched / In Progress
                  </Badge>
                </div>
                <p className='pt-0.5 text-xs text-muted-foreground'>
                  Shipping order has been submitted to the delivery carrier.
                  Tracking code is being registered.
                </p>
              </div>
            </div>

            <Button
              size='sm'
              variant='outline'
              className='h-8 gap-1.5 text-xs'
              onClick={() => {
                queryClient.invalidateQueries({
                  queryKey: ['orderDetails', order.uuid],
                })
                queryClient.invalidateQueries({
                  queryKey: ['orderTimeline', order.uuid],
                })
              }}
            >
              <RefreshCw className='size-3.5' /> Refresh Status
            </Button>
          </div>
        </div>
      ) : isCancelled || isReturned ? (
        <div className='flex items-center gap-3 rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-xs text-destructive'>
          <AlertTriangle className='size-5 shrink-0' />
          <div>
            <span className='font-semibold'>Shipping Dispatch Unavailable</span>
            <p className='pt-0.5 text-[11px] text-muted-foreground'>
              This order has been {order.status.toLowerCase()}. Creating a new
              delivery shipment is not allowed.
            </p>
          </div>
        </div>
      ) : (
        <CreateShippingForm
          orderUuid={order.uuid}
          shippingAddress={order.shipping_address}
          totalAmount={order.total_amount}
          onSuccess={() => setIsDispatched(true)}
        />
      )}

      {/* 2. Tracking Timeline & Stepper Progress (Storefront Design) */}
      <div className='space-y-3'>
        {isLoadingTimeline ? (
          <div className='space-y-3 border-l py-2 pl-6'>
            <Skeleton className='h-24 w-full rounded-xl' />
            <Skeleton className='h-32 w-full rounded-xl' />
          </div>
        ) : (
          <ShippingTimeline
            order={order}
            events={timelineData?.events || []}
            isSyncing={syncTracking.isPending}
            onSyncTracking={handleSyncTracking}
          />
        )}
      </div>
    </div>
  )
}
