'use client'

import React, { useState, useEffect, useCallback } from 'react'
import Link from 'next/link'
import { authFetch } from '@/lib/api'
import { formatCurrency } from '@/lib/format'
import { extractErrorMessage } from '@/lib/utils'
import {
  RotateCcw,
  Search,
  Filter,
  Clock,
  CheckCircle2,
  XCircle,
  ShieldCheck,
  X,
  ChevronRight,
  ChevronLeft,
  ChevronsLeft,
  ChevronsRight,
  Loader2,
  Package,
  ArrowRight,
  Eye,
  AlertCircle,
  Truck,
} from 'lucide-react'
import { ReturnDetailModal } from '@/components/order/return-detail-modal'

export interface UserReturnSummary {
  return_uuid: string
  order_uuid: string
  order_number: string
  total_amount: number
  payment_method: string
  payment_status: string
  reason: string
  images: string[]
  video_url?: string | null
  status: number // 1: Pending, 2: Approved, 3: Rejected, 4: Completed, 5: Cancelled
  status_name: string
  status_text: string
  admin_note?: string | null
  shipping_provider?: string | null
  shipping_code?: string | null
  created_at: string
  updated_at: string
}

const STATUS_FILTERS = [
  { label: 'All Requests', value: 0 },
  { label: 'Pending Approval', value: 1 },
  { label: 'Approved & Shipping', value: 2 },
  { label: 'Completed / Refunded', value: 4 },
  { label: 'Rejected', value: 3 },
  { label: 'Cancelled', value: 5 },
]

