'use client'

import React, { useState, useEffect, useCallback } from 'react'
import Link from 'next/link'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { authFetch } from '@/lib/api'
import { formatCurrency, formatDate, formatDateTime } from '@/lib/format'
import { getImageUrl, extractErrorMessage } from '@/lib/utils'
import {
  ShoppingBag,
  Clock,
  CheckCircle2,
  Package,
  AlertCircle,
  ArrowRight,
  ArrowLeft,
  CreditCard,
  X,
  Truck,
  Eye,
  RefreshCw,
  MapPin,
  FileText,
  Tag,
  Ban,
  RotateCcw,
  AlertTriangle,
  Building2,
  Filter,
  ChevronDown,
  CheckCheck,
  ShieldCheck,
  PackageCheck,
  Star,
  Search,
} from 'lucide-react'

interface OrderItem {
  uuid?: string
  product_uuid?: string
  product_name: string
  product_slug?: string
  image_url?: string
  price: number
  quantity: number
  subtotal?: number
}

interface TrackingEvent {
  event_id?: number
  status: string
  status_text?: string
  note?: string
  source?: string
  created_at: string
}

interface Order {
  uuid: string
  order_number: string
  created_at: string
  status: string
  payment_status?: string
  payment_method?: string
  total_amount: number
  subtotal_amount?: number
  shipping_fee?: number
  discount_amount?: number
  coupon_code?: string
  shipping_name?: string
  shipping_phone?: string
  shipping_address?: string
  note?: string
  items?: OrderItem[]
}

