'use client'

import React, { useState, useEffect, useCallback, useRef, Suspense } from 'react'
import Link from 'next/link'
import { useRouter, useSearchParams } from 'next/navigation'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { authFetch } from '@/lib/api'
import { formatCurrency, formatDate, formatDateTime } from '@/lib/format'
import { getImageUrl, extractErrorMessage } from '@/lib/utils'
import { WriteReviewModal } from '@/components/review/write-review-modal'
import { RequestReturnModal } from '@/components/order/request-return-modal'
import { ReturnDetailModal } from '@/components/order/return-detail-modal'
import { OrderTrackingModal } from '@/components/order/order-tracking-modal'
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
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  Download,
} from 'lucide-react'

interface OrderItem {
  uuid?: string
  order_item_id?: number
  product_id?: number
  product_uuid?: string
  product_name: string
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

function MyOrdersPageContent() {
  const { accessToken } = useAuthStore()
  const [orders, setOrders] = useState<Order[]>([])
  const [loading, setLoading] = useState(true)
  const [filter, setFilter] = useState('ALL')
  const [searchInput, setSearchInput] = useState('')
  const [debouncedSearch, setDebouncedSearch] = useState('')
  const [isSearching, setIsSearching] = useState(false)

  // Pagination State
  const [currentPage, setCurrentPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [pagination, setPagination] = useState({
    page: 1,
    limit: 10,
    total_records: 0,
    total_pages: 1,
    has_next: false,
    has_prev: false,
  })

  // 300ms Debounce on search input
  useEffect(() => {
    if (searchInput !== debouncedSearch) {
      setIsSearching(true)
    }
    const timer = setTimeout(() => {
      setDebouncedSearch(searchInput)
      setIsSearching(false)
    }, 300)
    return () => clearTimeout(timer)
  }, [searchInput, debouncedSearch])

  // Reset to page 1 on filter, search keyword, or page size change
  useEffect(() => {
    setCurrentPage(1)
  }, [filter, debouncedSearch, pageSize])

  // Selected Order for in-place Detail View
  const [selectedOrder, setSelectedOrder] = useState<Order | null>(null)
  const [timelineEvents, setTimelineEvents] = useState<TrackingEvent[]>([])
  const [isLoadingDetail, setIsLoadingDetail] = useState(false)

  // Cancel Order Modal State
  const [orderToCancel, setOrderToCancel] = useState<Order | null>(null)
  const [isCancelling, setIsCancelling] = useState(false)

  // Return / Refund Modal State
  const [orderToReturn, setOrderToReturn] = useState<Order | null>(null)
  const [orderToViewReturn, setOrderToViewReturn] = useState<Order | null>(null)

  // Live Tracking Modal State
  const [orderToTrack, setOrderToTrack] = useState<Order | null>(null)

  // Confirm Delivery Modal State
  const [orderToConfirmDelivery, setOrderToConfirmDelivery] = useState<Order | null>(null)
  const [isConfirmingDelivery, setIsConfirmingDelivery] = useState(false)

  // Invoice Download State
  const [downloadingInvoiceUuid, setDownloadingInvoiceUuid] = useState<string | null>(null)

  // Write Review Modal State
  const [reviewingItem, setReviewingItem] = useState<{
    orderUuid: string
    productUuid: string
    productName: string
    productImageUrl?: string
  } | null>(null)

  // Sync Tracking State
  const [isSyncingTracking, setIsSyncingTracking] = useState(false)

  const fetchOrders = useCallback(async () => {
    if (!accessToken) return
    setLoading(true)
    try {
      const baseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
      const params = new URLSearchParams()
      params.set('page', String(currentPage))
      params.set('limit', String(pageSize))

      if (debouncedSearch.trim()) {
        params.set('search', debouncedSearch.trim())
      }

      const statusMap: Record<string, number> = {
        PENDING: 1,
        CONFIRMED: 2,
        SHIPPED: 3,
        DELIVERED: 4,
        CANCELLED: 5,
        RETURNED: 6,
      }
      if (filter !== 'ALL' && statusMap[filter]) {
        params.set('status', String(statusMap[filter]))
      }

      const res = await authFetch(`${baseUrl}/api/v1/orders?${params.toString()}`)
      if (res.ok) {
        const json = await res.json()
        const orderList: Order[] = Array.isArray(json?.data)
          ? json.data
          : Array.isArray(json?.data?.data)
          ? json.data.data
          : Array.isArray(json?.data?.items)
          ? json.data.items
          : []
        setOrders(orderList)

        const paginationData = json?.pagination || json?.data?.pagination
        if (paginationData) {
          setPagination(paginationData)
        } else {
          setPagination({
            page: currentPage,
            limit: pageSize,
            total_records: orderList.length,
            total_pages: Math.max(1, Math.ceil(orderList.length / pageSize)),
            has_next: false,
            has_prev: false,
          })
        }
      } else {
        setOrders([])
      }
    } catch {
      setOrders([])
    } finally {
      setLoading(false)
    }
  }, [accessToken, currentPage, pageSize, debouncedSearch, filter])

  useEffect(() => {
    fetchOrders()
  }, [fetchOrders])

  const router = useRouter()
  // URL Parameter Handling for Direct Link / Reload persistence and dynamic navigation
  const searchParams = useSearchParams()
  const urlOrderId = searchParams.get('orderId') || searchParams.get('order')
  const activeOrderRef = useRef<string | null>(null)

  // In-place switch to order detail with URL synchronization (without reload)
  const handleOpenOrderDetails = useCallback(async (order: Order, updateUrl = true) => {
    setSelectedOrder(order)
    setTimelineEvents([])
    setIsLoadingDetail(true)

    const ref = order.order_number || order.uuid
    activeOrderRef.current = ref

    if (updateUrl) {
      router.push(`/account/orders?orderId=${encodeURIComponent(ref)}`, { scroll: false })
    }

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
  }, [router])

  const handleBackToOrdersList = () => {
    setSelectedOrder(null)
    activeOrderRef.current = null
    router.push('/account/orders', { scroll: false })
  }


  // Automatically sync & open order detail whenever URL searchParams orderId changes (reload, direct link, or notification click)
  useEffect(() => {
    if (!accessToken) return

    if (!urlOrderId) {
      if (activeOrderRef.current !== null || selectedOrder !== null) {
        activeOrderRef.current = null
        setSelectedOrder(null)
      }
      return
    }

    // If currently displaying this exact order, skip re-fetching
    if (
      activeOrderRef.current === urlOrderId ||
      (selectedOrder && (selectedOrder.order_number === urlOrderId || selectedOrder.uuid === urlOrderId))
    ) {
      return
    }

    // 1. Check in already loaded orders
    const matched = orders.find(
      (o) => o.order_number === urlOrderId || o.uuid === urlOrderId
    )
    if (matched) {
      handleOpenOrderDetails(matched, false)
      return
    }

    // 2. If not found in loaded orders, query API directly
    let isCancelled = false
    const fetchDirectOrder = async () => {
      setIsLoadingDetail(true)
      try {
        const baseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
        const isUuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(urlOrderId)
        if (isUuid) {
          const res = await authFetch(`${baseUrl}/api/v1/orders/${urlOrderId}`)
          if (res.ok) {
            const json = await res.json()
            if (json?.data && !isCancelled) {
              handleOpenOrderDetails(json.data, false)
              return
            }
          }
        } else {
          const res = await authFetch(`${baseUrl}/api/v1/orders?search=${encodeURIComponent(urlOrderId)}`)
          if (res.ok) {
            const json = await res.json()
            const orderList: Order[] = Array.isArray(json?.data)
              ? json.data
              : Array.isArray(json?.data?.items)
              ? json.data.items
              : []
            const found = orderList.find(
              (o) => o.order_number === urlOrderId || o.uuid === urlOrderId
            )
            if (found && !isCancelled) {
              handleOpenOrderDetails(found, false)
              return
            }
          }
        }
      } catch {
        // ignore
      } finally {
        if (!isCancelled) {
          setIsLoadingDetail(false)
        }
      }
    }

    fetchDirectOrder()

    return () => {
      isCancelled = true
    }
  }, [urlOrderId, orders, accessToken, handleOpenOrderDetails, selectedOrder])

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
  }

  const handleOpenReturnDetails = (order: Order) => {
    setOrderToViewReturn(order)
  }

  const handleOpenTrackingModal = (order: Order) => {
    setOrderToTrack(order)
  }

  // --- CONFIRM DELIVERY ACTIONS ---
  const handleOpenConfirmDelivery = (order: Order) => {
    setOrderToConfirmDelivery(order)
  }

  const handleConfirmDelivery = async () => {
    if (!orderToConfirmDelivery) return
    setIsConfirmingDelivery(true)

    try {
      const baseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
      const res = await authFetch(
        `${baseUrl}/api/v1/orders/${orderToConfirmDelivery.uuid}/confirm-delivery`,
        {
          method: 'POST',
        }
      )

      if (res.ok) {
        toast.success(
          `Order #${orderToConfirmDelivery.order_number || orderToConfirmDelivery.uuid.slice(0, 8).toUpperCase()} delivery confirmed! Thank you for shopping with GOshop.`
        )
        setOrderToConfirmDelivery(null)
        if (selectedOrder && selectedOrder.uuid === orderToConfirmDelivery.uuid) {
          setSelectedOrder({ ...selectedOrder, status: 'Delivered' })
        }
        await fetchOrders()
      } else {
        const errJson = await res.json().catch(() => null)
        const msg = extractErrorMessage(errJson, 'Failed to confirm order delivery.')
        toast.error(msg)
      }
    } catch {
      toast.error('Network error occurred while confirming delivery.')
    } finally {
      setIsConfirmingDelivery(false)
    }
  }

  // --- DOWNLOAD INVOICE ACTIONS ---
  const handleDownloadInvoice = async (order: Order) => {
    if (downloadingInvoiceUuid) return
    setDownloadingInvoiceUuid(order.uuid)

    try {
      const baseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
      const res = await authFetch(`${baseUrl}/api/v1/orders/${order.uuid}/invoice`)

      if (!res.ok) {
        const errJson = await res.json().catch(() => null)
        const msg = extractErrorMessage(errJson, 'Invoice is not available yet.')
        toast.error(msg)
        return
      }

      const blob = await res.blob()
      const url = window.URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `Invoice_${order.order_number || order.uuid.slice(0, 8).toUpperCase()}.pdf`
      document.body.appendChild(a)
      a.click()
      a.remove()
      window.URL.revokeObjectURL(url)

      toast.success('Invoice downloaded successfully!')
    } catch {
      toast.error('Failed to download invoice PDF.')
    } finally {
      setDownloadingInvoiceUuid(null)
    }
  }

  // --- WRITE REVIEW ACTIONS ---
  const handleOpenReviewItem = (order: Order, item: OrderItem) => {
    const pUuid = item.product_uuid || item.uuid
    if (!pUuid) {
      toast.error('Product details are not available for this item.')
      return
    }
    setReviewingItem({
      orderUuid: order.uuid,
      productUuid: pUuid,
      productName: item.product_name,
      productImageUrl: item.image_url ? getImageUrl(item.image_url) : undefined,
    })
  }

  // --- SYNC TRACKING ACTIONS ---
  const handleSyncTracking = async (order: Order) => {
    if (isSyncingTracking) return
    setIsSyncingTracking(true)

    try {
      const baseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
      const res = await authFetch(`${baseUrl}/api/v1/orders/${order.uuid}/sync-tracking`, {
        method: 'POST',
      })

      if (res.ok) {
        toast.success('Live tracking data synchronized with shipping carrier!')
        // Refresh timeline events & order details
        const [orderRes, timelineRes] = await Promise.all([
          authFetch(`${baseUrl}/api/v1/orders/${order.uuid}`),
          authFetch(`${baseUrl}/api/v1/orders/${order.uuid}/timeline`),
        ])
        if (orderRes.ok) {
          const ordJson = await orderRes.json()
          if (ordJson?.data) setSelectedOrder(ordJson.data)
        }
        if (timelineRes.ok) {
          const tlJson = await timelineRes.json()
          const events: TrackingEvent[] = Array.isArray(tlJson?.data)
            ? tlJson.data
            : Array.isArray(tlJson?.data?.data)
            ? tlJson.data.data
            : []
          setTimelineEvents(events)
        }
        await fetchOrders()
      } else {
        const errJson = await res.json().catch(() => null)
        const msg = extractErrorMessage(errJson, 'Failed to sync carrier tracking.')
        toast.error(msg)
      }
    } catch {
      toast.error('Network error while synchronizing tracking details.')
    } finally {
      setIsSyncingTracking(false)
    }
  }

  const generatePaginationPages = (current: number, total: number) => {
    if (total <= 7) {
      return Array.from({ length: total }, (_, i) => i + 1)
    }
    if (current <= 4) {
      return [1, 2, 3, 4, 5, '...', total]
    }
    if (current >= total - 3) {
      return [1, '...', total - 4, total - 3, total - 2, total - 1, total]
    }
    return [1, '...', current - 1, current, current + 1, '...', total]
  }

  const filteredOrders = orders

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
    const isConfirmed = selectedOrder.status?.toUpperCase() === 'CONFIRMED'
    const isShipped =
      selectedOrder.status?.toUpperCase() === 'SHIPPED' ||
      selectedOrder.status?.toUpperCase() === 'IN_TRANSIT' ||
      selectedOrder.status?.toUpperCase() === 'INTRANSIT'
    const isDelivered = selectedOrder.status?.toUpperCase() === 'DELIVERED'
    const isPaid = selectedOrder.payment_status?.toUpperCase() === 'PAID'
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
            onClick={handleBackToOrdersList}
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
                  ? 'Parcel delivered successfully. You can request return/refund or review items below.'
                  : isShipped
                  ? 'Your parcel is in transit with GHN Express. Click "Confirm Delivery" once you receive your package.'
                  : activeStepIndex >= 2
                  ? 'Your parcel is handled by GHN Express and moving towards your destination.'
                  : 'Your order has been received and is being prepared for dispatch.'}
              </span>
            </div>

