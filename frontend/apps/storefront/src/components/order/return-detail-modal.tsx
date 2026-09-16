'use client'

import React, { useState, useEffect, useCallback } from 'react'
import { authFetch } from '@/lib/api'
import { formatCurrency } from '@/lib/format'
import { extractErrorMessage } from '@/lib/utils'
import {
  X,
  RotateCcw,
  Truck,
  Building2,
  Calendar,
  AlertCircle,
  CheckCircle2,
  Clock,
  XCircle,
  Loader2,
  Image as ImageIcon,
  Film,
  Send,
  Edit2,
  ShieldCheck,
  Package,
} from 'lucide-react'
import { toast } from 'sonner'

export interface TrackingEvent {
  event_id: number
  status: string
  status_text: string
  note?: string
  source: string
  created_at: string
}

export interface ReturnDetailData {
  return_uuid: string
  order_uuid: string
  order_number: string
  reason: string
  images: string[]
  video_url?: string | null
  status: number // 1: Pending, 2: Approved, 3: Rejected, 4: Completed, 5: Cancelled
  status_name: string
  status_text: string
  admin_note?: string | null
  shipping_provider?: string | null
  shipping_code?: string | null
  shipping_note?: string | null
  bank_name?: string | null
  bank_account?: string | null
  bank_account_name?: string | null
  total_amount: number
  created_at: string
  updated_at: string
  events?: TrackingEvent[]
}

interface ReturnDetailModalProps {
  isOpen: boolean
  onClose: () => void
  orderUuid: string
  onUpdated?: () => void
}

const COMMON_RETURN_COURIERS = [
  'GHN Express (Giao Hàng Nhanh)',
  'GHTK (Giao Hàng Tiết Kiệm)',
  'Viettel Post',
  'VNPost (Vietnam Post)',
  'J&T Express',
  'Ninja Van',
  'BEST Express',
  'Other Carrier',
]

const VIETNAM_BANKS = [
  'Vietcombank',
  'Techcombank',
  'MB Bank (Military Bank)',
  'ACB (Asia Commercial Bank)',
  'VPBank',
  'BIDV',
  'VietinBank',
  'TPBank',
  'Sacombank',
  'VIB',
  'HDBank',
  'MSB (Maritime Bank)',
  'SHB',
  'SeABank',
  'Agribank',
  'MoMo E-Wallet Account',
  'Other Bank',
]