export default function MyOrdersPage() {
  const { accessToken } = useAuthStore()
  const [orders, setOrders] = useState<Order[]>([])
  const [loading, setLoading] = useState(true)
  const [filter, setFilter] = useState('ALL')
  const [searchQuery, setSearchQuery] = useState('')

  // Selected Order for in-place Detail View
  const [selectedOrder, setSelectedOrder] = useState<Order | null>(null)
  const [timelineEvents, setTimelineEvents] = useState<TrackingEvent[]>([])
  const [isLoadingDetail, setIsLoadingDetail] = useState(false)

  // Cancel Order Modal State
  const [orderToCancel, setOrderToCancel] = useState<Order | null>(null)
  const [isCancelling, setIsCancelling] = useState(false)

  // Return / Refund Modal State
  const [orderToReturn, setOrderToReturn] = useState<Order | null>(null)
  const [isSubmittingReturn, setIsSubmittingReturn] = useState(false)
  const [returnReason, setReturnReason] = useState('')
  const [bankName, setBankName] = useState('')
  const [bankAccount, setBankAccount] = useState('')
  const [bankAccountName, setBankAccountName] = useState('')

  const fetchOrders = useCallback(async () => {
    if (!accessToken) return
    setLoading(true)
    try {
      const baseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
      const res = await authFetch(`${baseUrl}/api/v1/orders`)
      if (res.ok) {
        const json = await res.json()
        const orderList: Order[] = Array.isArray(json?.data)
          ? json.data
          : Array.isArray(json?.data?.items)
          ? json.data.items
          : []
        setOrders(orderList)
      } else {
        setOrders([])
      }
    } catch {
      setOrders([])
    } finally {
      setLoading(false)
    }
  }, [accessToken])

  useEffect(() => {
    fetchOrders()
  }, [fetchOrders])

  // In-place switch to order detail
  const handleOpenOrderDetails = async (order: Order) => {
    setSelectedOrder(order)
    setTimelineEvents([])
    setIsLoadingDetail(true)

    try {
      const baseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'

      const [orderRes, timelineRes] = await Promise.allSettled([
        authFetch(`${baseUrl}/api/v1/orders/${order.uuid}`),
        authFetch(`${baseUrl}/api/v1/orders/${order.uuid}/timeline`),
      ])

      if (orderRes.status === 'fulfilled' && orderRes.value.ok) {
        const json = await orderRes.value.json()
        if (json?.data) {
          setSelectedOrder(json.data)
        }
      }

      if (timelineRes.status === 'fulfilled' && timelineRes.value.ok) {
        const json = await timelineRes.value.json()
        const events: TrackingEvent[] = Array.isArray(json?.data?.events)
          ? json.data.events
          : Array.isArray(json?.data)
          ? json.data
          : []
        setTimelineEvents(events)
      }
    } catch {
      // Retain existing summary if network fails
    } finally {
      setIsLoadingDetail(false)
    }
  }

  // --- CANCEL ORDER ACTIONS ---
  const handleOpenCancelConfirmation = (order: Order) => {
    setOrderToCancel(order)
  }

  const handleConfirmCancel = async () => {
    if (!orderToCancel) return
    setIsCancelling(true)

    try {
      const baseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
      const res = await authFetch(`${baseUrl}/api/v1/orders/${orderToCancel.uuid}/cancel`, {
        method: 'PATCH',
      })

      if (res.ok) {
        toast.success(
          `Order #${orderToCancel.order_number || orderToCancel.uuid.slice(0, 8)} cancelled successfully.`
        )
        setOrderToCancel(null)
        if (selectedOrder && selectedOrder.uuid === orderToCancel.uuid) {
          setSelectedOrder({ ...selectedOrder, status: 'Cancelled' })
        }
        await fetchOrders()
      } else {
        const errJson = await res.json().catch(() => null)
        const msg = extractErrorMessage(errJson, 'Failed to cancel order.')
        toast.error(msg)
      }
    } catch {
      toast.error('Network error occurred while cancelling order.')
    } finally {
      setIsCancelling(false)
    }
  }

  // --- RETURN / REFUND ACTIONS ---
  const handleOpenReturnModal = (order: Order) => {
    setOrderToReturn(order)
    setReturnReason('')
    setBankName('')
    setBankAccount('')
    setBankAccountName('')
  }

  const handleSubmitReturn = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!orderToReturn) return

    if (returnReason.trim().length < 10) {
      toast.error('Please provide a detailed return reason (at least 10 characters).')
      return
    }

    setIsSubmittingReturn(true)

    try {
      const baseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
      const payload: Record<string, string> = {
        reason: returnReason.trim(),
      }
      if (bankName.trim()) payload.bank_name = bankName.trim()
      if (bankAccount.trim()) payload.bank_account = bankAccount.trim()
      if (bankAccountName.trim()) payload.bank_account_name = bankAccountName.trim()

      const res = await authFetch(`${baseUrl}/api/v1/orders/${orderToReturn.uuid}/return`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(payload),
      })

      if (res.ok) {
        toast.success(
          'Return & refund request submitted successfully! Our team will review it shortly.'
        )
        setOrderToReturn(null)
        if (selectedOrder && selectedOrder.uuid === orderToReturn.uuid) {
          setSelectedOrder({ ...selectedOrder, status: 'Returned' })
        }
        await fetchOrders()
      } else {
        const errJson = await res.json().catch(() => null)
        const msg = extractErrorMessage(errJson, 'Failed to submit return request.')
        toast.error(msg)
      }
    } catch {
      toast.error('Network error occurred while submitting return request.')
    } finally {
      setIsSubmittingReturn(false)
    }
  }

  const filteredOrders = orders.filter((o) => {
    // 1. Status Filter
    if (filter !== 'ALL') {
      const s = (o.status || '').toUpperCase()
      if (filter === 'PENDING' && !s.includes('PENDING')) return false
      if (filter === 'CONFIRMED' && !s.includes('CONFIRM')) return false
      if (filter === 'SHIPPED' && !s.includes('SHIP')) return false
      if (filter === 'DELIVERED' && !s.includes('DELIVER') && !s.includes('COMPLETE')) return false
      if (filter === 'CANCELLED' && !s.includes('CANCEL')) return false
      if (filter === 'RETURNED' && !s.includes('RETURN')) return false
      if (!['PENDING', 'CONFIRMED', 'SHIPPED', 'DELIVERED', 'CANCELLED', 'RETURNED'].includes(filter) && s !== filter) {
        return false
      }
    }

    // 2. Search Query Filter (Order Number, ID, Recipient info, Product name)
    if (searchQuery.trim()) {
      const q = searchQuery.trim().toLowerCase()
      const matchOrderNumber = (o.order_number || '').toLowerCase().includes(q)
      const matchUuid = (o.uuid || '').toLowerCase().includes(q)
      const matchRecipient = (o.shipping_name || '').toLowerCase().includes(q)
      const matchPhone = (o.shipping_phone || '').toLowerCase().includes(q)
      const matchAddress = (o.shipping_address || '').toLowerCase().includes(q)
      const matchItems = o.items?.some((item) =>
        (item.product_name || '').toLowerCase().includes(q)
      )

      if (
        !matchOrderNumber &&
        !matchUuid &&
        !matchRecipient &&
        !matchPhone &&
        !matchAddress &&
        !matchItems
      ) {
        return false
      }
    }

    return true
  })

  // Order Fulfillment Status Badge
  const getOrderStatusBadge = (status: string) => {
    const s = (status || '').toUpperCase()
    if (s.includes('DELIVER') || s.includes('COMPLETE')) {
      return (
        <span className="px-2.5 py-1 rounded-full bg-emerald-50 text-emerald-700 text-[11px] font-bold inline-flex items-center gap-1 border border-emerald-200">
          <CheckCircle2 className="w-3 h-3" />
          {status}
        </span>
      )
    }
    if (s.includes('SHIP')) {
      return (
        <span className="px-2.5 py-1 rounded-full bg-indigo-50 text-indigo-700 text-[11px] font-bold inline-flex items-center gap-1 border border-indigo-200">
          <Truck className="w-3 h-3" />
          {status}
        </span>
      )
    }
    if (s.includes('CONFIRM')) {
      return (
        <span className="px-2.5 py-1 rounded-full bg-blue-50 text-blue-700 text-[11px] font-bold inline-flex items-center gap-1 border border-blue-200">
          <CheckCircle2 className="w-3 h-3" />
          {status}
        </span>
      )
    }
    if (s.includes('PENDING')) {
      return (
        <span className="px-2.5 py-1 rounded-full bg-amber-50 text-amber-700 text-[11px] font-bold inline-flex items-center gap-1 border border-amber-200">
          <Clock className="w-3 h-3" />
          {status}
        </span>
      )
    }
    if (s.includes('CANCEL')) {
      return (
        <span className="px-2.5 py-1 rounded-full bg-rose-50 text-rose-700 text-[11px] font-bold inline-flex items-center gap-1 border border-rose-200">
          <AlertCircle className="w-3 h-3" />
          {status}
        </span>
      )
    }
    if (s.includes('RETURN')) {
      return (
        <span className="px-2.5 py-1 rounded-full bg-purple-50 text-purple-700 text-[11px] font-bold inline-flex items-center gap-1 border border-purple-200">
          <RotateCcw className="w-3 h-3" />
          {status}
        </span>
      )
    }
    return (
      <span className="px-2.5 py-1 rounded-full bg-slate-100 text-slate-700 text-[11px] font-bold inline-flex items-center gap-1 border border-slate-200">
        <Package className="w-3 h-3" />
        {status || 'Processing'}
      </span>
    )
  }

  // Payment Status Badge (Paid / Unpaid / Refunded)
  const getPaymentStatusBadge = (paymentStatus?: string) => {
    const ps = (paymentStatus || '').toUpperCase()
    if (ps === 'PAID') {
      return (
        <span className="px-2.5 py-1 rounded-full bg-emerald-500/10 text-emerald-700 text-[11px] font-bold inline-flex items-center gap-1 border border-emerald-500/20">
          <CheckCircle2 className="w-3 h-3 text-emerald-600" />
          Paid
        </span>
      )
    }
    if (ps === 'UNPAID') {
      return (
        <span className="px-2.5 py-1 rounded-full bg-amber-500/10 text-amber-700 text-[11px] font-bold inline-flex items-center gap-1 border border-amber-500/20">
          <Clock className="w-3 h-3 text-amber-600" />
          Unpaid
        </span>
      )
    }
    if (ps.includes('REFUND')) {
      return (
        <span className="px-2.5 py-1 rounded-full bg-purple-500/10 text-purple-700 text-[11px] font-bold inline-flex items-center gap-1 border border-purple-500/20">
          <RefreshCw className="w-3 h-3 text-purple-600" />
          {paymentStatus}
        </span>
      )
    }
    return (
      <span className="px-2.5 py-1 rounded-full bg-slate-100 text-slate-700 text-[11px] font-bold inline-flex items-center gap-1 border border-slate-200">
        <CreditCard className="w-3 h-3 text-slate-500" />
        {paymentStatus || 'Pending'}
      </span>
    )
  }

  // ==========================================
  // NEW 5-STAGE PROCESS BAR CALCULATION
  // Order Placed -> Payment info confirmed (COD) / Order paid (Online) -> Order shipped out -> Order received -> To rate
  // ==========================================
  const checkIsCOD = (order: Order) => {
    const pm = (order.payment_method || '').toUpperCase()
    return pm.includes('COD') || pm.includes('CASH')
  }

  const getActiveStepIndex = (order: Order, _events: TrackingEvent[]): number => {
    const s = (order.status || '').toUpperCase()
    const ps = (order.payment_status || '').toUpperCase()
    const isCOD = checkIsCOD(order)

    if (s.includes('CANCEL')) return -1
    if (s.includes('RETURN')) return -2

    // If delivered or completed, order received is reached (Step 3 or Step 4 To Rate)
    if (s.includes('DELIVER') || s.includes('COMPLETE')) {
      return 4 // Reached "To rate" stage
    }

    if (s.includes('SHIP')) {
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

  const getStepTimestamp = (
    stepIdx: number,
    order: Order,
    events: TrackingEvent[]
  ): string | null => {
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
        const ev = events.find((e) => (e.status || '').toLowerCase().includes('confirm'))
        return ev?.created_at || null
      } else {
        // Online payment: find event with paid or payment timestamp
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
          const confirmEv = events.find((e) => (e.status || '').toLowerCase().includes('confirm'))
          return confirmEv?.created_at || order.created_at || null
        }
        return null
      }
    }

    // Step 2: Order shipped out
    if (stepIdx === 2) {
      const ev = events.find((e) => {
        const s = (e.status || '').toLowerCase()
        return s.includes('ship') || s.includes('transit') || s.includes('pick')
      })
      return ev?.created_at || null
    }

    // Step 3: Order received
    if (stepIdx === 3) {
      const ev = events.find((e) => {
        const s = (e.status || '').toLowerCase()
        return (
          s.includes('deliver') ||
          s.includes('complete') ||
          s.includes('received')
        )
      })
      return ev?.created_at || null
    }

    // Step 4: To rate
    if (stepIdx === 4) {
      return null
    }

    return null
  }

  // Timeline Event Classification Helpers
  const isDeliveredEvent = (ev: TrackingEvent): boolean => {
    const s = (ev.status || '').toLowerCase()
    const st = (ev.status_text || '').toLowerCase()
    return (
      s.includes('deliver') ||
      s.includes('complete') ||
      st.includes('deliver') ||
      st.includes('complete') ||
      s.includes('received') ||
      st.includes('đã giao')
    )
  }

  const isShippingEvent = (ev: TrackingEvent): boolean => {
    const s = (ev.status || '').toLowerCase()
    const st = (ev.status_text || '').toLowerCase()
    return (
      s.includes('ship') ||
      s.includes('transit') ||
      s.includes('pick') ||
      s.includes('transport') ||
      st.includes('ship') ||
      st.includes('transit') ||
      st.includes('delivering') ||
      st.includes('vận chuyển') ||
      st.includes('đang giao')
    )
  }

  if (loading) {
    return (
      <div className="p-8 rounded-3xl bg-white border border-slate-200/80 shadow-xs flex items-center justify-center min-h-[300px]">
        <div className="w-8 h-8 rounded-full border-2 border-[#2196F3] border-t-transparent animate-spin" />
      </div>
    )
  }

  // ==========================================
  // VIEW 1: SHOPEE-STYLE ORDER DETAIL VIEW
  // ==========================================
  if (selectedOrder) {
    const isPending = selectedOrder.status?.toUpperCase() === 'PENDING'
    const isDelivered = selectedOrder.status?.toUpperCase() === 'DELIVERED'
    const activeStepIndex = getActiveStepIndex(selectedOrder, timelineEvents)
    const isCancelled = activeStepIndex === -1
    const isReturned = activeStepIndex === -2
    const isCOD = checkIsCOD(selectedOrder)

    // Dynamic 5-stage steps
    const currentSteps = [
      { key: 'placed', label: 'Order Placed', icon: FileText },
      {
        key: 'payment',
        label: isCOD ? 'Payment info confirmed' : 'Order paid',
        icon: isCOD ? CheckCircle2 : CreditCard,
      },
      { key: 'shipped', label: 'Order shipped out', icon: Truck },
      { key: 'received', label: 'Order received', icon: PackageCheck },
      { key: 'rate', label: 'To rate', icon: Star },
    ]

    // Reverse events for tracking history (latest first)
    const sortedEvents = [...timelineEvents].reverse()

    return (
      <div className="space-y-4">
        {/* Shopee Top Control Bar */}
        <div className="p-4 sm:p-5 rounded-2xl bg-white border border-slate-200/80 shadow-2xs flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <button
            type="button"
            onClick={() => setSelectedOrder(null)}
            className="inline-flex items-center gap-2 text-xs font-bold text-slate-600 hover:text-emerald-600 transition-colors cursor-pointer group w-fit"
          >
            <ArrowLeft className="w-4 h-4 transition-transform group-hover:-translate-x-1" />
            <span>BACK TO MY ORDERS</span>
          </button>

          <div className="flex items-center flex-wrap gap-2 sm:gap-3 text-xs">
            <span className="font-mono text-slate-400">ORDER ID:</span>
            <span className="font-extrabold text-[#0D1B2A] tracking-wide">
              #{selectedOrder.order_number || selectedOrder.uuid.slice(0, 8).toUpperCase()}
            </span>
            <span className="text-slate-300">|</span>
            {getPaymentStatusBadge(selectedOrder.payment_status)}
            {getOrderStatusBadge(selectedOrder.status)}
          </div>
        </div>

        {/* Shopee Stepper Progress Bar (ALL GREEN) */}
        <div className="p-5 sm:p-6 rounded-2xl bg-white border border-slate-200/80 shadow-2xs">
          {isCancelled ? (
            <div className="p-4 rounded-xl bg-rose-50 border border-rose-200 flex items-start gap-3 text-xs">
              <div className="w-9 h-9 rounded-full bg-rose-100 text-rose-600 flex items-center justify-center shrink-0 mt-0.5">
                <Ban className="w-5 h-5" />
              </div>
              <div className="space-y-1">
                <h3 className="font-extrabold text-rose-800 text-sm">Order Cancelled</h3>
                <p className="text-rose-700">
                  This order has been cancelled. Any pre-authorized charges will be refunded
                  according to your payment provider&apos;s timeline.
                </p>
                <p className="text-[11px] text-rose-500 font-medium pt-1">
                  Updated on: {formatDateTime(selectedOrder.created_at)}
                </p>
              </div>
            </div>
          ) : isReturned ? (
            <div className="p-4 rounded-xl bg-purple-50 border border-purple-200 flex items-start gap-3 text-xs">
              <div className="w-9 h-9 rounded-full bg-purple-100 text-purple-600 flex items-center justify-center shrink-0 mt-0.5">
                <RotateCcw className="w-5 h-5" />
              </div>
              <div className="space-y-1">
                <h3 className="font-extrabold text-purple-800 text-sm">
                  Return & Refund Under Review
                </h3>
                <p className="text-purple-700">
                  Your return request has been submitted and is currently being inspected by our
                  support specialists.
                </p>
              </div>
            </div>
          ) : (
            <div className="overflow-x-auto pb-2">
              <div className="min-w-[560px] sm:min-w-0">
                <div className="relative flex items-center justify-between">
                  {/* Inactive Background Track */}
                  <div className="absolute top-5 sm:top-6 -translate-y-1/2 left-[10%] right-[10%] h-1 bg-slate-100 z-0" />

                  {/* Active Process Line (GREEN) */}
                  <div
                    className="absolute top-5 sm:top-6 -translate-y-1/2 left-[10%] h-1 bg-emerald-500 transition-all duration-500 z-0"
                    style={{
                      width: `${
                        (Math.min(activeStepIndex, currentSteps.length - 1) /
                          (currentSteps.length - 1)) *
                        80
                      }%`,
                    }}
                  />

                  {/* Step Icons & Labels (ALL GREEN FOR ACTIVE/COMPLETED) */}
                  {currentSteps.map((step, idx) => {
                    const isCompleted = idx < activeStepIndex
                    const isCurrent = idx === activeStepIndex
                    const isReached = idx <= activeStepIndex
                    const timestamp = getStepTimestamp(idx, selectedOrder, timelineEvents)
                    const Icon = step.icon

                    return (
                      <div
                        key={step.key}
                        className="flex flex-col items-center text-center relative z-10 flex-1 px-1"
                      >
                        {/* Step Circle */}
                        <div
                          className={`w-10 h-10 sm:w-12 sm:h-12 rounded-full flex items-center justify-center transition-all duration-300 ${
                            isCompleted
                              ? 'bg-emerald-500 text-white shadow-md shadow-emerald-500/20'
                              : isCurrent
                              ? 'bg-emerald-600 text-white ring-4 ring-emerald-500/25 shadow-md shadow-emerald-500/30 animate-pulse-slow'
                              : 'bg-white border-2 border-slate-200 text-slate-400'
                          }`}
                        >
                          {isCompleted ? (
                            <CheckCheck className="w-5 h-5 sm:w-6 sm:h-6" />
                          ) : (
                            <Icon className="w-5 h-5 sm:w-6 sm:h-6" />
                          )}
                        </div>

                        {/* Step Label */}
                        <span
                          className={`mt-2.5 text-xs font-bold leading-tight ${
                            isReached ? 'text-emerald-700' : 'text-slate-400'
                          }`}
                        >
                          {step.label}
                        </span>

                        {/* Step Timestamp */}
                        {timestamp ? (
                          <span
                            className={`text-[10px] mt-1 font-mono leading-none ${
                              isReached ? 'text-emerald-600 font-medium' : 'text-slate-400'
                            }`}
                          >
                            {formatDateTime(timestamp)}
                          </span>
                        ) : (
                          <span className="text-[10px] text-slate-300 mt-1 font-mono leading-none">
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

          {/* Shopee Status Message Bar */}
          <div className="mt-5 pt-4 border-t border-slate-100 flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs">
            <div className="flex items-center gap-2 text-slate-600">
              <ShieldCheck className="w-4 h-4 text-emerald-600 shrink-0" />
              <span>
                {isDelivered
                  ? 'Parcel delivered successfully. You can request return/refund if needed.'
                  : activeStepIndex >= 2
                  ? 'Your parcel is handled by GHN Express and moving towards your destination.'
                  : 'Your order has been received and is being prepared for dispatch.'}
              </span>
            </div>

            <div className="flex items-center gap-2">
              {isPending && (
                <button
                  type="button"
                  onClick={() => handleOpenCancelConfirmation(selectedOrder)}
                  className="px-3 py-1.5 rounded-lg bg-rose-50 hover:bg-rose-100 text-rose-700 border border-rose-200 text-xs font-bold transition-all flex items-center gap-1.5 cursor-pointer"
                >
                  <Ban className="w-3.5 h-3.5" />
                  <span>Cancel Order</span>
                </button>
              )}

              {isDelivered && (
                <button
                  type="button"
                  onClick={() => handleOpenReturnModal(selectedOrder)}
                  className="px-3 py-1.5 rounded-lg bg-amber-50 hover:bg-amber-100 text-amber-700 border border-amber-200 text-xs font-bold transition-all flex items-center gap-1.5 cursor-pointer"
                >
                  <RotateCcw className="w-3.5 h-3.5" />
                  <span>Return / Refund</span>
                </button>
              )}
            </div>
          </div>
        </div>

        {/* ========================================================= */}
        {/* 1. ORDER TIMELINE & TRACKING HISTORY (ABOVE DELIVERY ADDR) */}
        {/* ========================================================= */}
        <div className="p-5 sm:p-6 rounded-2xl bg-white border border-slate-200/80 shadow-2xs space-y-5">
          <div className="flex items-center justify-between pb-3 border-b border-slate-100">
            <div className="flex items-center gap-2.5">
              <div className="w-8 h-8 rounded-lg bg-emerald-50 text-emerald-600 flex items-center justify-center">
                <Clock className="w-4 h-4" />
              </div>
              <h3 className="text-sm font-extrabold uppercase tracking-wider text-[#0D1B2A]">
                Order Timeline & Tracking History
              </h3>
            </div>
            <span className="text-xs text-slate-500">
              Carrier: <strong className="text-slate-800 font-semibold">GHN Logistics</strong>
            </span>
          </div>

          {isLoadingDetail ? (
            <div className="py-8 flex items-center justify-center gap-2 text-slate-400 text-sm">
              <div className="w-4 h-4 rounded-full border-2 border-emerald-500 border-t-transparent animate-spin" />
              <span>Loading tracking updates...</span>
            </div>
          ) : sortedEvents.length > 0 ? (
            <div className="space-y-0">
              {sortedEvents.map((ev, i) => {
                const isDelivered = isDeliveredEvent(ev)
                const isShipping = isShippingEvent(ev)
                const isLast = i === sortedEvents.length - 1

                return (
                  <div
                    key={ev.event_id || i}
                    className="flex items-start gap-3 sm:gap-4 relative"
                  >
                    {/* Left Column: Timestamp */}
                    <div className="w-28 sm:w-36 shrink-0 text-right pt-0.5">
                      <p
                        className={`text-xs sm:text-sm font-semibold font-mono leading-tight ${
                          isDelivered ? 'text-emerald-700' : 'text-slate-600'
                        }`}
                      >
                        {formatDateTime(ev.created_at)}
                      </p>
                    </div>

                    {/* Middle Column: Continuous Line + Exactly Centered Node */}
                    <div className="relative flex flex-col items-center shrink-0 w-8 self-stretch">
                      {sortedEvents.length > 1 && (
                        <div
                          className={`absolute left-1/2 -translate-x-1/2 w-0.5 bg-slate-200 ${
                            i === 0
                              ? 'top-3 bottom-0'
                              : isLast
                              ? 'top-0 h-3'
                              : 'top-0 bottom-0'
                          }`}
                        />
                      )}

                      {/* Node circle on top of line */}
                      <div className="relative z-10 bg-white">
                        {isDelivered ? (
                          <div className="w-6 h-6 rounded-full bg-emerald-500 text-white flex items-center justify-center shadow-xs">
                            <CheckCheck className="w-3.5 h-3.5" />
                          </div>
                        ) : isShipping ? (
                          <div className="w-6 h-6 rounded-full bg-white border-2 border-slate-400 text-slate-700 flex items-center justify-center shadow-xs">
                            <Truck className="w-3.5 h-3.5" />
                          </div>
                        ) : (
                          <div className="w-6 h-6 flex items-center justify-center">
                            <div className="w-3 h-3 rounded-full bg-slate-300 border-2 border-white ring-1 ring-slate-200" />
                          </div>
                        )}
                      </div>
                    </div>

                    {/* Right Column: Event Content */}
                    <div className={`flex-1 min-w-0 pt-0.5 ${isLast ? 'pb-1' : 'pb-6'}`}>
                      <h4
                        className={`text-sm sm:text-base font-bold leading-snug ${
                          isDelivered ? 'text-emerald-700' : 'text-slate-900'
                        }`}
                      >
                        {ev.status_text || ev.status.toUpperCase()}
                      </h4>
                      {ev.note && (
                        <p
                          className={`text-xs sm:text-sm mt-1 leading-relaxed ${
                            isDelivered ? 'text-emerald-600' : 'text-slate-600'
                          }`}
                        >
                          {ev.note}
                        </p>
                      )}
                    </div>
                  </div>
                )
              })}
            </div>
          ) : (
            <div className="text-sm text-slate-600 py-3 flex items-center gap-3">
              <CheckCircle2 className="w-5 h-5 text-emerald-500 shrink-0" />
              <span>
                Order was successfully created on{' '}
                <strong>{formatDateTime(selectedOrder.created_at)}</strong>. Awaiting logistics
                dispatch.
              </span>
            </div>
          )}
        </div>

        {/* ========================================================= */}
        {/* 2. DELIVERY ADDRESS & CARRIER CARD (BELOW TIMELINE)       */}
        {/* ========================================================= */}
        <div className="p-5 rounded-2xl bg-white border border-slate-200/80 shadow-2xs space-y-3 text-xs">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 pb-3 border-b border-slate-100">
            <h3 className="font-extrabold uppercase tracking-wider text-[#0D1B2A] text-xs flex items-center gap-2">
              <MapPin className="w-4 h-4 text-emerald-600" />
              Delivery Address
            </h3>
            <span className="px-2.5 py-1 rounded-full bg-blue-50 text-[#0D47A1] text-[11px] font-bold border border-blue-100 flex items-center gap-1.5 w-fit">
              <Truck className="w-3.5 h-3.5" />
              GHN Express (Standard Delivery)
            </span>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4 pt-1">
            <div className="space-y-1.5">
              <div className="flex items-center gap-2 text-sm">
                <span className="font-bold text-slate-900">
                  {selectedOrder.shipping_name || 'Customer'}
                </span>
                <span className="text-slate-300">|</span>
                <span className="font-medium text-slate-600">
                  {selectedOrder.shipping_phone || '—'}
                </span>
              </div>
              <p className="text-slate-600 leading-relaxed text-xs">
                {selectedOrder.shipping_address || 'No shipping address provided'}
              </p>
            </div>

            <div className="space-y-2 md:border-l md:border-slate-100 md:pl-4">
              <div>
                <span className="text-slate-400 text-[11px] block">Payment Method</span>
                <span className="font-bold text-slate-800 flex items-center gap-1.5 mt-0.5">
                  <CreditCard className="w-3.5 h-3.5 text-slate-500" />
                  {selectedOrder.payment_method || 'Cash on Delivery (COD)'}
                </span>
              </div>
              {selectedOrder.note && (
                <div>
                  <span className="text-slate-400 text-[11px] block">Order Note</span>
                  <p className="text-slate-600 text-[11px] italic mt-0.5">
                    &ldquo;{selectedOrder.note}&rdquo;
                  </p>
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Shopee Purchased Items Table */}
        <div className="rounded-2xl bg-white border border-slate-200/80 overflow-hidden shadow-2xs">
          <div className="px-5 py-3.5 bg-slate-50/70 border-b border-slate-100 flex items-center justify-between text-xs">
            <span className="font-extrabold uppercase tracking-wider text-[#0D1B2A] flex items-center gap-2">
              <Package className="w-4 h-4 text-emerald-600" />
              Order Items ({selectedOrder.items?.length || 0})
            </span>
            <span className="text-slate-500 text-[11px] font-medium flex items-center gap-1">
              <ShieldCheck className="w-3.5 h-3.5 text-emerald-600" />
              100% Genuine Guarantee
            </span>
          </div>

          {isLoadingDetail ? (
            <div className="p-8 text-center text-xs text-slate-400 flex items-center justify-center gap-2">
              <div className="w-4 h-4 rounded-full border-2 border-emerald-500 border-t-transparent animate-spin" />
              <span>Loading item details...</span>
            </div>
          ) : selectedOrder.items && selectedOrder.items.length > 0 ? (
            <div className="divide-y divide-slate-100">
              {selectedOrder.items.map((item, idx) => (
                <div
                  key={item.uuid || idx}
                  className="p-4 sm:p-5 flex flex-col sm:flex-row sm:items-center justify-between gap-4 text-xs hover:bg-slate-50/40 transition-colors"
                >
                  <div className="flex items-center gap-3.5 min-w-0">
                    <div className="w-16 h-16 rounded-xl bg-slate-100 overflow-hidden shrink-0 border border-slate-200/60 flex items-center justify-center">
                      {item.image_url ? (
                        <img
                          src={getImageUrl(item.image_url)}
                          alt={item.product_name}
                          className="w-full h-full object-cover"
                        />
                      ) : (
                        <Package className="w-6 h-6 text-slate-400" />
                      )}
                    </div>
                    <div className="min-w-0 space-y-1">
                      <h4 className="font-bold text-slate-900 text-sm line-clamp-1 hover:text-emerald-600 transition-colors">
                        {item.product_name}
                      </h4>
                      <p className="text-[11px] text-slate-500">
                        Quantity:{' '}
                        <strong className="text-slate-800 font-semibold">{item.quantity}</strong>
                      </p>
                      <p className="text-[11px] text-slate-400">
                        Unit Price: {formatCurrency(item.price)}
                      </p>
                    </div>
                  </div>

                  <div className="flex sm:flex-col items-center sm:items-end justify-between sm:justify-center gap-1 shrink-0 pt-2 sm:pt-0 border-t sm:border-t-0 border-slate-100">
                    <span className="text-slate-400 text-[11px] sm:hidden">Item Subtotal:</span>
                    <span className="text-sm font-extrabold text-[#0D47A1]">
                      {formatCurrency(item.subtotal || item.price * item.quantity)}
                    </span>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="p-8 text-center text-xs text-slate-400">
              No item details found for this order.
            </div>
          )}

          {/* Financial Summary */}
          <div className="p-5 bg-slate-50/70 border-t border-slate-100 space-y-2 text-xs">
            <div className="max-w-md ml-auto space-y-2">
              <div className="flex justify-between text-slate-600">
                <span>Merchandise Subtotal</span>
                <span>
                  {formatCurrency(selectedOrder.subtotal_amount || selectedOrder.total_amount)}
                </span>
              </div>

              {selectedOrder.discount_amount !== undefined && selectedOrder.discount_amount > 0 && (
                <div className="flex justify-between text-emerald-600 font-medium">
                  <span className="flex items-center gap-1">
                    <Tag className="w-3.5 h-3.5" />
                    Voucher Discount{' '}
                    {selectedOrder.coupon_code ? `(${selectedOrder.coupon_code})` : ''}
                  </span>
                  <span>-{formatCurrency(selectedOrder.discount_amount)}</span>
                </div>
              )}

              {selectedOrder.shipping_fee !== undefined && (
                <div className="flex justify-between text-slate-600">
                  <span>Shipping Fee (GHN Express)</span>
                  <span>{formatCurrency(selectedOrder.shipping_fee)}</span>
                </div>
              )}

              <div className="pt-3 border-t border-slate-200 flex justify-between items-baseline">
                <div>
                  <span className="text-sm font-extrabold text-[#0D1B2A] block">
                    Order Total
                  </span>
                  <span className="text-[11px] text-slate-400">
                    Includes VAT & handling fees
                  </span>
                </div>
                <span className="text-xl sm:text-2xl font-black text-[#0D47A1]">
                  {formatCurrency(selectedOrder.total_amount)}
                </span>
              </div>
            </div>
          </div>
        </div>

        {/* Bottom Action Footer */}
        <div className="p-4 sm:p-5 rounded-2xl bg-white border border-slate-200/80 shadow-2xs flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <button
            type="button"
            onClick={() => setSelectedOrder(null)}
            className="inline-flex items-center justify-center gap-2 px-4 py-2.5 rounded-xl border border-slate-200 bg-white hover:bg-slate-50 text-slate-700 text-xs font-bold transition-all cursor-pointer shadow-2xs"
          >
            <ArrowLeft className="w-4 h-4" />
            <span>Back to Orders</span>
          </button>

          <div className="flex items-center gap-2.5 flex-wrap justify-end">
            {isPending && (
              <button
                type="button"
                onClick={() => handleOpenCancelConfirmation(selectedOrder)}
                className="px-5 py-2.5 rounded-xl bg-rose-50 hover:bg-rose-100 text-rose-700 border border-rose-200 text-xs font-bold transition-all flex items-center gap-1.5 cursor-pointer"
              >
                <Ban className="w-4 h-4" />
                <span>Cancel Order</span>
              </button>
            )}

            {isDelivered && (
              <button
                type="button"
                onClick={() => handleOpenReturnModal(selectedOrder)}
                className="px-5 py-2.5 rounded-xl bg-amber-50 hover:bg-amber-100 text-amber-700 border border-amber-200 text-xs font-bold transition-all flex items-center gap-1.5 cursor-pointer"
              >
                <RotateCcw className="w-4 h-4" />
                <span>Return / Refund</span>
              </button>
            )}

            <Link
              href="/products"
              className="inline-flex items-center justify-center gap-1.5 px-5 py-2.5 rounded-xl bg-[#2196F3] hover:bg-[#0D47A1] text-white text-xs font-bold transition-all shadow-xs"
            >
              <span>Explore Products</span>
              <ArrowRight className="w-4 h-4" />
            </Link>
          </div>
        </div>

        {/* Render confirmation modals if triggered */}
        {renderActionModals()}
      </div>
    )
  }

  // ==========================================
  // VIEW 2: ORDERS HISTORY LIST VIEW
  // ==========================================
  return (
    <div className="p-5 sm:p-6 rounded-3xl bg-white border border-slate-200/80 shadow-xs space-y-6">
      <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-4 pb-2 border-b border-slate-100">
        <div>
          <h1 className="text-xl font-extrabold text-[#0D1B2A] tracking-tight">
            Order History
          </h1>
          <p className="text-xs text-slate-500 mt-1">
            Track and review all purchases, delivery updates, and invoices. Click any order to view details.
          </p>
        </div>

        {/* Search & Status Filter Controls */}
        <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-2.5">
          {/* Search Input Bar */}
          <div className="relative flex-1 sm:w-64 md:w-72">
            <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none text-slate-400">
              <Search className="w-3.5 h-3.5" />
            </div>
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search by order #, product, or recipient..."
              className="w-full pl-8.5 pr-8 py-2 rounded-xl bg-white border border-slate-200 text-xs font-medium text-[#0D1B2A] placeholder:text-slate-400 focus:outline-none focus:border-[#2196F3] shadow-2xs transition-all"
            />
            {searchQuery && (
              <button
                type="button"
                onClick={() => setSearchQuery('')}
                className="absolute inset-y-0 right-0 pr-2.5 flex items-center text-slate-400 hover:text-slate-600"
                title="Clear search"
              >
                <X className="w-3.5 h-3.5" />
              </button>
            )}
          </div>

          {/* Status Filter Dropdown */}
          <div className="relative">
            <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none text-slate-400">
              <Filter className="w-3.5 h-3.5 text-[#2196F3]" />
            </div>
            <select
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              className="w-full sm:w-auto pl-8.5 pr-9 py-2 rounded-xl bg-white border border-slate-200 text-xs font-semibold text-[#0D1B2A] focus:outline-none focus:border-[#2196F3] shadow-2xs appearance-none cursor-pointer hover:border-slate-300 transition-all"
            >
              <option value="ALL">All Orders ({orders.length})</option>
              <option value="PENDING">Pending</option>
              <option value="CONFIRMED">Confirmed</option>
              <option value="SHIPPED">Shipped</option>
              <option value="DELIVERED">Delivered</option>
              <option value="CANCELLED">Cancelled</option>
              <option value="RETURNED">Returned</option>
            </select>
            <div className="absolute inset-y-0 right-0 pr-3 flex items-center pointer-events-none text-slate-400">
              <ChevronDown className="w-3.5 h-3.5" />
            </div>
          </div>

          {(filter !== 'ALL' || searchQuery.trim() !== '') && (
            <button
              type="button"
              onClick={() => {
                setFilter('ALL')
                setSearchQuery('')
              }}
              className="px-2.5 py-2 rounded-xl text-xs font-medium text-slate-500 hover:text-slate-800 hover:bg-slate-100 transition-colors cursor-pointer text-center"
            >
              Reset
            </button>
          )}
        </div>
      </div>

      {/* Orders List */}
      {filteredOrders.length === 0 ? (
        <div className="p-12 text-center border border-dashed border-slate-200 rounded-2xl space-y-3">
          <div className="w-14 h-14 rounded-full bg-[#E3F2FD] text-[#2196F3] flex items-center justify-center mx-auto">
            <ShoppingBag className="w-7 h-7" />
          </div>
          <h3 className="text-sm font-bold text-[#0D1B2A]">No orders found</h3>
          <p className="text-xs text-slate-500 max-w-sm mx-auto">
            {orders.length === 0
              ? "You haven't placed any orders yet."
              : searchQuery.trim()
              ? `No orders matching "${searchQuery.trim()}".`
              : "You haven't placed any orders matching this filter yet."}
          </p>
          <div className="pt-2 flex items-center justify-center gap-2">
            {orders.length > 0 && (filter !== 'ALL' || searchQuery.trim()) ? (
              <button
                type="button"
                onClick={() => {
                  setFilter('ALL')
                  setSearchQuery('')
                }}
                className="inline-flex items-center gap-1.5 px-4 py-2 rounded-xl bg-slate-100 text-slate-700 text-xs font-bold hover:bg-slate-200 transition-colors cursor-pointer"
              >
                <span>Clear Filters</span>
              </button>
            ) : (
              <Link
                href="/products"
                className="inline-flex items-center gap-1.5 px-4 py-2 rounded-xl bg-[#2196F3] text-white text-xs font-bold hover:bg-[#0D47A1] transition-colors"
              >
                <span>Explore Catalog</span>
                <ArrowRight className="w-3.5 h-3.5" />
              </Link>
            )}
          </div>
        </div>
      ) : (
        <div className="space-y-4">
          {filteredOrders.map((order) => {
            const isPending = order.status?.toUpperCase() === 'PENDING'
            const isDelivered = order.status?.toUpperCase() === 'DELIVERED'

            return (
              <div
                key={order.uuid}
                onClick={() => handleOpenOrderDetails(order)}
                className="p-5 rounded-2xl border border-slate-200/80 bg-slate-50/40 hover:bg-slate-50 hover:border-[#2196F3]/50 hover:shadow-xs transition-all space-y-3 cursor-pointer group"
              >
                {/* Order Header: Order #, Date, and Badges */}
                <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 pb-3 border-b border-slate-200/60 text-xs">
                  <div className="flex items-center flex-wrap gap-2">
                    <span className="text-slate-400 font-mono">ORDER #</span>
                    <span className="font-bold text-[#0D1B2A]">
                      {order.order_number || order.uuid.slice(0, 8).toUpperCase()}
                    </span>
                    <span className="text-slate-300 mx-1">•</span>
                    <span className="text-slate-500">
                      {formatDate(order.created_at)}
                    </span>
                  </div>

                  {/* Badges: Payment Status & Order Status */}
                  <div className="flex items-center gap-2 flex-wrap">
                    {getPaymentStatusBadge(order.payment_status)}
                    {getOrderStatusBadge(order.status)}
                  </div>
                </div>

                {/* Order Content Summary */}
                <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs">
                  <div className="space-y-1">
                    <p className="text-slate-700 font-medium">
                      {order.items && order.items.length > 0
                        ? `${order.items[0].product_name} ${
                            order.items.length > 1
                              ? `+ ${order.items.length - 1} other item(s)`
                              : ''
                          }`
                        : 'Standard shipment'}
                    </p>
                    {order.shipping_address && (
                      <p className="text-[11px] text-slate-400 truncate max-w-md flex items-center gap-1">
                        <MapPin className="w-3 h-3 text-slate-400 shrink-0" />
                        <span className="truncate">{order.shipping_address}</span>
                      </p>
                    )}
                    {order.payment_method && (
                      <p className="text-[11px] text-slate-500 flex items-center gap-1">
                        <CreditCard className="w-3 h-3 text-slate-400 shrink-0" />
                        <span>
                          Payment Method:{' '}
                          <strong className="text-slate-700">{order.payment_method}</strong>
                        </span>
                      </p>
                    )}
                  </div>

                  <div className="flex flex-col sm:items-end justify-between sm:justify-center gap-2.5 shrink-0">
                    <div className="text-right">
                      <span className="text-slate-500 text-[11px] block">
                        Total Amount
                      </span>
                      <span className="text-base font-extrabold text-[#0D47A1]">
                        {formatCurrency(order.total_amount)}
                      </span>
                    </div>

                    {/* Action Buttons */}
                    <div className="flex items-center gap-2 flex-wrap sm:justify-end">
                      {/* Cancel Button (if Pending) */}
                      {isPending && (
                        <button
                          type="button"
                          onClick={(e) => {
                            e.stopPropagation()
                            handleOpenCancelConfirmation(order)
                          }}
                          className="px-2.5 py-1 rounded-lg bg-rose-50 hover:bg-rose-100 text-rose-700 border border-rose-200 text-xs font-semibold transition-all flex items-center gap-1 cursor-pointer"
                          title="Cancel this order"
                        >
                          <Ban className="w-3.5 h-3.5" />
                          <span>Cancel Order</span>
                        </button>
                      )}

                      {/* Return / Refund Button (if Delivered) */}
                      {isDelivered && (
                        <button
                          type="button"
                          onClick={(e) => {
                            e.stopPropagation()
                            handleOpenReturnModal(order)
                          }}
                          className="px-2.5 py-1 rounded-lg bg-amber-50 hover:bg-amber-100 text-amber-700 border border-amber-200 text-xs font-semibold transition-all flex items-center gap-1 cursor-pointer"
                          title="Request return or refund"
                        >
                          <RotateCcw className="w-3.5 h-3.5" />
                          <span>Return / Refund</span>
                        </button>
                      )}

                      {/* View Details Button */}
                      <button
                        type="button"
                        className="inline-flex items-center gap-1 text-xs font-bold text-[#2196F3] group-hover:text-[#0D47A1] transition-colors py-1"
                      >
                        <span>View Details</span>
                        <Eye className="w-3.5 h-3.5 transition-transform group-hover:scale-110" />
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      )}

      {renderActionModals()}
    </div>
  )

  // ==========================================
  // ACTION MODALS (CANCEL & RETURN / REFUND)
  // ==========================================
  function renderActionModals() {
    return (
      <>
        {/* Cancel Confirmation Dialog */}
        {orderToCancel && (
          <div className="fixed inset-0 z-60 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 overflow-y-auto animate-in fade-in">
            <div className="w-full max-w-md bg-white rounded-3xl p-6 shadow-2xl space-y-4 border border-slate-100">
              <div className="w-12 h-12 rounded-full bg-rose-50 text-rose-500 flex items-center justify-center mx-auto">
                <AlertTriangle className="w-6 h-6" />
              </div>

              <div className="text-center space-y-1">
                <h3 className="text-base font-extrabold text-[#0D1B2A]">
                  Cancel Order Confirmation
                </h3>
                <p className="text-xs text-slate-500">
                  Are you sure you want to cancel order{' '}
                  <strong className="text-slate-800">
                    #{orderToCancel.order_number || orderToCancel.uuid.slice(0, 8).toUpperCase()}
                  </strong>
                  ? This action cannot be undone.
                </p>
              </div>

              <div className="flex gap-2 pt-2">
                <button
                  type="button"
                  disabled={isCancelling}
                  onClick={() => setOrderToCancel(null)}
                  className="flex-1 py-2.5 rounded-xl bg-slate-100 hover:bg-slate-200 text-slate-700 text-xs font-bold transition-all cursor-pointer"
                >
                  Keep Order
                </button>
                <button
                  type="button"
                  disabled={isCancelling}
                  onClick={handleConfirmCancel}
                  className="flex-1 py-2.5 rounded-xl bg-rose-600 hover:bg-rose-700 text-white text-xs font-bold transition-all shadow-xs flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-50"
                >
                  {isCancelling ? (
                    <>
                      <div className="w-3.5 h-3.5 rounded-full border-2 border-white border-t-transparent animate-spin" />
                      <span>Cancelling...</span>
                    </>
                  ) : (
                    <>
                      <Ban className="w-3.5 h-3.5" />
                      <span>Yes, Cancel</span>
                    </>
                  )}
                </button>
              </div>
            </div>
          </div>
        )}

        {/* Return / Refund Request Modal */}
        {orderToReturn && (
          <div className="fixed inset-0 z-60 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 overflow-y-auto animate-in fade-in">
            <div className="w-full max-w-lg bg-white rounded-3xl p-6 sm:p-7 shadow-2xl space-y-5 border border-slate-100 my-8">
              <div className="flex items-start justify-between gap-2 border-b border-slate-100 pb-3">
                <div>
                  <h3 className="text-base font-extrabold text-[#0D1B2A] flex items-center gap-1.5">
                    <RotateCcw className="w-4 h-4 text-[#2196F3]" />
                    Request Return & Refund
                  </h3>
                  <p className="text-xs text-slate-500 mt-0.5">
                    Order #{orderToReturn.order_number || orderToReturn.uuid.slice(0, 8).toUpperCase()}
                  </p>
                </div>

                <button
                  onClick={() => setOrderToReturn(null)}
                  className="w-7 h-7 rounded-full bg-slate-100 hover:bg-slate-200 text-slate-500 flex items-center justify-center transition-colors cursor-pointer"
                >
                  <X className="w-3.5 h-3.5" />
                </button>
              </div>

              <form onSubmit={handleSubmitReturn} className="space-y-4 text-xs">
                {/* Reason Field */}
                <div className="space-y-1.5">
                  <label className="font-bold text-slate-700 block">
                    Reason for Return <span className="text-rose-500">*</span>
                  </label>
                  <textarea
                    required
                    rows={3}
                    value={returnReason}
                    onChange={(e) => setReturnReason(e.target.value)}
                    placeholder="Please describe in detail why you want to return this order (minimum 10 characters)..."
                    className="w-full p-3 rounded-xl border border-slate-200 focus:outline-none focus:border-[#2196F3] text-xs text-[#0D1B2A] resize-none"
                  />
                  <span className="text-[11px] text-slate-400">
                    {returnReason.trim().length} / 10 characters minimum
                  </span>
                </div>

                {/* Bank Details for Refund (Optional for COD) */}
                <div className="p-3.5 rounded-2xl bg-slate-50 border border-slate-200/70 space-y-2.5">
                  <span className="font-bold text-slate-700 flex items-center gap-1.5 text-[11px] uppercase tracking-wider">
                    <Building2 className="w-3.5 h-3.5 text-slate-500" />
                    Bank Account for Refund (Optional)
                  </span>
                  <p className="text-[11px] text-slate-500">
                    Provide your bank information if you paid with Cash on Delivery (COD) to receive your refund.
                  </p>

                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 pt-1">
                    <div className="space-y-1">
                      <label className="text-[11px] font-medium text-slate-600">Bank Name</label>
                      <input
                        type="text"
                        value={bankName}
                        onChange={(e) => setBankName(e.target.value)}
                        placeholder="e.g. Vietcombank, MBBank"
                        className="w-full px-2.5 py-1.5 rounded-lg border border-slate-200 bg-white text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3]"
                      />
                    </div>

                    <div className="space-y-1">
                      <label className="text-[11px] font-medium text-slate-600">Account Number</label>
                      <input
                        type="text"
                        value={bankAccount}
                        onChange={(e) => setBankAccount(e.target.value)}
                        placeholder="e.g. 1029384756"
                        className="w-full px-2.5 py-1.5 rounded-lg border border-slate-200 bg-white text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3]"
                      />
                    </div>
                  </div>

                  <div className="space-y-1">
                    <label className="text-[11px] font-medium text-slate-600">Account Holder Name</label>
                    <input
                      type="text"
                      value={bankAccountName}
                      onChange={(e) => setBankAccountName(e.target.value)}
                      placeholder="e.g. NGUYEN VAN A"
                      className="w-full px-2.5 py-1.5 rounded-lg border border-slate-200 bg-white text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3] uppercase"
                    />
                  </div>
                </div>

                {/* Form Action Buttons */}
                <div className="flex justify-end gap-2 pt-2 border-t border-slate-100">
                  <button
                    type="button"
                    disabled={isSubmittingReturn}
                    onClick={() => setOrderToReturn(null)}
                    className="px-4 py-2 rounded-xl bg-slate-100 hover:bg-slate-200 text-slate-700 text-xs font-bold transition-all cursor-pointer"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    disabled={isSubmittingReturn || returnReason.trim().length < 10}
                    className="px-5 py-2 rounded-xl bg-[#2196F3] hover:bg-[#0D47A1] text-white text-xs font-bold transition-all shadow-xs flex items-center gap-1.5 cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {isSubmittingReturn ? (
                      <>
                        <div className="w-3.5 h-3.5 rounded-full border-2 border-white border-t-transparent animate-spin" />
                        <span>Submitting...</span>
                      </>
                    ) : (
                      <>
                        <RotateCcw className="w-3.5 h-3.5" />
                        <span>Submit Return Request</span>
                      </>
                    )}
                  </button>
                </div>
              </form>
            </div>
          </div>
        )}
      </>
    )
  }
}
