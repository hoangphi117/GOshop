'use client'

import React, { useState, useEffect } from 'react'
import Link from 'next/link'
import { useAuthStore } from '@/stores/auth-store'
import { formatCurrency, formatDate } from '@/lib/format'
import { ShoppingBag, Clock, CheckCircle, Package, AlertCircle, ArrowRight } from 'lucide-react'

interface OrderItem {
  product_name: string
  quantity: number
  price: number
}

interface Order {
  uuid: string
  created_at: string
  status: string
  payment_status?: string
  total_amount: number
  shipping_address?: string
  items?: OrderItem[]
}

export default function MyOrdersPage() {
  const { accessToken } = useAuthStore()
  const [orders, setOrders] = useState<Order[]>([])
  const [loading, setLoading] = useState(true)
  const [filter, setFilter] = useState('ALL')

  useEffect(() => {
    if (!accessToken) return

    fetch('http://localhost:8080/api/v1/orders', {
      headers: {
        'Authorization': `Bearer ${accessToken}`,
        'X-API-KEY': 'abcd-1234-1111',
      },
    })
      .then((res) => (res.ok ? res.json() : null))
      .then((data) => {
        if (data?.data && Array.isArray(data.data)) {
          setOrders(data.data)
        } else {
          setOrders([])
        }
      })
      .catch(() => setOrders([]))
      .finally(() => setLoading(false))
  }, [accessToken])

  const filteredOrders = orders.filter((o) => {
    if (filter === 'ALL') return true
    return o.status?.toUpperCase() === filter
  })

  const getStatusBadge = (status: string) => {
    const s = (status || '').toUpperCase()
    if (s.includes('COMPLETE') || s.includes('DELIVER') || s.includes('PAID')) {
      return (
        <span className="px-2.5 py-1 rounded-full bg-emerald-50 text-emerald-700 text-[11px] font-bold inline-flex items-center gap-1 border border-emerald-200">
          <CheckCircle className="w-3 h-3" />
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
    return (
      <span className="px-2.5 py-1 rounded-full bg-blue-50 text-blue-700 text-[11px] font-bold inline-flex items-center gap-1 border border-blue-200">
        <Package className="w-3 h-3" />
        {status || 'Processing'}
      </span>
    )
  }

  if (loading) {
    return (
      <div className="p-8 rounded-3xl bg-white border border-slate-200/80 shadow-xs flex items-center justify-center min-h-[300px]">
        <div className="w-8 h-8 rounded-full border-2 border-[#2196F3] border-t-transparent animate-spin" />
      </div>
    )
  }

  return (
    <div className="p-6 sm:p-8 rounded-3xl bg-white border border-slate-200/80 shadow-xs space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-extrabold text-[#0D1B2A] tracking-tight">
            Order History
          </h1>
          <p className="text-xs text-slate-500 mt-1">
            Track and review all purchases, delivery updates, and invoices.
          </p>
        </div>

        {/* Filter Pills */}
        <div className="flex flex-wrap gap-1.5 p-1 bg-slate-100 rounded-xl">
          {['ALL', 'PENDING', 'PROCESSING', 'COMPLETED'].map((tab) => (
            <button
              key={tab}
              onClick={() => setFilter(tab)}
              className={`px-3 py-1.5 rounded-lg text-xs font-semibold transition-all ${
                filter === tab
                  ? 'bg-white text-[#0D47A1] shadow-xs'
                  : 'text-slate-600 hover:text-[#0D1B2A]'
              }`}
            >
              {tab === 'ALL' ? 'All Orders' : tab}
            </button>
          ))}
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
            You haven&apos;t placed any orders matching this filter yet.
          </p>
          <div className="pt-2">
            <Link
              href="/products"
              className="inline-flex items-center gap-1.5 px-4 py-2 rounded-xl bg-[#2196F3] text-white text-xs font-bold hover:bg-[#0D47A1] transition-colors"
            >
              <span>Explore Catalog</span>
              <ArrowRight className="w-3.5 h-3.5" />
            </Link>
          </div>
        </div>
      ) : (
        <div className="space-y-4">
          {filteredOrders.map((order) => (
            <div
              key={order.uuid}
              className="p-5 rounded-2xl border border-slate-200/80 bg-slate-50/40 hover:bg-slate-50 transition-colors space-y-3"
            >
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 pb-3 border-b border-slate-200/60 text-xs">
                <div>
                  <span className="text-slate-400 font-mono">ORDER #</span>
                  <span className="font-bold text-[#0D1B2A] ml-1">
                    {order.uuid.slice(0, 8).toUpperCase()}
                  </span>
                  <span className="text-slate-400 mx-2">•</span>
                  <span className="text-slate-500">
                    {formatDate(order.created_at)}
                  </span>
                </div>
                <div>{getStatusBadge(order.status)}</div>
              </div>

              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs">
                <div className="space-y-1">
                  <p className="text-slate-600 font-medium">
                    {order.items && order.items.length > 0
                      ? `${order.items[0].product_name} ${
                          order.items.length > 1
                            ? `+ ${order.items.length - 1} other item(s)`
                            : ''
                        }`
                      : 'Standard e-commerce shipment'}
                  </p>
                  {order.shipping_address && (
                    <p className="text-[11px] text-slate-400 truncate max-w-md">
                      Shipping to: {order.shipping_address}
                    </p>
                  )}
                </div>

                <div className="text-right">
                  <span className="text-slate-500 text-[11px] block">
                    Total Amount
                  </span>
                  <span className="text-base font-extrabold text-[#0D47A1]">
                    {formatCurrency(order.total_amount)}
                  </span>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