export function ReturnDetailModal({
  isOpen,
  onClose,
  orderUuid,
  onUpdated,
}: ReturnDetailModalProps) {
  const [data, setData] = useState<ReturnDetailData | null>(null)
  const [loading, setLoading] = useState(true)
  const [errorMsg, setErrorMsg] = useState<string | null>(null)

  // Sub-actions state
  const [isCancelling, setIsCancelling] = useState(false)
  const [cancelReason, setCancelReason] = useState('')
  const [showCancelPrompt, setShowCancelPrompt] = useState(false)

  // Return shipment form
  const [shippingProvider, setShippingProvider] = useState(COMMON_RETURN_COURIERS[0])
  const [shippingCode, setShippingCode] = useState('')
  const [shippingNote, setShippingNote] = useState('')
  const [isSubmittingShipping, setIsSubmittingShipping] = useState(false)

  // Edit Bank Account form
  const [isEditingBank, setIsEditingBank] = useState(false)
  const [editBankName, setEditBankName] = useState('')
  const [editBankAccount, setEditBankAccount] = useState('')
  const [editBankAccountName, setEditBankAccountName] = useState('')
  const [isSavingBank, setIsSavingBank] = useState(false)

  const fetchReturnDetail = useCallback(async () => {
    setLoading(true)
    setErrorMsg(null)
    try {
      const res = await authFetch(`/api/v1/orders/${orderUuid}/return`)
      if (!res.ok) {
        const err = await res.json().catch(() => null)
        const msg = extractErrorMessage(err, 'Could not retrieve return details.')
        setErrorMsg(msg)
        return
      }
      const json = await res.json()
      const returnData: ReturnDetailData = json.data
      setData(returnData)
      if (returnData.bank_name) setEditBankName(returnData.bank_name)
      if (returnData.bank_account) setEditBankAccount(returnData.bank_account)
      if (returnData.bank_account_name) setEditBankAccountName(returnData.bank_account_name)
    } catch {
      setErrorMsg('Failed to load return details. Please check your network.')
    } finally {
      setLoading(false)
    }
  }, [orderUuid])

  useEffect(() => {
    if (isOpen && orderUuid) {
      fetchReturnDetail()
    }
  }, [isOpen, orderUuid, fetchReturnDetail])

  if (!isOpen) return null

  const handleCancelReturn = async () => {
    if (!data) return
    setIsCancelling(true)
    try {
      const res = await authFetch(`/api/v1/orders/${orderUuid}/return/cancel`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ reason: cancelReason.trim() || undefined }),
      })
      if (!res.ok) {
        const err = await res.json().catch(() => null)
        toast.error(extractErrorMessage(err, 'Failed to cancel return request.'))
        setIsCancelling(false)
        return
      }
      toast.success('Return request cancelled successfully.')
      setShowCancelPrompt(false)
      fetchReturnDetail()
      onUpdated?.()
    } catch {
      toast.error('Network error while cancelling return.')
    } finally {
      setIsCancelling(false)
    }
  }

  const handleSubmitReturnShipping = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!shippingCode.trim()) {
      toast.error('Please enter the tracking code provided by your courier.')
      return
    }
    setIsSubmittingShipping(true)
    try {
      const res = await authFetch(`/api/v1/orders/${orderUuid}/return/shipping`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          shipping_provider: shippingProvider,
          shipping_code: shippingCode.trim(),
          note: shippingNote.trim() || undefined,
        }),
      })
      if (!res.ok) {
        const err = await res.json().catch(() => null)
        toast.error(extractErrorMessage(err, 'Failed to submit return shipping details.'))
        setIsSubmittingShipping(false)
        return
      }
      toast.success('Return shipping information saved!')
      fetchReturnDetail()
      onUpdated?.()
    } catch {
      toast.error('Network error. Please try again.')
    } finally {
      setIsSubmittingShipping(false)
    }
  }

  const handleSaveBankInfo = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!editBankName || !editBankAccount || !editBankAccountName) {
      toast.error('Please fill in all bank details.')
      return
    }
    setIsSavingBank(true)
    try {
      const res = await authFetch(`/api/v1/orders/${orderUuid}/return/bank-account`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          bank_name: editBankName.trim(),
          bank_account: editBankAccount.trim(),
          bank_account_name: editBankAccountName.trim().toUpperCase(),
        }),
      })
      if (!res.ok) {
        const err = await res.json().catch(() => null)
        toast.error(extractErrorMessage(err, 'Failed to update bank details.'))
        setIsSavingBank(false)
        return
      }
      toast.success('Refund bank account updated!')
      setIsEditingBank(false)
      fetchReturnDetail()
      onUpdated?.()
    } catch {
      toast.error('Network error. Please try again.')
    } finally {
      setIsSavingBank(false)
    }
  }

  const getStatusBadge = (status: number, statusText: string) => {
    switch (status) {
      case 1: // Pending
        return (
          <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-bold bg-amber-50 text-amber-700 border border-amber-200">
            <Clock className="w-3.5 h-3.5" />
            <span>{statusText || 'Pending Approval'}</span>
          </span>
        )
      case 2: // Approved
        return (
          <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-bold bg-blue-50 text-blue-700 border border-blue-200">
            <CheckCircle2 className="w-3.5 h-3.5" />
            <span>{statusText || 'Approved (Awaiting Return Shipment)'}</span>
          </span>
        )
      case 3: // Rejected
        return (
          <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-bold bg-rose-50 text-rose-700 border border-rose-200">
            <XCircle className="w-3.5 h-3.5" />
            <span>{statusText || 'Rejected'}</span>
          </span>
        )
      case 4: // Completed
        return (
          <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-bold bg-emerald-50 text-emerald-700 border border-emerald-200">
            <ShieldCheck className="w-3.5 h-3.5" />
            <span>{statusText || 'Completed & Refunded'}</span>
          </span>
        )
      case 5: // Cancelled
        return (
          <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-bold bg-slate-100 text-slate-600 border border-slate-200">
            <X className="w-3.5 h-3.5" />
            <span>{statusText || 'Cancelled'}</span>
          </span>
        )
      default:
        return (
          <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-bold bg-slate-100 text-slate-700">
            <span>{statusText || 'Processing'}</span>
          </span>
        )
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 overflow-y-auto animate-in fade-in">
      <div className="w-full max-w-2xl bg-white rounded-3xl p-6 shadow-2xl space-y-5 border border-slate-100 max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-start justify-between gap-3 border-b border-slate-100 pb-4">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-2xl bg-amber-50 text-amber-600 flex items-center justify-center shrink-0 shadow-2xs">
              <RotateCcw className="w-5 h-5" />
            </div>
            <div>
              <div className="flex items-center gap-2 flex-wrap">
                <h2 className="text-lg font-black text-[#0D1B2A]">
                  Return Request #{data?.return_uuid.slice(0, 8).toUpperCase() || '...'}
                </h2>
                {data && getStatusBadge(data.status, data.status_text)}
              </div>
              <p className="text-xs text-slate-500 mt-0.5">
                For Order #{data?.order_number || orderUuid.slice(0, 8).toUpperCase()}
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

        {loading ? (
          <div className="py-16 flex flex-col items-center justify-center space-y-3">
            <Loader2 className="w-8 h-8 text-[#2196F3] animate-spin" />
            <p className="text-xs text-slate-500 font-medium">Loading return details...</p>
          </div>
        ) : errorMsg ? (
          <div className="p-4 rounded-2xl bg-rose-50 border border-rose-100 text-xs text-rose-600 flex items-start gap-3">
            <AlertCircle className="w-5 h-5 shrink-0" />
            <div>
              <p className="font-bold">Could not load return request</p>
              <p className="text-rose-500 mt-0.5">{errorMsg}</p>
            </div>
          </div>
        ) : data ? (
          <div className="space-y-5 text-xs">
            {/* Admin Note if any */}
            {data.admin_note && (
              <div className="p-4 rounded-2xl bg-amber-50/70 border border-amber-200/80 space-y-1.5">
                <div className="flex items-center gap-2 font-bold text-amber-900">
                  <AlertCircle className="w-4 h-4 text-amber-600" />
                  <span>Shop Response / Note:</span>
                </div>
                <p className="text-slate-700 leading-relaxed pl-6">
                  {data.admin_note}
                </p>
              </div>
            )}

            {/* Request Summary Card */}
            <div className="p-4 rounded-2xl bg-slate-50 border border-slate-200/70 space-y-3">
              <div className="flex justify-between items-center pb-2 border-b border-slate-200/60">
                <span className="font-bold text-[#0D1B2A]">Return Reason & Description</span>
                <span className="text-slate-400 font-medium">
                  {new Date(data.created_at).toLocaleString('en-US', {
                    dateStyle: 'medium',
                    timeStyle: 'short',
                  })}
                </span>
              </div>
              <p className="text-slate-700 whitespace-pre-line leading-relaxed">
                {data.reason}
              </p>

              {/* Uploaded Media */}
              {(data.images?.length > 0 || data.video_url) && (
                <div className="pt-2 space-y-1.5">
                  <span className="font-semibold text-slate-500">Submitted Proof Media:</span>
                  <div className="flex gap-2 flex-wrap pt-1">
                    {data.images?.map((img, idx) => (
                      <a
                        key={idx}
                        href={img.startsWith('http') ? img : `${process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'}${img}`}
                        target="_blank"
                        rel="noreferrer"
                        className="w-16 h-16 rounded-xl border border-slate-200 overflow-hidden group relative block shadow-2xs hover:border-[#2196F3] transition-all"
                      >
                        {/* eslint-disable-next-line @next/next/no-img-element */}
                        <img
                          src={img.startsWith('http') ? img : `${process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'}${img}`}
                          alt={`Proof ${idx + 1}`}
                          className="w-full h-full object-cover group-hover:scale-105 transition-transform"
                        />
                      </a>
                    ))}
                    {data.video_url && (
                      <a
                        href={data.video_url.startsWith('http') ? data.video_url : `${process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'}${data.video_url}`}
                        target="_blank"
                        rel="noreferrer"
                        className="w-16 h-16 rounded-xl border border-slate-200 bg-slate-900 flex flex-col items-center justify-center text-amber-400 hover:border-amber-400 transition-all shadow-2xs"
                      >
                        <Film className="w-5 h-5" />
                        <span className="text-[9px] font-bold text-white mt-1">Video</span>
                      </a>
                    )}
                  </div>
                </div>
              )}
            </div>

            {/* Return Shipment Section (Required if Approved) */}
            {data.status === 2 && (
              <div className="p-4 rounded-2xl bg-blue-50/50 border border-blue-200 space-y-3">
                <div className="flex items-center gap-2 text-blue-900 font-bold">
                  <Truck className="w-4 h-4 text-[#2196F3]" />
                  <span>Return Shipping Information</span>
                </div>
                <p className="text-slate-600 text-[11px] leading-relaxed">
                  Your return request is approved! Please ship the product back to the merchant and enter your courier name & tracking code below.
                </p>

                {data.shipping_code ? (
                  <div className="p-3 bg-white rounded-xl border border-blue-100 space-y-1">
                    <div className="flex justify-between">
                      <span className="text-slate-500">Carrier:</span>
                      <span className="font-bold text-[#0D1B2A]">{data.shipping_provider}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-slate-500">Tracking Code:</span>
                      <span className="font-mono font-bold text-[#0D47A1]">{data.shipping_code}</span>
                    </div>
                    {data.shipping_note && (
                      <div className="flex justify-between">
                        <span className="text-slate-500">Note:</span>
                        <span className="text-slate-700">{data.shipping_note}</span>
                      </div>
                    )}
                  </div>
                ) : (
                  <form onSubmit={handleSubmitReturnShipping} className="space-y-2.5 pt-1">
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                      <div>
                        <label className="text-[11px] font-semibold text-slate-600">Shipping Provider</label>
                        <select
                          value={shippingProvider}
                          onChange={(e) => setShippingProvider(e.target.value)}
                          className="w-full px-3 py-2 rounded-xl border border-slate-200 bg-white text-xs text-slate-800 focus:outline-none focus:border-[#2196F3]"
                        >
                          {COMMON_RETURN_COURIERS.map((c) => (
                            <option key={c} value={c}>
                              {c}
                            </option>
                          ))}
                        </select>
                      </div>

                      <div>
                        <label className="text-[11px] font-semibold text-slate-600">Tracking / Waybill Number *</label>
                        <input
                          type="text"
                          required
                          value={shippingCode}
                          onChange={(e) => setShippingCode(e.target.value)}
                          placeholder="e.g. GHN123456789"
                          className="w-full px-3 py-2 rounded-xl border border-slate-200 bg-white font-mono text-xs text-slate-800 focus:outline-none focus:border-[#2196F3]"
                        />
                      </div>
                    </div>

                    <div>
                      <label className="text-[11px] font-semibold text-slate-600">Note (Optional)</label>
                      <input
                        type="text"
                        value={shippingNote}
                        onChange={(e) => setShippingNote(e.target.value)}
                        placeholder="e.g. Package sent on morning of..."
                        className="w-full px-3 py-2 rounded-xl border border-slate-200 bg-white text-xs text-slate-800 focus:outline-none focus:border-[#2196F3]"
                      />
                    </div>

                    <button
                      type="submit"
                      disabled={isSubmittingShipping}
                      className="px-4 py-2 rounded-xl bg-[#2196F3] hover:bg-[#0D47A1] text-white text-xs font-bold transition-all shadow-xs flex items-center gap-1.5 cursor-pointer disabled:opacity-50"
                    >
                      {isSubmittingShipping ? (
                        <Loader2 className="w-3.5 h-3.5 animate-spin" />
                      ) : (
                        <Send className="w-3.5 h-3.5" />
                      )}
                      <span>Submit Return Tracking</span>
                    </button>
                  </form>
                )}
              </div>
            )}

            {/* Refund Bank Info */}
            <div className="p-4 rounded-2xl bg-slate-50 border border-slate-200/70 space-y-2.5">
              <div className="flex justify-between items-center">
                <div className="flex items-center gap-2 font-bold text-[#0D1B2A]">
                  <Building2 className="w-4 h-4 text-emerald-600" />
                  <span>Refund Bank Information</span>
                </div>
                {data.status < 4 && !isEditingBank && (
                  <button
                    type="button"
                    onClick={() => setIsEditingBank(true)}
                    className="text-[11px] font-bold text-[#2196F3] hover:underline flex items-center gap-1 cursor-pointer"
                  >
                    <Edit2 className="w-3 h-3" />
                    <span>{data.bank_account ? 'Update Bank' : 'Add Bank'}</span>
                  </button>
                )}
              </div>

              {isEditingBank ? (
                <form onSubmit={handleSaveBankInfo} className="space-y-2.5 pt-1">
                  <div className="space-y-1">
                    <label className="text-[11px] font-semibold text-slate-600">Bank Name</label>
                    <input
                      type="text"
                      list="edit-vietnam-banks"
                      required
                      value={editBankName}
                      onChange={(e) => setEditBankName(e.target.value)}
                      placeholder="e.g. Vietcombank"
                      className="w-full px-3 py-1.5 rounded-xl border border-slate-200 bg-white text-xs"
                    />
                    <datalist id="edit-vietnam-banks">
                      {VIETNAM_BANKS.map((b) => (
                        <option key={b} value={b} />
                      ))}
                    </datalist>
                  </div>
                  <div className="grid grid-cols-2 gap-2">
                    <div className="space-y-1">
                      <label className="text-[11px] font-semibold text-slate-600">Account Number</label>
                      <input
                        type="text"
                        required
                        value={editBankAccount}
                        onChange={(e) => setEditBankAccount(e.target.value)}
                        placeholder="e.g. 1903456789"
                        className="w-full px-3 py-1.5 rounded-xl border border-slate-200 bg-white font-mono text-xs"
                      />
                    </div>
                    <div className="space-y-1">
                      <label className="text-[11px] font-semibold text-slate-600">Account Name</label>
                      <input
                        type="text"
                        required
                        value={editBankAccountName}
                        onChange={(e) => setEditBankAccountName(e.target.value)}
                        placeholder="e.g. NGUYEN VAN A"
                        className="w-full px-3 py-1.5 rounded-xl border border-slate-200 bg-white uppercase text-xs"
                      />
                    </div>
                  </div>
                  <div className="flex gap-2 justify-end pt-1">
                    <button
                      type="button"
                      onClick={() => setIsEditingBank(false)}
                      className="px-3 py-1.5 rounded-lg bg-slate-200 text-slate-700 text-xs font-semibold"
                    >
                      Cancel
                    </button>
                    <button
                      type="submit"
                      disabled={isSavingBank}
                      className="px-3 py-1.5 rounded-lg bg-emerald-600 text-white text-xs font-bold shadow-xs disabled:opacity-50"
                    >
                      {isSavingBank ? 'Saving...' : 'Save Bank Details'}
                    </button>
                  </div>
                </form>
              ) : (
                <div className="space-y-1 text-slate-600">
                  <div className="flex justify-between">
                    <span className="text-slate-500">Bank Name:</span>
                    <span className="font-semibold text-slate-800">{data.bank_name || 'Not provided'}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-slate-500">Account Number:</span>
                    <span className="font-mono font-semibold text-slate-800">{data.bank_account || 'Not provided'}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-slate-500">Account Holder:</span>
                    <span className="font-semibold uppercase text-slate-800">{data.bank_account_name || 'Not provided'}</span>
                  </div>
                </div>
              )}
            </div>

            {/* Tracking Events Timeline */}
            {data.events && data.events.length > 0 && (
              <div className="p-4 rounded-2xl bg-white border border-slate-200/80 space-y-3">
                <span className="font-bold text-[#0D1B2A] block">Return Progress Timeline</span>
                <div className="relative pl-6 space-y-4 before:absolute before:left-2 before:top-2 before:bottom-2 before:w-0.5 before:bg-slate-200">
                  {data.events.map((evt, idx) => {
                    const isFinished =
                      (evt.status || '').toLowerCase().includes('returned') ||
                      (evt.status || '').toLowerCase().includes('complete') ||
                      (evt.status || '').toLowerCase().includes('refunded') ||
                      (data.status === 4 && idx === 0)

                    return (
                      <div key={idx} className="relative">
                        {isFinished ? (
                          <div className="absolute -left-7 top-0.5 w-5 h-5 rounded-full bg-emerald-500 text-white flex items-center justify-center shadow-xs">
                            <CheckCircle2 className="w-3.5 h-3.5" />
                          </div>
                        ) : (
                          <div className="absolute -left-6 top-1 w-3 h-3 rounded-full bg-[#2196F3] border-2 border-white shadow-2xs" />
                        )}
                        <div className="space-y-0.5">
                        <div className="flex items-center justify-between">
                          <span className="font-bold text-slate-900">
                            {evt.status_text && !evt.status_text.includes('_')
                              ? evt.status_text
                              : evt.status === 'return_shipping_submitted'
                                ? 'Return waybill submitted'
                                : evt.status_text || evt.status}
                          </span>
                          <span className="text-[11px] text-slate-400">
                            {new Date(evt.created_at).toLocaleString('en-US', {
                              dateStyle: 'short',
                              timeStyle: 'short',
                            })}
                          </span>
                        </div>
                          {evt.note && (
                            <p className="text-slate-600 text-[11px]">
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

            {/* Cancel Return Button for Pending */}
            {data.status === 1 && (
              <div className="pt-2">
                {showCancelPrompt ? (
                  <div className="p-4 rounded-2xl bg-rose-50 border border-rose-200 space-y-2">
                    <p className="font-bold text-rose-800">Are you sure you want to cancel this return request?</p>
                    <input
                      type="text"
                      value={cancelReason}
                      onChange={(e) => setCancelReason(e.target.value)}
                      placeholder="Optional cancellation note..."
                      className="w-full px-3 py-2 rounded-xl border border-rose-200 bg-white text-xs"
                    />
                    <div className="flex gap-2 justify-end pt-1">
                      <button
                        type="button"
                        onClick={() => setShowCancelPrompt(false)}
                        className="px-3 py-1.5 rounded-xl bg-slate-200 text-slate-700 text-xs font-semibold"
                      >
                        Keep Request
                      </button>
                      <button
                        type="button"
                        disabled={isCancelling}
                        onClick={handleCancelReturn}
                        className="px-3 py-1.5 rounded-xl bg-rose-600 hover:bg-rose-700 text-white text-xs font-bold disabled:opacity-50"
                      >
                        {isCancelling ? 'Cancelling...' : 'Confirm Cancellation'}
                      </button>
                    </div>
                  </div>
                ) : (
                  <button
                    type="button"
                    onClick={() => setShowCancelPrompt(true)}
                    className="w-full py-2.5 rounded-xl border border-rose-200 text-rose-600 hover:bg-rose-50 text-xs font-bold transition-all cursor-pointer"
                  >
                    Cancel Return Request
                  </button>
                )}
              </div>
            )}
          </div>
        ) : null}
      </div>
    </div>
  )
}