            <div className="flex items-center gap-2 flex-wrap">
              {(isPaid || isDelivered || isConfirmed || isShipped) && (
                <button
                  type="button"
                  disabled={downloadingInvoiceUuid === selectedOrder.uuid}
                  onClick={() => handleDownloadInvoice(selectedOrder)}
                  className="px-3 py-1.5 rounded-lg bg-slate-50 hover:bg-slate-100 text-slate-700 border border-slate-200 text-xs font-bold transition-all flex items-center gap-1.5 cursor-pointer disabled:opacity-50"
                  title="Download official PDF sales invoice"
                >
                  {downloadingInvoiceUuid === selectedOrder.uuid ? (
                    <RefreshCw className="w-3.5 h-3.5 animate-spin text-[#2196F3]" />
                  ) : (
                    <Download className="w-3.5 h-3.5 text-slate-600" />
                  )}
                  <span>Download Invoice</span>
                </button>
              )}

              {isShipped && (
                <button
                  type="button"
                  onClick={() => handleOpenConfirmDelivery(selectedOrder)}
                  className="px-3 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-700 text-white text-xs font-bold transition-all flex items-center gap-1.5 shadow-xs cursor-pointer"
                  title="Confirm that you have received this parcel in good condition"
                >
                  <PackageCheck className="w-3.5 h-3.5" />
                  <span>Confirm Delivery</span>
                </button>
              )}

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

