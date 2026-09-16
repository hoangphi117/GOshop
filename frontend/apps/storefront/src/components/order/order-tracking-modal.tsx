'use client'

import React, { useState, useEffect, useCallback } from 'react'
import { authFetch } from '@/lib/api'
import { extractErrorMessage } from '@/lib/utils'
import {
  X,
  Truck,
  RefreshCw,
  Clock,
  CheckCircle2,
  Package,
  MapPin,
  AlertCircle,
  Loader2,
  ShieldCheck,
  Send,
} from 'lucide-react'
import { toast } from 'sonner'
import { TrackingEvent } from './return-detail-modal'

interface OrderTrackingModalProps {
  isOpen: boolean
  onClose: () => void
  orderUuid: string
  orderNumber?: string
  shippingCode?: string
  shippingProvider?: string
  orderStatus?: string
  onStatusUpdated?: () => void
}

interface TimelineResponse {
  order_uuid: string
  order_number: string
  events: TrackingEvent[]
}

export function OrderTrackingModal({
  isOpen,
  onClose,
  orderUuid,
  orderNumber,
  shippingCode,
  shippingProvider = 'GHN Express',
  orderStatus,
  onStatusUpdated,
}: OrderTrackingModalProps) {
  const [events, setEvents] = useState<TrackingEvent[]>([])
  const [loading, setLoading] = useState(true)
  const [isSyncing, setIsSyncing] = useState(false)
  const [errorMsg, setErrorMsg] = useState<string | null>(null)

  const fetchTimeline = useCallback(async () => {
    setLoading(true)
    setErrorMsg(null)
    try {
      const res = await authFetch(`/api/v1/orders/${orderUuid}/timeline`)
      if (!res.ok) {
        const err = await res.json().catch(() => null)
        setErrorMsg(extractErrorMessage(err, 'Failed to retrieve tracking events.'))
        return
      }
      const json = await res.json()
      const timelineData: TimelineResponse = json.data
      setEvents(timelineData.events || [])
    } catch {
      setErrorMsg('Network error while fetching tracking timeline.')
    } finally {
      setLoading(false)
    }
  }, [orderUuid])

  useEffect(() => {
    if (isOpen && orderUuid) {
      fetchTimeline()
    }
  }, [isOpen, orderUuid, fetchTimeline])

  if (!isOpen) return null

  const handleSyncTracking = async () => {
    setIsSyncing(true)
    try {
      const res = await authFetch(`/api/v1/orders/${orderUuid}/sync-tracking`, {
        method: 'POST',
      })
      if (!res.ok) {
        const err = await res.json().catch(() => null)
        toast.error(extractErrorMessage(err, 'Could not sync tracking with carrier.'))
        setIsSyncing(false)
        return
      }
      toast.success('Carrier tracking synchronized successfully!')
      fetchTimeline()
      onStatusUpdated?.()
    } catch {
      toast.error('Network error during tracking sync.')
    } finally {
      setIsSyncing(false)
    }
  }

  const getSourceBadge = (source: string) => {
    switch (source?.toLowerCase()) {
      case 'ghn':
        return (
          <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-amber-50 text-amber-700 border border-amber-200">
            GHN Carrier
          </span>
        )
      case 'admin':
        return (
          <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-blue-50 text-blue-700 border border-blue-200">
            Store Ops
          </span>
        )
      case 'user':
        return (
          <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-purple-50 text-purple-700 border border-purple-200">
            Customer
          </span>
        )
      default:
        return (
          <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-slate-100 text-slate-600 border border-slate-200">
            System
          </span>
        )
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 overflow-y-auto animate-in fade-in">
      <div className="w-full max-w-xl bg-white rounded-3xl p-6 shadow-2xl space-y-5 border border-slate-100 max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-start justify-between gap-3 border-b border-slate-100 pb-4">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-2xl bg-blue-50 text-[#2196F3] flex items-center justify-center shrink-0 shadow-2xs">
              <Truck className="w-5 h-5" />
            </div>
            <div>
              <h2 className="text-lg font-black text-[#0D1B2A]">
                Shipment Tracking
              </h2>
              <p className="text-xs text-slate-500">
                Order #{orderNumber || orderUuid.slice(0, 8).toUpperCase()}
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1.5 rounded-full hover:bg-slate-100 text-slate-400 hover:text-slate-600 transition-colors cursor-pointer"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Carrier Info Bar */}
        <div className="p-4 rounded-2xl bg-slate-50 border border-slate-200/80 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 text-xs">
          <div className="space-y-1">
            <div className="flex items-center gap-2">
              <span className="text-slate-500">Carrier:</span>
              <span className="font-bold text-[#0D1B2A]">{shippingProvider}</span>
            </div>
            {shippingCode ? (
              <div className="flex items-center gap-2">
                <span className="text-slate-500">Waybill / Tracking:</span>
                <span className="font-mono font-bold text-[#0D47A1] bg-blue-50/80 px-2 py-0.5 rounded-lg border border-blue-100">
                  {shippingCode}
                </span>
              </div>
            ) : (
              <div className="flex items-center gap-2">
                <span className="text-slate-500">Status:</span>
                <span className="font-bold text-slate-700">{orderStatus || 'Processing'}</span>
              </div>
            )}
          </div>

          <button
            type="button"
            disabled={isSyncing || loading}
            onClick={handleSyncTracking}
            className="px-3.5 py-2 rounded-xl bg-white border border-slate-200 hover:border-[#2196F3] hover:text-[#2196F3] text-slate-700 text-xs font-bold transition-all shadow-2xs flex items-center gap-1.5 cursor-pointer disabled:opacity-50"
            title="Fetch latest updates from GHN"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${isSyncing ? 'animate-spin text-[#2196F3]' : ''}`} />
            <span>{isSyncing ? 'Syncing...' : 'Sync Carrier'}</span>
          </button>
        </div>

        {/* Timeline Content */}
        {loading ? (
          <div className="py-16 flex flex-col items-center justify-center space-y-3">
            <Loader2 className="w-8 h-8 text-[#2196F3] animate-spin" />
            <p className="text-xs text-slate-500 font-medium">Loading tracking timeline...</p>
          </div>
        ) : errorMsg ? (
          <div className="p-4 rounded-2xl bg-rose-50 border border-rose-100 text-xs text-rose-600 flex items-start gap-3">
            <AlertCircle className="w-5 h-5 shrink-0" />
            <div>
              <p className="font-bold">Could not load tracking information</p>
              <p className="text-rose-500 mt-0.5">{errorMsg}</p>
            </div>
          </div>
        ) : events.length === 0 ? (
          <div className="py-12 text-center space-y-2">
            <Package className="w-10 h-10 text-slate-300 mx-auto" />
            <p className="text-xs font-bold text-slate-700">No tracking events recorded yet</p>
            <p className="text-[11px] text-slate-400">
              Tracking updates will appear here once the shipping partner processes your package.
            </p>
          </div>
        ) : (
          <div className="space-y-4">
            <h3 className="text-xs font-bold text-[#0D1B2A] uppercase tracking-wider">
              Activity History ({events.length} updates)
            </h3>

            <div className="relative pl-6 space-y-5 before:absolute before:left-2.5 before:top-2 before:bottom-2 before:w-0.5 before:bg-slate-200">
              {events.map((evt, idx) => {
                const isLatest = idx === 0
                return (
                  <div key={evt.event_id || idx} className="relative group">
                    <div
                      className={`absolute -left-6 top-1 w-3.5 h-3.5 rounded-full border-2 border-white shadow-2xs transition-all ${
                        isLatest
                          ? 'bg-emerald-500 ring-4 ring-emerald-100'
                          : 'bg-[#2196F3]'
                      }`}
                    />
                    <div className="bg-slate-50/70 group-hover:bg-slate-50 p-3.5 rounded-2xl border border-slate-200/70 transition-colors space-y-1">
                      <div className="flex items-center justify-between gap-2 flex-wrap">
                        <div className="flex items-center gap-2">
                          <span className="text-xs font-black text-[#0D1B2A]">
                            {evt.status_text && !evt.status_text.includes('_')
                              ? evt.status_text
                              : evt.status === 'return_shipping_submitted'
                                ? 'Return waybill submitted'
                                : evt.status_text || evt.status}
                          </span>
                          {getSourceBadge(evt.source)}
                        </div>
                        <span className="text-[11px] font-semibold text-slate-400">
                          {new Date(evt.created_at).toLocaleString('en-US', {
                            dateStyle: 'medium',
                            timeStyle: 'short',
                          })}
                        </span>
                      </div>
                      {evt.note && (
                        <p className="text-xs text-slate-600 leading-relaxed pt-0.5">
                          {evt.note.replace(
                            /User submitted return shipping info for/gi,
                            'Return waybill submitted for'
                          )}
                        </p>
                      )}
                    </div>
                  </div>
                )
              })}
            </div>
          </div>
        )}

        <div className="pt-2 border-t border-slate-100 flex justify-end">
          <button
            type="button"
            onClick={onClose}
            className="px-5 py-2.5 rounded-xl bg-slate-100 hover:bg-slate-200 text-slate-700 text-xs font-bold transition-all cursor-pointer"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  )
}