export default function MyReturnsPage() {
  const [returns, setReturns] = useState<UserReturnSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [selectedStatus, setSelectedStatus] = useState<number>(0)
  const [searchQuery, setSearchQuery] = useState('')
  const [debouncedSearch, setDebouncedSearch] = useState('')
  const [currentPage, setCurrentPage] = useState(1)
  const [limit, setLimit] = useState(10)
  const [pagination, setPagination] = useState({
    page: 1,
    limit: 10,
    total_records: 0,
    total_pages: 1,
  })

  // Selected return to view in modal
  const [selectedOrderUuid, setSelectedOrderUuid] = useState<string | null>(null)

  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearch(searchQuery)
      setCurrentPage(1)
    }, 400)
    return () => clearTimeout(timer)
  }, [searchQuery])

  const fetchReturns = useCallback(async () => {
    setLoading(true)
    try {
      const query = new URLSearchParams()
      query.set('page', String(currentPage))
      query.set('limit', String(limit))
      if (selectedStatus > 0) {
        query.set('status', String(selectedStatus))
      }
      if (debouncedSearch.trim()) {
        query.set('search', debouncedSearch.trim())
      }

      const res = await authFetch(`/api/v1/orders/returns?${query.toString()}`)
      if (res.ok) {
        const json = await res.json()
        setReturns(json.data || [])
        if (json.pagination) {
          setPagination({
            page: json.pagination.page,
            limit: json.pagination.limit,
            total_records: json.pagination.total_records,
            total_pages: json.pagination.total_pages || 1,
          })
        }
      }
    } catch {
      // ignore
    } finally {
      setLoading(false)
    }
  }, [currentPage, limit, selectedStatus, debouncedSearch])

  useEffect(() => {
    fetchReturns()
  }, [fetchReturns])

  const getStatusBadge = (status: number, statusText: string) => {
    switch (status) {
      case 1:
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-bold bg-amber-50 text-amber-700 border border-amber-200">
            <Clock className="w-3 h-3" />
            <span>{statusText || 'Pending Approval'}</span>
          </span>
        )
      case 2:
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-bold bg-blue-50 text-blue-700 border border-blue-200">
            <CheckCircle2 className="w-3 h-3" />
            <span>{statusText || 'Approved (Awaiting Return Shipment)'}</span>
          </span>
        )
      case 3:
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-bold bg-rose-50 text-rose-700 border border-rose-200">
            <XCircle className="w-3 h-3" />
            <span>{statusText || 'Rejected'}</span>
          </span>
        )
      case 4:
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-bold bg-emerald-50 text-emerald-700 border border-emerald-200">
            <ShieldCheck className="w-3 h-3" />
            <span>{statusText || 'Completed'}</span>
          </span>
        )
      case 5:
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-bold bg-slate-100 text-slate-600 border border-slate-200">
            <X className="w-3 h-3" />
            <span>{statusText || 'Cancelled'}</span>
          </span>
        )
      default:
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-bold bg-slate-100 text-slate-700">
            <span>{statusText || 'Processing'}</span>
          </span>
        )
    }
  }

  return (
    <div className="space-y-6">
      {/* Title Bar */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-slate-100 pb-5">
        <div>
          <h1 className="text-2xl font-black text-[#0D1B2A] tracking-tight">
            Returns & Refund Requests
          </h1>
          <p className="text-xs text-slate-500 mt-1">
            Track and manage your product return requests, shipping status, and refund settlements.
          </p>
        </div>
        <Link
          href="/account/orders"
          className="inline-flex items-center gap-2 px-4 py-2.5 rounded-xl bg-white border border-slate-200 hover:border-[#2196F3] text-xs font-bold text-slate-700 hover:text-[#2196F3] transition-all shadow-2xs shrink-0 self-start sm:self-auto"
        >
          <Package className="w-4 h-4 text-[#2196F3]" />
          <span>Browse Orders to Return</span>
        </Link>
      </div>

      {/* Filter Tabs & Search */}
      <div className="space-y-3">
        <div className="flex items-center gap-2 overflow-x-auto pb-1 no-scrollbar">
          {STATUS_FILTERS.map((f) => {
            const isActive = selectedStatus === f.value
            return (
              <button
                key={f.value}
                type="button"
                onClick={() => {
                  setSelectedStatus(f.value)
                  setCurrentPage(1)
                }}
                className={`px-3.5 py-2 rounded-xl text-xs font-bold whitespace-nowrap transition-all cursor-pointer ${
                  isActive
                    ? 'bg-[#2196F3] text-white shadow-xs'
                    : 'bg-white border border-slate-200 text-slate-600 hover:bg-slate-50 hover:text-slate-900'
                }`}
              >
                {f.label}
              </button>
            )
          })}
        </div>

        {/* Search Bar */}
        <div className="relative">
          <Search className="absolute left-3.5 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Search by order number or reason..."
            className="w-full pl-10 pr-4 py-2.5 rounded-xl border border-slate-200 bg-white text-xs text-slate-800 focus:outline-none focus:border-[#2196F3] focus:ring-2 focus:ring-[#2196F3]/15 transition-all"
          />
        </div>
      </div>

      {/* List / Content */}
      {loading ? (
        <div className="py-20 flex flex-col items-center justify-center space-y-3">
          <Loader2 className="w-8 h-8 text-[#2196F3] animate-spin" />
          <p className="text-xs text-slate-500 font-semibold">Loading return requests...</p>
        </div>
      ) : returns.length === 0 ? (
        <div className="py-16 text-center space-y-4 bg-white rounded-3xl border border-slate-100 p-8">
          <div className="w-14 h-14 rounded-2xl bg-slate-50 text-slate-400 flex items-center justify-center mx-auto">
            <RotateCcw className="w-7 h-7" />
          </div>
          <div className="space-y-1">
            <h3 className="text-base font-bold text-[#0D1B2A]">No return requests found</h3>
            <p className="text-xs text-slate-500 max-w-sm mx-auto">
              You do not have any return or refund requests matching the selected filter.
            </p>
          </div>
          <Link
            href="/account/orders"
            className="inline-flex items-center gap-2 px-5 py-2.5 rounded-xl bg-[#2196F3] text-white text-xs font-bold hover:bg-[#0D47A1] transition-all shadow-xs"
          >
            <span>View Delivered Orders</span>
            <ArrowRight className="w-4 h-4" />
          </Link>
        </div>
      ) : (
        <div className="space-y-4">
          {returns.map((ret) => (
            <div
              key={ret.return_uuid}
              className="bg-white rounded-2xl border border-slate-200/80 p-5 shadow-2xs hover:shadow-md transition-all space-y-3"
            >
              {/* Card Header */}
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 border-b border-slate-100 pb-3">
                <div className="flex items-center gap-3 flex-wrap">
                  <span className="text-xs font-black text-[#0D1B2A]">
                    Return #{ret.return_uuid.slice(0, 8).toUpperCase()}
                  </span>
                  <span className="text-slate-300">•</span>
                  <span className="text-xs font-semibold text-slate-600">
                    Order #{ret.order_number || ret.order_uuid.slice(0, 8).toUpperCase()}
                  </span>
                  {getStatusBadge(ret.status, ret.status_text)}
                </div>

                <span className="text-[11px] font-semibold text-slate-400">
                  {new Date(ret.created_at).toLocaleDateString('en-US', {
                    month: 'short',
                    day: 'numeric',
                    year: 'numeric',
                  })}
                </span>
              </div>

              {/* Card Body */}
              <div className="grid grid-cols-1 md:grid-cols-3 gap-3 text-xs">
                <div className="md:col-span-2 space-y-1.5">
                  <span className="text-slate-400 font-semibold block text-[11px]">Reason</span>
                  <p className="text-slate-700 font-medium line-clamp-2 leading-relaxed">
                    {ret.reason}
                  </p>
                  {ret.shipping_code && (
                    <div className="flex items-center gap-2 text-[11px] text-blue-700 bg-blue-50 px-2.5 py-1 rounded-lg w-fit mt-1">
                      <Truck className="w-3.5 h-3.5" />
                      <span>
                        Return Waybill: <strong>{ret.shipping_code}</strong> ({ret.shipping_provider})
                      </span>
                    </div>
                  )}
                </div>

                <div className="flex md:flex-col items-center md:items-end justify-between md:justify-center border-t md:border-t-0 border-slate-100 pt-2 md:pt-0">
                  <div className="text-right">
                    <span className="text-[11px] text-slate-400 block">Refund Amount</span>
                    <span className="text-sm font-black text-rose-600">
                      {formatCurrency(ret.total_amount)}
                    </span>
                  </div>

                  {ret.status === 2 && !ret.shipping_code ? (
                    <button
                      type="button"
                      onClick={() => setSelectedOrderUuid(ret.order_uuid)}
                      className="mt-2 inline-flex items-center gap-1.5 px-3.5 py-2 rounded-xl bg-blue-600 hover:bg-blue-700 text-white text-xs font-bold transition-all shadow-xs cursor-pointer"
                    >
                      <Truck className="w-3.5 h-3.5" />
                      <span>Update Shipment</span>
                    </button>
                  ) : (
                    <button
                      type="button"
                      onClick={() => setSelectedOrderUuid(ret.order_uuid)}
                      className="mt-2 inline-flex items-center gap-1.5 px-3.5 py-2 rounded-xl bg-slate-50 hover:bg-blue-50 text-slate-700 hover:text-[#2196F3] border border-slate-200 hover:border-blue-200 text-xs font-bold transition-all cursor-pointer"
                    >
                      <Eye className="w-3.5 h-3.5" />
                      <span>View Details</span>
                    </button>
                  )}
                </div>
              </div>
            </div>
          ))}

          {/* Pagination */}
          {pagination.total_pages > 1 && (
            <div className="flex items-center justify-between gap-4 pt-4 border-t border-slate-100 flex-wrap">
              <span className="text-xs text-slate-500">
                Page {currentPage} of {pagination.total_pages} ({pagination.total_records} requests)
              </span>

              <div className="flex items-center gap-1">
                <button
                  type="button"
                  disabled={currentPage <= 1 || loading}
                  onClick={() => setCurrentPage(1)}
                  className="p-2 rounded-xl border border-slate-200 bg-white disabled:opacity-40"
                  title="First Page"
                >
                  <ChevronsLeft className="w-4 h-4 text-slate-600" />
                </button>
                <button
                  type="button"
                  disabled={currentPage <= 1 || loading}
                  onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
                  className="p-2 rounded-xl border border-slate-200 bg-white disabled:opacity-40"
                  title="Previous Page"
                >
                  <ChevronLeft className="w-4 h-4 text-slate-600" />
                </button>
                <span className="px-3 py-1.5 rounded-xl bg-[#2196F3] text-white text-xs font-bold">
                  {currentPage}
                </span>
                <button
                  type="button"
                  disabled={currentPage >= pagination.total_pages || loading}
                  onClick={() => setCurrentPage((p) => Math.min(pagination.total_pages, p + 1))}
                  className="p-2 rounded-xl border border-slate-200 bg-white disabled:opacity-40"
                  title="Next Page"
                >
                  <ChevronRight className="w-4 h-4 text-slate-600" />
                </button>
                <button
                  type="button"
                  disabled={currentPage >= pagination.total_pages || loading}
                  onClick={() => setCurrentPage(pagination.total_pages)}
                  className="p-2 rounded-xl border border-slate-200 bg-white disabled:opacity-40"
                  title="Last Page"
                >
                  <ChevronsRight className="w-4 h-4 text-slate-600" />
                </button>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Return Detail Modal */}
      {selectedOrderUuid && (
        <ReturnDetailModal
          isOpen={!!selectedOrderUuid}
          onClose={() => setSelectedOrderUuid(null)}
          orderUuid={selectedOrderUuid}
          onUpdated={fetchReturns}
        />
      )}
    </div>
  )
}