              {isReturned && (
                <button
                  type="button"
                  onClick={() => handleOpenReturnDetails(selectedOrder)}
                  className="px-3 py-1.5 rounded-lg bg-purple-50 hover:bg-purple-100 text-purple-700 border border-purple-200 text-xs font-bold transition-all flex items-center gap-1.5 cursor-pointer"
                >
                  <RotateCcw className="w-3.5 h-3.5" />
                  <span>View Return Request</span>
                </button>
              )}
            </div>
          </div>
        </div>

        {/* ========================================================= */}
        {/* 1. ORDER TIMELINE & TRACKING HISTORY (ABOVE DELIVERY ADDR) */}
        {/* ========================================================= */}
        <div className="p-5 sm:p-6 rounded-2xl bg-white border border-slate-200/80 shadow-2xs space-y-5">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between pb-3 border-b border-slate-100 gap-2">
            <div className="flex items-center gap-2.5">
              <div className="w-8 h-8 rounded-lg bg-emerald-50 text-emerald-600 flex items-center justify-center">
                <Clock className="w-4 h-4" />
              </div>
              <h3 className="text-sm font-extrabold uppercase tracking-wider text-[#0D1B2A]">
                Order Timeline & Tracking History
              </h3>
            </div>

            <div className="flex items-center gap-3 text-xs">
              <span className="text-slate-500">
                Carrier: <strong className="text-slate-800 font-semibold">GHN Logistics</strong>
              </span>

