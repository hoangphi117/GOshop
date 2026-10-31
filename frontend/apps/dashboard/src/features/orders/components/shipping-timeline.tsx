import {
  FileText,
  CheckCircle2,
  CreditCard,
  Truck,
  PackageCheck,
  CheckCheck,
  Clock,
  Ban,
  RotateCcw,
  ShieldCheck,
  RefreshCw,
  Loader2,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { formatDateTime } from '../order-utils'
import type { Order, TrackingEvent } from '../types'

interface ShippingTimelineProps {
  order: Order
  events: TrackingEvent[]
  isSyncing?: boolean
  onSyncTracking?: () => void
}

export function ShippingTimeline({
  order,
  events = [],
  isSyncing = false,
  onSyncTracking,
}: ShippingTimelineProps) {
  // Classification helpers matching Storefront
  const isDeliveredEvent = (ev: TrackingEvent): boolean => {
    const s = (ev.status || '').toLowerCase()
    const st = (ev.status_text || '').toLowerCase()
    if (
      s.includes('delivering') ||
      st.includes('out for delivery') ||
      st.includes('đang giao')
    ) {
      return false
    }
    return (
      s.includes('delivered') ||
      s.includes('complete') ||
      s.includes('returned') ||
      s.includes('refunded') ||
      st.includes('delivered') ||
      st.includes('complete') ||
      st.includes('returned') ||
      st.includes('refunded') ||
      s.includes('received') ||
      st.includes('received') ||
      st.includes('đã giao') ||
      st.includes('đã nhận') ||
      st.includes('thành công')
    )
  }

  const isShippingEvent = (ev: TrackingEvent): boolean => {
    const s = (ev.status || '').toLowerCase()
    const st = (ev.status_text || '').toLowerCase()
    if (
      s.includes('fail') ||
      st.includes('fail') ||
      s.includes('cancel') ||
      st.includes('cancel')
    ) {
      return false
    }
    return (
      s.includes('ship') ||
      s.includes('transit') ||
      s.includes('pick') ||
      s.includes('transport') ||
      s.includes('delivering') ||
      st.includes('ship') ||
      st.includes('transit') ||
      st.includes('delivering') ||
      st.includes('out for delivery') ||
      st.includes('vận chuyển') ||
      st.includes('đang giao')
    )
  }

  const formatEventStatusText = (ev: TrackingEvent): string => {
    if (ev.status_text && !ev.status_text.includes('_')) return ev.status_text
    const s = (ev.status || '').toLowerCase()
    switch (s) {
      case 'shipping_failed':
        return 'Shipping creation failed'
      case 'return_shipping_submitted':
        return 'Return waybill submitted'
      case 'return_requested':
        return 'Return requested'
      case 'return_approved':
        return 'Return request approved'
      case 'return_rejected':
        return 'Return request rejected'
      case 'return_cancelled':
        return 'Return request cancelled'
      case 'returned':
        return 'Returned goods processed'
      default:
        return (
          ev.status_text ||
          ev.status?.replace(/_/g, ' ').toUpperCase() ||
          'Status Update'
        )
    }
  }

  const formatEventNote = (note?: string): string => {
    if (!note) return ''
    return note.replace(
      /User submitted return shipping info for/gi,
      'Return waybill submitted for'
    )
  }

  const checkIsCOD = (o: Order) => {
    const pm = (o.payment_method || '').toUpperCase()
    return pm.includes('COD') || pm.includes('CASH')
  }

  // Check delivered timestamp
  const deliveredEv = events.find((e) => {
    const s = (e.status || '').toLowerCase()
    const st = (e.status_text || '').toLowerCase()
    if (s.includes('delivering') || st.includes('out for delivery'))
      return false
    return (
      s.includes('delivered') ||
      s.includes('complete') ||
      s.includes('received')
    )
  })
  const deliveredTimestamp =
    deliveredEv?.created_at ||
    ((order as unknown as Record<string, unknown>)
      .order_delivered_at as string) ||
    null

  const getActiveStepIndex = (o: Order): number => {
    const s = (o.status || '').toUpperCase()
    const ps = (o.payment_status || '').toUpperCase()
    const isCOD = checkIsCOD(o)

    if (s.includes('CANCEL')) return -1

    // Check if any returned or return requested
    const hasReturnEvent = events.some((e) =>
      (e.status || '').toLowerCase().startsWith('return')
    )
    if (s === 'RETURNED' || hasReturnEvent) {
      return 4
    }

    // If delivered or completed
    if (s === 'DELIVERED' || s === 'COMPLETED' || s === 'COMPLETE') {
      return 4
    }

    // If carrier event has delivered (excluding delivering/out for delivery)
    const hasDeliveredEvent = events.some((e) => {
      const es = (e.status || '').toLowerCase()
      const est = (e.status_text || '').toLowerCase()
      if (
        es.includes('delivering') ||
        est.includes('out for delivery') ||
        est.includes('đang giao')
      ) {
        return false
      }
      return (
        es.includes('delivered') ||
        es.includes('complete') ||
        es.includes('received') ||
        est.includes('delivered') ||
        est.includes('đã giao') ||
        est.includes('đã nhận')
      )
    })
    if (hasDeliveredEvent) {
      return 4
    }

    if (
      s === 'SHIPPED' ||
      s === 'IN_TRANSIT' ||
      s === 'INTRANSIT' ||
      events.some(isShippingEvent)
    ) {
      return 2 // Order shipped out
    }

    if (s.includes('CONFIRM')) {
      return 1 // Payment info confirmed (COD) / Order paid (Online)
    }

    if (!isCOD && ps === 'PAID') {
      return 1 // Order paid (Online)
    }

    return 0 // Order Placed
  }

  const getStepTimestamp = (stepIdx: number): string | null => {
    // Step 0: Order Placed
    if (stepIdx === 0) {
      const ev = events.find((e) => {
        const s = (e.status || '').toLowerCase()
        return s === 'pending' || s.includes('placed') || s.includes('create')
      })
      return ev?.created_at || order.created_at || null
    }

    // Step 1: Payment info confirmed (COD) / Order paid (Online)
    if (stepIdx === 1) {
      const isCOD = checkIsCOD(order)
      if (isCOD) {
        const ev = events.find((e) =>
          (e.status || '').toLowerCase().includes('confirm')
        )
        return ev?.created_at || null
      } else {
        const ev = events.find((e) => {
          const s = (e.status || '').toLowerCase()
          const st = (e.status_text || '').toLowerCase()
          const n = (e.note || '').toLowerCase()
          return (
            s.includes('paid') ||
            s.includes('payment') ||
            st.includes('paid') ||
            st.includes('payment') ||
            n.includes('paid') ||
            n.includes('thanh toán')
          )
        })
        if (ev?.created_at) return ev.created_at
        if ((order.payment_status || '').toUpperCase() === 'PAID') {
          const confirmEv = events.find((e) =>
            (e.status || '').toLowerCase().includes('confirm')
          )
          return confirmEv?.created_at || order.created_at || null
        }
        return null
      }
    }

    // Step 2: Order shipped out
    if (stepIdx === 2) {
      const ev = events.find((e) => isShippingEvent(e))
      return ev?.created_at || null
    }

    // Step 3: Order received (Must NOT match delivering or out for delivery)
    if (stepIdx === 3) {
      const ev = events.find((e) => {
        const s = (e.status || '').toLowerCase()
        const st = (e.status_text || '').toLowerCase()
        if (
          s.includes('delivering') ||
          st.includes('out for delivery') ||
          st.includes('đang giao')
        ) {
          return false
        }
        return (
          s.includes('delivered') ||
          s.includes('complete') ||
          s.includes('received') ||
          st.includes('delivered') ||
          st.includes('complete') ||
          st.includes('đã giao') ||
          st.includes('đã nhận')
        )
      })
      const isDeliveredStatus =
        (order.status || '').toUpperCase() === 'DELIVERED' ||
        (order.status || '').toUpperCase() === 'RETURNED'
      return ev?.created_at || (isDeliveredStatus ? deliveredTimestamp : null)
    }

    // Step 4: Completed or Return State
    if (stepIdx === 4) {
      const returnCompletedEv = events.find(
        (e) => (e.status || '').toLowerCase() === 'returned'
      )
      if (returnCompletedEv) return returnCompletedEv.created_at

      const returnApprovedEv = events.find(
        (e) => (e.status || '').toLowerCase() === 'return_approved'
      )
      if (returnApprovedEv) return returnApprovedEv.created_at

      const returnRequestedEv = events.find(
        (e) => (e.status || '').toLowerCase() === 'return_requested'
      )
      if (returnRequestedEv) return returnRequestedEv.created_at

      const returnRejectedEv = events.find(
        (e) => (e.status || '').toLowerCase() === 'return_rejected'
      )
      if (returnRejectedEv) return returnRejectedEv.created_at

      const deliveredEv = events.find((e) => {
        const es = (e.status || '').toLowerCase()
        const est = (e.status_text || '').toLowerCase()
        if (
          es.includes('delivering') ||
          est.includes('out for delivery') ||
          est.includes('đang giao')
        ) {
          return false
        }
        return (
          es.includes('delivered') ||
          es.includes('complete') ||
          es.includes('received') ||
          est.includes('delivered') ||
          est.includes('đã giao')
        )
      })
      const isDeliveredStatus =
        (order.status || '').toUpperCase() === 'DELIVERED' ||
        (order.status || '').toUpperCase() === 'RETURNED'
      return deliveredEv?.created_at || (isDeliveredStatus ? deliveredTimestamp : null)
    }

    return null
  }

  const isCOD = checkIsCOD(order)
  const activeStepIndex = getActiveStepIndex(order)
  const isCancelled = activeStepIndex === -1

  const returnApprovedEvent = events.find(
    (e) => (e.status || '').toLowerCase() === 'return_approved'
  )
  const returnRequestedEvent = events.find(
    (e) => (e.status || '').toLowerCase() === 'return_requested'
  )
  const returnRejectedEvent = events.find(
    (e) => (e.status || '').toLowerCase() === 'return_rejected'
  )
  const returnCancelledEvent = events.find(
    (e) => (e.status || '').toLowerCase() === 'return_cancelled'
  )
  const isOrderReturned =
    (order.status || '').toUpperCase() === 'RETURNED' ||
    events.some((e) => (e.status || '').toLowerCase() === 'returned')

  const isReturnApproved = Boolean(
    returnApprovedEvent &&
      !isOrderReturned &&
      !returnRejectedEvent &&
      !returnCancelledEvent
  )
  const isReturnRequested = Boolean(
    returnRequestedEvent &&
      !returnApprovedEvent &&
      !isOrderReturned &&
      !returnRejectedEvent &&
      !returnCancelledEvent
  )

  let finalStepLabel = 'Completed'
  let finalStepIcon = CheckCircle2
  if (isOrderReturned) {
    finalStepLabel = 'Returned & Refunded'
    finalStepIcon = RotateCcw
  } else if (isReturnApproved) {
    finalStepLabel = 'Return approved'
    finalStepIcon = ShieldCheck
  } else if (isReturnRequested) {
    finalStepLabel = 'Return requested'
    finalStepIcon = Clock
  } else if (returnRejectedEvent) {
    finalStepLabel = 'Return rejected'
    finalStepIcon = Ban
  }

  const isDelivered = order.status?.toUpperCase() === 'DELIVERED'
  const isShipped =
    order.status?.toUpperCase() === 'SHIPPED' ||
    order.status?.toUpperCase() === 'IN_TRANSIT'

  const currentSteps = [
    { key: 'placed', label: 'Order Placed', icon: FileText },
    {
      key: 'payment',
      label: isCOD ? 'Payment info confirmed' : 'Order paid',
      icon: isCOD ? CheckCircle2 : CreditCard,
    },
    { key: 'shipped', label: 'Order shipped out', icon: Truck },
    { key: 'received', label: 'Order received', icon: PackageCheck },
    { key: 'completed', label: finalStepLabel, icon: finalStepIcon },
  ]

  const synthesizeOrderEvents = (ord: Order): TrackingEvent[] => {
    const list: TrackingEvent[] = []
    const s = (ord.status || '').toUpperCase()
    const ps = (ord.payment_status || '').toUpperCase()

    // 1. Placed
    list.push({
      event_id: 1,
      status: 'placed',
      status_text: 'Order placed successfully',
      note: `Order #${ord.order_number || ord.uuid?.slice(0, 8).toUpperCase()} has been placed.`,
      source: 'system',
      created_at: ord.created_at || new Date().toISOString(),
    })

    // 2. Confirmed / Paid
    if (
      ps === 'PAID' ||
      s === 'CONFIRMED' ||
      s === 'SHIPPED' ||
      s === 'DELIVERED' ||
      s === 'RETURNED'
    ) {
      list.push({
        event_id: 2,
        status: 'confirmed',
        status_text: isCOD ? 'Order confirmed' : 'Payment confirmed',
        note: isCOD
          ? 'Order verified for Cash on Delivery (COD)'
          : `Payment received via ${ord.payment_method || 'Online Payment'}.`,
        source: 'system',
        created_at: ord.created_at || new Date().toISOString(),
      })
    }

    // 3. Shipped
    if (
      s === 'SHIPPED' ||
      s === 'IN_TRANSIT' ||
      s === 'DELIVERED' ||
      s === 'RETURNED' ||
      ord.shipping_code
    ) {
      list.push({
        event_id: 3,
        status: 'in_transit',
        status_text: 'In transit with carrier',
        note: `Parcel handed over to GHN Express${ord.shipping_code ? ` (Tracking: ${ord.shipping_code})` : ''}.`,
        source: 'carrier',
        created_at: ord.created_at || new Date().toISOString(),
      })
    }

    // 4. Delivered
    if (s === 'DELIVERED' || s === 'RETURNED') {
      list.push({
        event_id: 4,
        status: 'delivered',
        status_text: 'Delivered successfully',
        note: 'Parcel delivered successfully to recipient.',
        source: 'carrier',
        created_at: ord.created_at || new Date().toISOString(),
      })
    }

    // 5. Cancelled
    if (s === 'CANCELLED') {
      list.push({
        event_id: 5,
        status: 'cancelled',
        status_text: 'Order cancelled',
        note: 'Order has been cancelled.',
        source: 'system',
        created_at: ord.created_at || new Date().toISOString(),
      })
    }

    // 6. Returned
    if (s === 'RETURNED') {
      list.push({
        event_id: 6,
        status: 'returned',
        status_text: 'Returned goods processed successfully',
        note: 'Return request processed and completed.',
        source: 'system',
        created_at: ord.created_at || new Date().toISOString(),
      })
    }

    return list
  }

  // Reverse events for tracking history (latest first)
  const effectiveEvents =
    events.length > 0 ? events : synthesizeOrderEvents(order)
  const sortedEvents = [...effectiveEvents].reverse()

  return (
    <div className='space-y-5'>
      {/* 1. HORIZONTAL 5-STAGE PROCESS STEPPER BAR (STOREFRONT DESIGN) */}
      <div className='rounded-xl border bg-card p-5 shadow-2xs sm:p-6'>
        {isCancelled ? (
          <div className='flex items-start gap-3 rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-xs'>
            <div className='mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-full bg-destructive/10 text-destructive'>
              <Ban className='size-5' />
            </div>
            <div className='space-y-1'>
              <h3 className='text-sm font-bold text-destructive'>
                Order Cancelled
              </h3>
              <p className='text-muted-foreground'>
                This order has been cancelled. Any active carrier shipments have
                been voided.
              </p>
              <p className='pt-1 font-mono text-[11px] font-medium text-destructive/80'>
                Updated on: {formatDateTime(order.created_at)}
              </p>
            </div>
          </div>
        ) : (
          <div className='overflow-x-auto pb-2'>
            <div className='min-w-[560px] sm:min-w-0'>
              <div className='relative flex items-center justify-between'>
                {/* Inactive Background Track */}
                <div className='absolute top-5 right-[10%] left-[10%] z-0 h-1 -translate-y-1/2 bg-muted sm:top-6' />

                {/* Active Process Line (GREEN) */}
                <div
                  className='absolute top-5 left-[10%] z-0 h-1 -translate-y-1/2 bg-emerald-500 transition-all duration-500 sm:top-6'
                  style={{
                    width: `${
                      (Math.min(activeStepIndex, currentSteps.length - 1) /
                        (currentSteps.length - 1)) *
                      80
                    }%`,
                  }}
                />

                {/* Step Nodes & Labels */}
                {currentSteps.map((step, idx) => {
                  const isCompleted = idx < activeStepIndex
                  const isCurrent = idx === activeStepIndex
                  const isReached = idx <= activeStepIndex
                  const timestamp = getStepTimestamp(idx)
                  const Icon = step.icon

                  return (
                    <div
                      key={step.key}
                      className='relative z-10 flex flex-1 flex-col items-center px-1 text-center'
                    >
                      {/* Step Circle */}
                      <div
                        className={`flex size-10 items-center justify-center rounded-full transition-all duration-300 sm:size-12 ${
                          isCompleted
                            ? 'bg-emerald-500 text-white shadow-md shadow-emerald-500/20'
                            : isCurrent
                              ? 'bg-emerald-600 text-white shadow-md ring-4 shadow-emerald-500/30 ring-emerald-500/25'
                              : 'border-2 border-border bg-card text-muted-foreground'
                        }`}
                      >
                        {isCompleted ? (
                          <CheckCheck className='size-5 sm:size-6' />
                        ) : (
                          <Icon className='size-5 sm:size-6' />
                        )}
                      </div>

                      {/* Step Label */}
                      <span
                        className={`mt-2.5 text-xs leading-tight font-bold ${
                          isReached
                            ? 'text-emerald-600 dark:text-emerald-400'
                            : 'text-muted-foreground'
                        }`}
                      >
                        {step.label}
                      </span>

                      {/* Step Timestamp */}
                      {timestamp ? (
                        <span
                          className={`mt-1 font-mono text-[10px] leading-none ${
                            isReached
                              ? 'font-medium text-emerald-600/80 dark:text-emerald-400/80'
                              : 'text-muted-foreground'
                          }`}
                        >
                          {formatDateTime(timestamp)}
                        </span>
                      ) : (
                        <span className='mt-1 font-mono text-[10px] leading-none text-muted-foreground/50'>
                          —
                        </span>
                      )}
                    </div>
                  )
                })}
              </div>
            </div>
          </div>
        )}

        {/* Status Context Bar */}
        <div className='mt-5 flex flex-col justify-between gap-3 border-t pt-4 text-xs sm:flex-row sm:items-center'>
          <div className='flex items-center gap-2 text-muted-foreground'>
            <ShieldCheck className='size-4 shrink-0 text-emerald-600 dark:text-emerald-400' />
            <span>
              {isDelivered
                ? 'Parcel has been delivered successfully to the recipient.'
                : isShipped
                  ? 'Parcel is in transit with carrier. Tracking synchronization is active.'
                  : activeStepIndex >= 2
                    ? 'Parcel is being handled by delivery carrier and moving towards destination.'
                    : 'Order is confirmed and awaiting logistics dispatch.'}
            </span>
          </div>
        </div>
      </div>

      {/* 2. VERTICAL TIMELINE & TRACKING HISTORY (STOREFRONT 3-COLUMN DESIGN) */}
      <div className='space-y-5 rounded-xl border bg-card p-5 shadow-2xs sm:p-6'>
        <div className='flex flex-col justify-between gap-2 border-b pb-3 sm:flex-row sm:items-center'>
          <div className='flex items-center gap-2.5'>
            <div className='flex size-8 items-center justify-center rounded-lg bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'>
              <Clock className='size-4' />
            </div>
            <h3 className='text-xs font-extrabold tracking-wider text-foreground uppercase sm:text-sm'>
              Order Timeline & Tracking History
            </h3>
          </div>

          <div className='flex items-center gap-3 text-xs'>
            <span className='text-muted-foreground'>
              Carrier:{' '}
              <strong className='font-semibold text-foreground'>
                {order.shipping_provider?.toUpperCase() === 'GHN'
                  ? 'GHN Express'
                  : order.shipping_provider || 'Standard Carrier'}
              </strong>
            </span>

            {onSyncTracking && (
              <Button
                size='sm'
                variant='outline'
                disabled={isSyncing}
                onClick={onSyncTracking}
                className='h-7 gap-1.5 text-xs'
                title='Synchronize live tracking updates from shipping carrier'
              >
                {isSyncing ? (
                  <Loader2 className='size-3.5 animate-spin text-primary' />
                ) : (
                  <RefreshCw className='size-3.5' />
                )}
                Sync Tracking
              </Button>
            )}
          </div>
        </div>

        {sortedEvents.length > 0 ? (
          <div className='space-y-0'>
            {sortedEvents.map((ev, i) => {
              const isDeliveredEv = isDeliveredEvent(ev)
              const isShippingEv = isShippingEvent(ev)
              const isLast = i === sortedEvents.length - 1

              return (
                <div
                  key={ev.event_id || i}
                  className='relative flex items-start gap-3 sm:gap-4'
                >
                  {/* Left Column: Timestamp */}
                  <div className='w-28 shrink-0 pt-0.5 text-right sm:w-36'>
                    <p
                      className={`font-mono text-xs leading-tight font-semibold sm:text-sm ${
                        isDeliveredEv
                          ? 'text-emerald-600 dark:text-emerald-400'
                          : 'text-muted-foreground'
                      }`}
                    >
                      {formatDateTime(ev.created_at)}
                    </p>
                  </div>

                  {/* Middle Column: Continuous Line + Centered Node */}
                  <div className='relative flex w-8 shrink-0 flex-col items-center self-stretch'>
                    {sortedEvents.length > 1 && (
                      <div
                        className={`absolute left-1/2 w-0.5 -translate-x-1/2 bg-border ${
                          i === 0
                            ? 'top-3 bottom-0'
                            : isLast
                              ? 'top-0 h-3'
                              : 'top-0 bottom-0'
                        }`}
                      />
                    )}

                    {/* Node circle on top of line */}
                    <div className='relative z-10 bg-card'>
                      {isDeliveredEv ? (
                        <div className='flex size-6 items-center justify-center rounded-full bg-emerald-500 text-white shadow-xs'>
                          <CheckCheck className='size-3.5' />
                        </div>
                      ) : isShippingEv ? (
                        <div className='flex size-6 items-center justify-center rounded-full border-2 border-slate-400 bg-card text-foreground shadow-xs dark:border-slate-500'>
                          <Truck className='size-3.5' />
                        </div>
                      ) : (
                        <div className='flex size-6 items-center justify-center'>
                          <div className='size-3 rounded-full border-2 border-background bg-muted-foreground/40 ring-1 ring-border' />
                        </div>
                      )}
                    </div>
                  </div>

                  {/* Right Column: Event Content */}
                  <div
                    className={`min-w-0 flex-1 pt-0.5 ${
                      isLast ? 'pb-1' : 'pb-6'
                    }`}
                  >
                    <div className='flex flex-wrap items-center gap-2'>
                      <h4
                        className={`text-xs leading-snug font-bold sm:text-sm ${
                          isDeliveredEv
                            ? 'text-emerald-600 dark:text-emerald-400'
                            : 'text-foreground'
                        }`}
                      >
                        {formatEventStatusText(ev)}
                      </h4>
                    </div>
                    {ev.note && (
                      <p
                        className={`mt-1 text-xs leading-relaxed ${
                          isDeliveredEv
                            ? 'text-emerald-600/90 dark:text-emerald-400/90'
                            : 'text-muted-foreground'
                        }`}
                      >
                        {formatEventNote(ev.note)}
                      </p>
                    )}
                  </div>
                </div>
              )
            })}
          </div>
        ) : (
          <div className='flex items-center gap-2.5 rounded-lg border bg-muted/20 p-3 py-3 text-xs text-muted-foreground'>
            <CheckCircle2 className='size-4 shrink-0 text-emerald-500' />
            <span>
              Order was successfully created on{' '}
              <strong className='text-foreground'>
                {formatDateTime(order.created_at)}
              </strong>
              . Awaiting logistics dispatch and carrier tracking events.
            </span>
          </div>
        )}
      </div>
    </div>
  )
}
