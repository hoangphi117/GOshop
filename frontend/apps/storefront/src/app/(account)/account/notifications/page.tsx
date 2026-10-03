'use client'

import React, { useState, useEffect, useCallback } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { authFetch } from '@/lib/api'
import { extractErrorMessage } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'
import {
  useNotificationStore,
  NotificationItem,
} from '@/stores/notification-store'
import { useMounted } from '@/hooks/use-mounted'
import {
  Bell,
  CheckCheck,
  Check,
  Trash2,
  Package,
  Truck,
  CheckCircle2,
  RotateCcw,
  Star,
  Tag,
  Sparkles,
  Info,
  ShieldAlert,
  ExternalLink,
  RefreshCw,
  Loader2,
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  Filter,
} from 'lucide-react'
import { toast } from 'sonner'

export default function NotificationsPage() {
  const router = useRouter()
  const { accessToken } = useAuthStore()
  const mounted = useMounted()

  const {
    notifications,
    unreadCount,
    isLoading: storeLoading,
    isMarkingAllRead,
    fetchNotifications,
    fetchUnreadCount,
    markAsRead,
    markAllAsRead,
    deleteNotification,
  } = useNotificationStore()

  const [activeTab, setActiveTab] = useState<'ALL' | 'UNREAD' | 'SECURITY' | 'ORDERS' | 'PROMO'>('ALL')
  const [isRefreshing, setIsRefreshing] = useState(false)
  const [currentPage, setCurrentPage] = useState(1)
  const limit = 10

  const handleRefresh = async () => {
    setIsRefreshing(true)
    try {
      await Promise.all([fetchNotifications(), fetchUnreadCount()])
      toast.success('Notifications refreshed')
    } catch {
      toast.error('Failed to refresh notifications')
    } finally {
      setIsRefreshing(false)
    }
  }

  // Filter notifications according to active tab
  const filteredNotifications = notifications.filter((item) => {
    if (activeTab === 'UNREAD') return !item.is_read
    if (activeTab === 'SECURITY') return item.type === 'security_alert' || item.type.includes('security')
    if (activeTab === 'ORDERS') return item.type.includes('order') || item.type.includes('shipping') || item.type.includes('delivery')
    if (activeTab === 'PROMO') return item.type.includes('promo') || item.type.includes('coupon') || item.type.includes('discount')
    return true
  })

  // Pagination calculation
  const totalRecords = filteredNotifications.length
  const totalPages = Math.max(1, Math.ceil(totalRecords / limit))
  const paginatedItems = filteredNotifications.slice(
    (currentPage - 1) * limit,
    currentPage * limit
  )

  // Reset page when filter tab changes
  useEffect(() => {
    setCurrentPage(1)
  }, [activeTab])

  // Get icon & visual theme based on notification type
  const getNotificationVisuals = (type: string) => {
    switch (type) {
      case 'security_alert':
        return {
          icon: ShieldAlert,
          bg: 'bg-rose-50 border-rose-200 text-rose-600',
          badgeBg: 'bg-rose-100 text-rose-700',
          label: 'Security Alert',
        }
      case 'order_created':
      case 'order_placed':
      case 'order_confirmed':
        return {
          icon: Package,
          bg: 'bg-blue-50 border-blue-200 text-blue-600',
          badgeBg: 'bg-blue-100 text-blue-700',
          label: 'Order Update',
        }
      case 'order_shipped':
      case 'shipping_update':
        return {
          icon: Truck,
          bg: 'bg-indigo-50 border-indigo-200 text-indigo-600',
          badgeBg: 'bg-indigo-100 text-indigo-700',
          label: 'Shipping',
        }
      case 'order_delivered':
      case 'order_completed':
        return {
          icon: CheckCircle2,
          bg: 'bg-emerald-50 border-emerald-200 text-emerald-600',
          badgeBg: 'bg-emerald-100 text-emerald-700',
          label: 'Delivered',
        }
      case 'refund_processed':
      case 'order_cancelled':
      case 'return_requested':
        return {
          icon: RotateCcw,
          bg: 'bg-amber-50 border-amber-200 text-amber-600',
          badgeBg: 'bg-amber-100 text-amber-700',
          label: 'Return & Refund',
        }
      case 'review_reminder':
      case 'review_received':
        return {
          icon: Star,
          bg: 'bg-amber-50 border-amber-200 text-amber-600',
          badgeBg: 'bg-amber-100 text-amber-700',
          label: 'Review',
        }
      case 'promotion':
      case 'coupon_available':
      case 'flash_sale':
        return {
          icon: Sparkles,
          bg: 'bg-purple-50 border-purple-200 text-purple-600',
          badgeBg: 'bg-purple-100 text-purple-700',
          label: 'Promotion',
        }
      default:
        return {
          icon: Info,
          bg: 'bg-slate-100 border-slate-200 text-slate-600',
          badgeBg: 'bg-slate-100 text-slate-700',
          label: 'System Notification',
        }
    }
  }

  // Handle action click
  const handleActionClick = (item: NotificationItem) => {
    if (!item.is_read) {
      markAsRead(item.uuid)
    }

    if (item.data?.url) {
      router.push(item.data.url)
      return
    }

    if (item.type === 'security_alert') {
      router.push('/account/security')
      return
    }

    if (
      item.type.includes('order') ||
      item.type.includes('shipping') ||
      item.type.includes('delivery')
    ) {
      const orderRef = item.data?.order_number || item.data?.order_uuid
      if (orderRef) {
        router.push(`/account/orders?orderId=${encodeURIComponent(String(orderRef))}`)
      } else {
        router.push('/account/orders')
      }
      return
    }

    if (item.type.includes('review')) {
      router.push('/account/reviews')
      return
    }
  }

  // Pagination page numbers generator
  const getPaginationPages = () => {
    const pages: (number | string)[] = []
    const delta = 2

    for (let i = 1; i <= totalPages; i++) {
      if (
        i === 1 ||
        i === totalPages ||
        (i >= currentPage - delta && i <= currentPage + delta)
      ) {
        pages.push(i)
      } else if (
        (i === currentPage - delta - 1 || i === currentPage + delta + 1) &&
        pages[pages.length - 1] !== '...'
      ) {
        pages.push('...')
      }
    }

    return pages
  }

  if (!mounted) {
    return (
      <div className="p-8 rounded-3xl bg-white border border-slate-200/80 shadow-2xs flex items-center justify-center min-h-[300px]">
        <Loader2 className="w-8 h-8 text-[#2196F3] animate-spin" />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header Card */}
      <div className="p-6 sm:p-8 rounded-3xl bg-white border border-slate-200/80 shadow-2xs space-y-5">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div>
            <div className="flex items-center gap-2.5">
              <h1 className="text-xl sm:text-2xl font-black text-[#0D1B2A] tracking-tight">
                Notifications
              </h1>
              {unreadCount > 0 && (
                <span className="px-2.5 py-0.5 rounded-full bg-rose-500 text-white text-xs font-bold shadow-xs">
                  {unreadCount} Unread
                </span>
              )}
            </div>
            <p className="text-xs text-slate-500 mt-1">
              Stay updated on security alerts, order shipments, and special offers.
            </p>
          </div>

          <div className="flex items-center gap-2 self-start sm:self-auto">
            <button
              type="button"
              onClick={handleRefresh}
              disabled={isRefreshing || storeLoading}
              className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-xl border border-slate-200 bg-white hover:bg-slate-50 text-slate-700 text-xs font-bold transition-all disabled:opacity-50 cursor-pointer"
              title="Refresh notifications"
            >
              <RefreshCw className={`w-3.5 h-3.5 ${isRefreshing ? 'animate-spin text-[#2196F3]' : 'text-slate-500'}`} />
              <span>Refresh</span>
            </button>

            {unreadCount > 0 && (
              <button
                type="button"
                onClick={() => markAllAsRead()}
                disabled={isMarkingAllRead}
                className="inline-flex items-center gap-1.5 px-4 py-2 rounded-xl bg-[#0D47A1] hover:bg-[#2196F3] text-white text-xs font-bold transition-all shadow-xs disabled:opacity-50 cursor-pointer"
              >
                {isMarkingAllRead ? (
                  <>
                    <Loader2 className="w-3.5 h-3.5 animate-spin" />
                    <span>Marking...</span>
                  </>
                ) : (
                  <>
                    <CheckCheck className="w-3.5 h-3.5" />
                    <span>Mark all read</span>
                  </>
                )}
              </button>
            )}
          </div>
        </div>

        {/* Filter Tabs */}
        <div className="flex items-center gap-1.5 overflow-x-auto pb-1 pt-1 border-t border-slate-100 scrollbar-none">
          {[
            { id: 'ALL', label: 'All', count: notifications.length },
            { id: 'UNREAD', label: 'Unread', count: unreadCount },
            {
              id: 'SECURITY',
              label: 'Security',
              count: notifications.filter((n) => n.type === 'security_alert' || n.type.includes('security')).length,
            },
            {
              id: 'ORDERS',
              label: 'Orders',
              count: notifications.filter((n) => n.type.includes('order') || n.type.includes('shipping') || n.type.includes('delivery')).length,
            },
            {
              id: 'PROMO',
              label: 'Promotions',
              count: notifications.filter((n) => n.type.includes('promo') || n.type.includes('coupon')).length,
            },
          ].map((tab) => {
            const isActive = activeTab === tab.id
            return (
              <button
                key={tab.id}
                type="button"
                onClick={() => setActiveTab(tab.id as typeof activeTab)}
                className={`flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-bold transition-all whitespace-nowrap cursor-pointer ${
                  isActive
                    ? 'bg-[#E3F2FD] text-[#0D47A1] shadow-2xs'
                    : 'text-slate-600 hover:bg-slate-50 hover:text-slate-900'
                }`}
              >
                <span>{tab.label}</span>
                {tab.count > 0 && (
                  <span
                    className={`px-1.5 py-0.5 rounded-md text-[10px] font-extrabold ${
                      isActive
                        ? 'bg-[#2196F3] text-white'
                        : 'bg-slate-100 text-slate-500'
                    }`}
                  >
                    {tab.count}
                  </span>
                )}
              </button>
            )
          })}
        </div>
      </div>

      {/* Notifications Stream */}
      {storeLoading && notifications.length === 0 ? (
        <div className="p-12 rounded-3xl bg-white border border-slate-200/80 shadow-2xs text-center space-y-3">
          <Loader2 className="w-8 h-8 text-[#2196F3] animate-spin mx-auto" />
          <p className="text-xs text-slate-500 font-medium">Loading notifications...</p>
        </div>
      ) : paginatedItems.length > 0 ? (
        <div className="space-y-3.5">
          {paginatedItems.map((item) => {
            const visuals = getNotificationVisuals(item.type)
            const Icon = visuals.icon

            const hasAction = Boolean(
              item.data?.url ||
              item.type === 'security_alert' ||
              item.type.includes('order') ||
              item.type.includes('review')
            )

            return (
              <div
                key={item.uuid}
                className={`p-5 sm:p-6 rounded-3xl border transition-all ${
                  item.is_read
                    ? 'bg-white border-slate-200/80 shadow-2xs hover:border-slate-300'
                    : 'bg-[#F4F9FF] border-[#BBDEFB] shadow-xs hover:border-[#90CAF9]'
                }`}
              >
                <div className="flex items-start gap-4">
                  {/* Icon */}
                  <div
                    className={`w-11 h-11 rounded-2xl flex items-center justify-center shrink-0 shadow-2xs border ${visuals.bg}`}
                  >
                    <Icon className="w-5 h-5" />
                  </div>

                  {/* Body & Full Details */}
                  <div className="flex-1 min-w-0 space-y-2">
                    <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-1 sm:gap-4">
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className={`px-2 py-0.5 rounded-md text-[10px] font-bold ${visuals.badgeBg}`}>
                          {visuals.label}
                        </span>
                        {!item.is_read && (
                          <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-rose-50 text-rose-600 text-[10px] font-bold border border-rose-200/60">
                            <span className="w-1.5 h-1.5 rounded-full bg-rose-500 animate-ping" />
                            New
                          </span>
                        )}
                        <h3 className={`text-sm ${item.is_read ? 'font-bold text-[#0D1B2A]' : 'font-extrabold text-[#0D1B2A]'}`}>
                          {item.title}
                        </h3>
                      </div>

                      <span className="text-[11px] text-slate-400 font-medium shrink-0 font-mono">
                        {item.created_at}
                      </span>
                    </div>

                    {/* Full Description text without truncation */}
                    <div className="text-xs text-slate-600 leading-relaxed whitespace-pre-line break-words">
                      {item.body}
                    </div>

                    {/* Footer Actions */}
                    <div className="pt-2 flex flex-wrap items-center justify-between gap-3 border-t border-slate-100/80">
                      {hasAction ? (
                        <button
                          type="button"
                          onClick={() => handleActionClick(item)}
                          className="text-xs font-bold text-[#0D47A1] hover:text-[#2196F3] inline-flex items-center gap-1.5 transition-colors cursor-pointer group"
                        >
                          <span>
                            {item.type === 'security_alert'
                              ? 'Manage Active Sessions'
                              : item.type.includes('order')
                              ? 'View Order Details'
                              : item.type.includes('review')
                              ? 'View My Reviews'
                              : 'View Details'}
                          </span>
                          <ExternalLink className="w-3.5 h-3.5 group-hover:translate-x-0.5 transition-transform" />
                        </button>
                      ) : (
                        <div />
                      )}

                      <div className="flex items-center gap-2">
                        {!item.is_read && (
                          <button
                            type="button"
                            onClick={() => markAsRead(item.uuid)}
                            className="inline-flex items-center gap-1 px-3 py-1.5 rounded-xl border border-slate-200 bg-white hover:bg-slate-50 text-slate-700 text-xs font-semibold transition-all cursor-pointer shadow-2xs"
                            title="Mark as read"
                          >
                            <Check className="w-3.5 h-3.5 text-blue-600" />
                            <span>Mark read</span>
                          </button>
                        )}

                        <button
                          type="button"
                          onClick={() => deleteNotification(item.uuid)}
                          className="inline-flex items-center gap-1 px-3 py-1.5 rounded-xl border border-rose-200/80 bg-rose-50 hover:bg-rose-100 text-rose-600 text-xs font-semibold transition-all cursor-pointer shadow-2xs"
                          title="Delete notification"
                        >
                          <Trash2 className="w-3.5 h-3.5 text-rose-500" />
                          <span>Delete</span>
                        </button>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            )
          })}

          {/* Pagination Controls */}
          {totalPages > 1 && (
            <div className="p-4 rounded-3xl bg-white border border-slate-200/80 shadow-2xs flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs mt-4">
              <span className="text-slate-500 font-medium text-center sm:text-left">
                Showing page <strong className="text-slate-800">{currentPage}</strong> of{' '}
                <strong className="text-slate-800">{totalPages}</strong> ({totalRecords} total items)
              </span>

              <div className="flex items-center justify-center gap-1">
                {/* First Page */}
                <button
                  type="button"
                  disabled={currentPage <= 1}
                  onClick={() => setCurrentPage(1)}
                  className="w-8 h-8 rounded-xl border border-slate-200 bg-white hover:bg-slate-50 disabled:opacity-40 disabled:cursor-not-allowed text-slate-700 font-bold flex items-center justify-center transition-all cursor-pointer"
                  title="First Page"
                >
                  <ChevronsLeft className="w-4 h-4" />
                </button>

                {/* Prev Page */}
                <button
                  type="button"
                  disabled={currentPage <= 1}
                  onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
                  className="w-8 h-8 rounded-xl border border-slate-200 bg-white hover:bg-slate-50 disabled:opacity-40 disabled:cursor-not-allowed text-slate-700 font-bold flex items-center justify-center transition-all cursor-pointer"
                  title="Previous Page"
                >
                  <ChevronLeft className="w-4 h-4" />
                </button>

                {/* Numbered Buttons */}
                {getPaginationPages().map((pageNum, idx) => {
                  if (pageNum === '...') {
                    return (
                      <span
                        key={`dots-${idx}`}
                        className="w-8 h-8 flex items-center justify-center text-slate-400 font-bold"
                      >
                        ...
                      </span>
                    )
                  }

                  const pageNumber = Number(pageNum)
                  const isActive = pageNumber === currentPage

                  return (
                    <button
                      key={pageNumber}
                      type="button"
                      onClick={() => setCurrentPage(pageNumber)}
                      className={`w-8 h-8 rounded-xl text-xs font-bold transition-all cursor-pointer ${
                        isActive
                          ? 'bg-[#2196F3] text-white shadow-2xs'
                          : 'border border-slate-200 bg-white hover:bg-slate-50 text-slate-700'
                      }`}
                    >
                      {pageNumber}
                    </button>
                  )
                })}

                {/* Next Page */}
                <button
                  type="button"
                  disabled={currentPage >= totalPages}
                  onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))}
                  className="w-8 h-8 rounded-xl border border-slate-200 bg-white hover:bg-slate-50 disabled:opacity-40 disabled:cursor-not-allowed text-slate-700 font-bold flex items-center justify-center transition-all cursor-pointer"
                  title="Next Page"
                >
                  <ChevronRight className="w-4 h-4" />
                </button>

                {/* Last Page */}
                <button
                  type="button"
                  disabled={currentPage >= totalPages}
                  onClick={() => setCurrentPage(totalPages)}
                  className="w-8 h-8 rounded-xl border border-slate-200 bg-white hover:bg-slate-50 disabled:opacity-40 disabled:cursor-not-allowed text-slate-700 font-bold flex items-center justify-center transition-all cursor-pointer"
                  title="Last Page"
                >
                  <ChevronsRight className="w-4 h-4" />
                </button>
              </div>
            </div>
          )}
        </div>
      ) : (
        /* Empty State */
        <div className="p-12 sm:p-16 rounded-3xl bg-white border border-slate-200/80 shadow-2xs text-center space-y-4">
          <div className="w-16 h-16 rounded-3xl bg-[#E3F2FD] text-[#2196F3] flex items-center justify-center mx-auto shadow-xs">
            <Bell className="w-8 h-8" />
          </div>
          <div className="space-y-1 max-w-md mx-auto">
            <h3 className="text-lg font-extrabold text-[#0D1B2A]">
              {activeTab === 'UNREAD'
                ? 'No Unread Notifications'
                : activeTab === 'SECURITY'
                ? 'No Security Alerts'
                : activeTab === 'ORDERS'
                ? 'No Order Notifications'
                : activeTab === 'PROMO'
                ? 'No Promotion Notifications'
                : 'No Notifications Yet'}
            </h3>
            <p className="text-xs text-slate-500 leading-relaxed">
              {activeTab === 'UNREAD'
                ? 'You are all caught up! There are no unread notifications at the moment.'
                : 'When there are important updates regarding your account, orders, or security, they will appear here.'}
            </p>
          </div>
          {activeTab !== 'ALL' && (
            <div className="pt-2">
              <button
                type="button"
                onClick={() => setActiveTab('ALL')}
                className="inline-flex items-center gap-2 px-5 py-2.5 rounded-xl bg-slate-100 hover:bg-slate-200 text-slate-700 text-xs font-bold transition-all cursor-pointer"
              >
                <Filter className="w-3.5 h-3.5" />
                <span>View All Notifications</span>
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