              <button
                type="button"
                disabled={isSyncingTracking}
                onClick={() => handleSyncTracking(selectedOrder)}
                className="px-2.5 py-1 rounded-lg bg-blue-50 hover:bg-blue-100 text-[#0D47A1] border border-blue-200 font-bold transition-all flex items-center gap-1.5 cursor-pointer disabled:opacity-50"
                title="Synchronize live tracking updates from shipping carrier"
              >
                <RefreshCw className={`w-3.5 h-3.5 ${isSyncingTracking ? 'animate-spin' : ''}`} />
                <span>Sync Tracking</span>
              </button>
            </div>
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

                  <div className="flex sm:flex-col items-center sm:items-end justify-between sm:justify-center gap-2 shrink-0 pt-2 sm:pt-0 border-t sm:border-t-0 border-slate-100">
                    <div className="flex items-center gap-2">
                      <span className="text-slate-400 text-[11px] sm:hidden">Item Subtotal:</span>
                      <span className="text-sm font-extrabold text-[#0D47A1]">
                        {formatCurrency(item.subtotal || item.price * item.quantity)}
                      </span>
                    </div>

                    {isDelivered && (
                      <button
                        type="button"
                        onClick={() => handleOpenReviewItem(selectedOrder, item)}
                        className="px-3 py-1 rounded-lg bg-amber-50 hover:bg-amber-100 text-amber-800 border border-amber-200 text-[11px] font-bold transition-all flex items-center gap-1 cursor-pointer shadow-2xs"
                        title="Write a review for this product"
                      >
                        <Star className="w-3.5 h-3.5 fill-amber-400 text-amber-400" />
                        <span>Review Product</span>
                      </button>
                    )}
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
            {(isPaid || isDelivered) && (
              <button
                type="button"
                disabled={downloadingInvoiceUuid === selectedOrder.uuid}
                onClick={() => handleDownloadInvoice(selectedOrder)}
                className="px-4 py-2.5 rounded-xl bg-slate-50 hover:bg-slate-100 text-slate-700 border border-slate-200 text-xs font-bold transition-all flex items-center gap-1.5 cursor-pointer disabled:opacity-50"
              >
                {downloadingInvoiceUuid === selectedOrder.uuid ? (
                  <RefreshCw className="w-3.5 h-3.5 animate-spin text-[#2196F3]" />
                ) : (
                  <Download className="w-3.5 h-3.5" />
                )}
                <span>Download Invoice</span>
              </button>
            )}

            {isShipped && (
              <button
                type="button"
                onClick={() => handleOpenConfirmDelivery(selectedOrder)}
                className="px-5 py-2.5 rounded-xl bg-emerald-600 hover:bg-emerald-700 text-white text-xs font-bold transition-all flex items-center gap-1.5 shadow-xs cursor-pointer"
              >
                <PackageCheck className="w-4 h-4" />
                <span>Confirm Delivery</span>
              </button>
            )}

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
          {/* Search Input Bar with Debounce indicator */}
          <div className="relative flex-1 sm:w-64 md:w-72">
            <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none text-slate-400">
              {isSearching ? (
                <RefreshCw className="w-3.5 h-3.5 text-[#2196F3] animate-spin" />
              ) : (
                <Search className="w-3.5 h-3.5" />
              )}
            </div>
            <input
              type="text"
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              placeholder="Search by order #, product, or recipient..."
              className="w-full pl-8.5 pr-8 py-2 rounded-xl bg-white border border-slate-200 text-xs font-medium text-[#0D1B2A] placeholder:text-slate-400 focus:outline-none focus:border-[#2196F3] shadow-2xs transition-all"
            />
            {searchInput && (
              <button
                type="button"
                onClick={() => setSearchInput('')}
                className="absolute inset-y-0 right-0 pr-2.5 flex items-center text-slate-400 hover:text-slate-600 cursor-pointer"
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
              <option value="ALL">
                All Orders {pagination.total_records > 0 ? `(${pagination.total_records})` : `(${orders.length})`}
              </option>
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

          {(filter !== 'ALL' || searchInput.trim() !== '') && (
            <button
              type="button"
              onClick={() => {
                setFilter('ALL')
                setSearchInput('')
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
              : debouncedSearch.trim()
              ? `No orders matching "${debouncedSearch.trim()}".`
              : "You haven't placed any orders matching this filter yet."}
          </p>
          <div className="pt-2 flex items-center justify-center gap-2">
            {orders.length > 0 && (filter !== 'ALL' || debouncedSearch.trim()) ? (
              <button
                type="button"
                onClick={() => {
                  setFilter('ALL')
                  setSearchInput('')
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
            const isConfirmed = order.status?.toUpperCase() === 'CONFIRMED'
            const isShipped =
              order.status?.toUpperCase() === 'SHIPPED' ||
              order.status?.toUpperCase() === 'IN_TRANSIT' ||
              order.status?.toUpperCase() === 'INTRANSIT'
            const isDelivered = order.status?.toUpperCase() === 'DELIVERED'
            const isReturned = order.status?.toUpperCase() === 'RETURNED'
            const isPaid = order.payment_status?.toUpperCase() === 'PAID'

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

                      {/* Confirm Delivery Button (if Shipped) */}
                      {isShipped && (
                        <button
                          type="button"
                          onClick={(e) => {
                            e.stopPropagation()
                            handleOpenConfirmDelivery(order)
                          }}
                          className="px-2.5 py-1 rounded-lg bg-emerald-600 hover:bg-emerald-700 text-white text-xs font-semibold transition-all flex items-center gap-1 shadow-xs cursor-pointer"
                          title="I have received this order"
                        >
                          <PackageCheck className="w-3.5 h-3.5" />
                          <span>Confirm Delivery</span>
                        </button>
                      )}

                      {/* Tracking Button */}
                      {(isConfirmed || isShipped || isDelivered) && (
                        <button
                          type="button"
                          onClick={(e) => {
                            e.stopPropagation()
                            handleOpenTrackingModal(order)
                          }}
                          className="px-2.5 py-1 rounded-lg bg-blue-50 hover:bg-blue-100 text-[#0D47A1] border border-blue-200 text-xs font-semibold transition-all flex items-center gap-1 cursor-pointer"
                          title="Track shipment"
                        >
                          <Truck className="w-3.5 h-3.5 text-[#2196F3]" />
                          <span>Track</span>
                        </button>
                      )}

                      {/* Invoice Download */}
                      {(isPaid || isDelivered || isConfirmed || isShipped) && (
                        <button
                          type="button"
                          disabled={downloadingInvoiceUuid === order.uuid}
                          onClick={(e) => {
                            e.stopPropagation()
                            handleDownloadInvoice(order)
                          }}
                          className="px-2.5 py-1 rounded-lg bg-slate-50 hover:bg-slate-100 text-slate-700 border border-slate-200 text-xs font-semibold transition-all flex items-center gap-1 cursor-pointer disabled:opacity-50"
                          title="Download Invoice PDF"
                        >
                          {downloadingInvoiceUuid === order.uuid ? (
                            <RefreshCw className="w-3.5 h-3.5 animate-spin text-[#2196F3]" />
                          ) : (
                            <Download className="w-3.5 h-3.5 text-slate-600" />
                          )}
                          <span>Invoice</span>
                        </button>
                      )}

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
                          <span>Cancel</span>
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
                          <span>Return</span>
                        </button>
                      )}

                      {/* View Return Button (if Returned) */}
                      {isReturned && (
                        <button
                          type="button"
                          onClick={(e) => {
                            e.stopPropagation()
                            handleOpenReturnDetails(order)
                          }}
                          className="px-2.5 py-1 rounded-lg bg-purple-50 hover:bg-purple-100 text-purple-700 border border-purple-200 text-xs font-semibold transition-all flex items-center gap-1 cursor-pointer"
                          title="View return request progress"
                        >
                          <RotateCcw className="w-3.5 h-3.5" />
                          <span>Return Details</span>
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

      {/* Pagination Controls << < 1 2 3 4 5 > >> */}
      {orders.length > 0 && (
        <div className="p-4 sm:p-5 rounded-2xl bg-white border border-slate-200/80 shadow-2xs flex flex-col md:flex-row items-center justify-between gap-4 mt-2">
          {/* Summary & Page Size selector */}
          <div className="flex items-center gap-3 text-xs text-slate-500 font-medium flex-wrap justify-center sm:justify-start">
            <span>
              Showing{' '}
              <strong className="text-slate-800">
                {Math.min(
                  (pagination.page - 1) * pagination.limit + 1,
                  pagination.total_records || orders.length
                )}
              </strong>{' '}
              to{' '}
              <strong className="text-slate-800">
                {Math.min(
                  pagination.page * pagination.limit,
                  pagination.total_records || orders.length
                )}
              </strong>{' '}
              of{' '}
              <strong className="text-slate-800">
                {pagination.total_records || orders.length}
              </strong>{' '}
              orders
            </span>

            <div className="flex items-center gap-1.5 pl-2 border-l border-slate-200">
              <span className="text-slate-400">Show:</span>
              <select
                value={pageSize}
                onChange={(e) => {
                  setPageSize(Number(e.target.value))
                  setCurrentPage(1)
                }}
                className="px-2 py-1 rounded-lg bg-slate-50 border border-slate-200 text-xs font-semibold text-[#0D1B2A] focus:outline-none focus:border-[#2196F3] cursor-pointer hover:bg-white transition-all"
              >
                <option value={10}>10 / page</option>
                <option value={20}>20 / page</option>
                <option value={50}>50 / page</option>
                <option value={100}>100 / page</option>
              </select>
            </div>
          </div>

          {/* Navigation Buttons: << < 1 2 3 4 5 > >> */}
          <div className="flex items-center gap-1.5 flex-wrap justify-center">
            {/* << First Page */}
            <button
              type="button"
              disabled={currentPage <= 1 || loading}
              onClick={() => setCurrentPage(1)}
              className="h-8 w-8 rounded-xl border border-slate-200 bg-white text-slate-600 hover:bg-slate-50 hover:text-slate-900 disabled:opacity-35 disabled:cursor-not-allowed flex items-center justify-center transition-all cursor-pointer shadow-2xs"
              title="First Page"
            >
              <ChevronsLeft className="w-4 h-4" />
            </button>

            {/* < Prev Page */}
            <button
              type="button"
              disabled={currentPage <= 1 || loading}
              onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
              className="h-8 w-8 rounded-xl border border-slate-200 bg-white text-slate-600 hover:bg-slate-50 hover:text-slate-900 disabled:opacity-35 disabled:cursor-not-allowed flex items-center justify-center transition-all cursor-pointer shadow-2xs"
              title="Previous Page"
            >
              <ChevronLeft className="w-4 h-4" />
            </button>

            {/* Page Numbers */}
            {generatePaginationPages(currentPage, pagination.total_pages).map((p, idx) => {
              if (p === '...') {
                return (
                  <span
                    key={`ellipsis-${idx}`}
                    className="w-8 h-8 flex items-center justify-center text-slate-400 text-xs font-semibold select-none"
                  >
                    ...
                  </span>
                )
              }

              const pageNum = Number(p)
              const isActive = pageNum === currentPage

              return (
                <button
                  key={`page-${pageNum}`}
                  type="button"
                  disabled={loading}
                  onClick={() => setCurrentPage(pageNum)}
                  className={`h-8 min-w-8 px-2.5 rounded-xl text-xs font-bold transition-all flex items-center justify-center cursor-pointer shadow-2xs ${
                    isActive
                      ? 'bg-[#2196F3] text-white shadow-xs'
                      : 'border border-slate-200 bg-white text-slate-700 hover:bg-slate-50 hover:border-slate-300'
                  }`}
                >
                  {pageNum}
                </button>
              )
            })}

            {/* > Next Page */}
            <button
              type="button"
              disabled={currentPage >= pagination.total_pages || loading}
              onClick={() => setCurrentPage((p) => Math.min(pagination.total_pages, p + 1))}
              className="h-8 w-8 rounded-xl border border-slate-200 bg-white text-slate-600 hover:bg-slate-50 hover:text-slate-900 disabled:opacity-35 disabled:cursor-not-allowed flex items-center justify-center transition-all cursor-pointer shadow-2xs"
              title="Next Page"
            >
              <ChevronRight className="w-4 h-4" />
            </button>

            {/* >> Last Page */}
            <button
              type="button"
              disabled={currentPage >= pagination.total_pages || loading}
              onClick={() => setCurrentPage(pagination.total_pages)}
              className="h-8 w-8 rounded-xl border border-slate-200 bg-white text-slate-600 hover:bg-slate-50 hover:text-slate-900 disabled:opacity-35 disabled:cursor-not-allowed flex items-center justify-center transition-all cursor-pointer shadow-2xs"
              title="Last Page"
            >
              <ChevronsRight className="w-4 h-4" />
            </button>
          </div>
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
          <RequestReturnModal
            isOpen={!!orderToReturn}
            onClose={() => setOrderToReturn(null)}
            orderUuid={orderToReturn.uuid}
            orderNumber={orderToReturn.order_number}
            paymentMethod={orderToReturn.payment_method}
            totalAmount={orderToReturn.total_amount}
            onSuccess={() => {
              setOrderToReturn(null)
              fetchOrders()
              if (selectedOrder && selectedOrder.uuid === orderToReturn.uuid) {
                handleOpenOrderDetails(orderToReturn, false)
              }
            }}
          />
        )}

        {/* Return Details Modal */}
        {orderToViewReturn && (
          <ReturnDetailModal
            isOpen={!!orderToViewReturn}
            onClose={() => setOrderToViewReturn(null)}
            orderUuid={orderToViewReturn.uuid}
            onUpdated={() => {
              fetchOrders()
              if (selectedOrder && selectedOrder.uuid === orderToViewReturn.uuid) {
                handleOpenOrderDetails(orderToViewReturn, false)
              }
            }}
          />
        )}

        {/* Order Tracking Modal */}
        {orderToTrack && (
          <OrderTrackingModal
            isOpen={!!orderToTrack}
            onClose={() => setOrderToTrack(null)}
            orderUuid={orderToTrack.uuid}
            orderNumber={orderToTrack.order_number}
            orderStatus={orderToTrack.status}
            onStatusUpdated={() => {
              fetchOrders()
              if (selectedOrder && selectedOrder.uuid === orderToTrack.uuid) {
                handleOpenOrderDetails(orderToTrack, false)
              }
            }}
          />
        )}

        {/* Confirm Delivery Confirmation Dialog */}
        {orderToConfirmDelivery && (
          <div className="fixed inset-0 z-60 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 overflow-y-auto animate-in fade-in">
            <div className="w-full max-w-md bg-white rounded-3xl p-6 shadow-2xl space-y-4 border border-slate-100">
              <div className="w-12 h-12 rounded-full bg-emerald-50 text-emerald-600 flex items-center justify-center mx-auto">
                <PackageCheck className="w-6 h-6" />
              </div>

              <div className="text-center space-y-1">
                <h3 className="text-base font-extrabold text-[#0D1B2A]">
                  Confirm Order Delivery
                </h3>
                <p className="text-xs text-slate-500 leading-relaxed">
                  Have you received all items for order{' '}
                  <strong className="text-slate-800">
                    #{orderToConfirmDelivery.order_number || orderToConfirmDelivery.uuid.slice(0, 8).toUpperCase()}
                  </strong>{' '}
                  in good condition?
                </p>
              </div>

              <div className="p-3 rounded-xl bg-slate-50 border border-slate-100 text-[11px] text-slate-500 flex items-start gap-2">
                <ShieldCheck className="w-4 h-4 text-emerald-600 shrink-0 mt-0.5" />
                <span>
                  Confirming delivery completes this order and unlocks product reviews and official sales invoices.
                </span>
              </div>

              <div className="flex gap-2 pt-2">
                <button
                  type="button"
                  disabled={isConfirmingDelivery}
                  onClick={() => setOrderToConfirmDelivery(null)}
                  className="flex-1 py-2.5 rounded-xl bg-slate-100 hover:bg-slate-200 text-slate-700 text-xs font-bold transition-all cursor-pointer"
                >
                  Not Yet
                </button>
                <button
                  type="button"
                  disabled={isConfirmingDelivery}
                  onClick={handleConfirmDelivery}
                  className="flex-1 py-2.5 rounded-xl bg-emerald-600 hover:bg-emerald-700 text-white text-xs font-bold transition-all shadow-xs flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-50"
                >
                  {isConfirmingDelivery ? (
                    <>
                      <div className="w-3.5 h-3.5 rounded-full border-2 border-white border-t-transparent animate-spin" />
                      <span>Confirming...</span>
                    </>
                  ) : (
                    <>
                      <PackageCheck className="w-3.5 h-3.5" />
                      <span>Yes, I Received It</span>
                    </>
                  )}
                </button>
              </div>
            </div>
          </div>
        )}

        {/* Write Review Modal for Order Items */}
        {reviewingItem && (
          <WriteReviewModal
            isOpen={!!reviewingItem}
            onClose={() => setReviewingItem(null)}
            orderUuid={reviewingItem.orderUuid}
            productUuid={reviewingItem.productUuid}
            productName={reviewingItem.productName}
            productImageUrl={reviewingItem.productImageUrl}
            onSuccess={() => {
              setReviewingItem(null)
              fetchOrders()
            }}
          />
        )}
      </>
    )
  }
}

export default function MyOrdersPage() {
  return (
    <Suspense
      fallback={
        <div className="p-12 text-center text-slate-400 space-y-2">
          <RefreshCw className="w-6 h-6 animate-spin mx-auto text-[#2196F3]" />
          <p className="text-xs font-medium">Loading orders...</p>
        </div>
      }
    >
      <MyOrdersPageContent />
    </Suspense>
  )
}
