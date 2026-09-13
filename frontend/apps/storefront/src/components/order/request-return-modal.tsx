'use client'

import React, { useState, useRef } from 'react'
import { authFetch } from '@/lib/api'
import { extractErrorMessage } from '@/lib/utils'
import {
  X,
  RotateCcw,
  Upload,
  Image as ImageIcon,
  Film,
  Trash2,
  Loader2,
  AlertCircle,
  Building2,
  CreditCard,
  User,
  HelpCircle,
} from 'lucide-react'
import { toast } from 'sonner'

interface RequestReturnModalProps {
  isOpen: boolean
  onClose: () => void
  orderUuid: string
  orderNumber?: string
  paymentMethod?: string
  totalAmount?: number
  onSuccess?: () => void
}

const COMMON_REASONS = [
  'Damaged or defective item upon arrival',
  'Incorrect product or wrong variant received',
  'Item does not match website description',
  'Missing parts, accessories, or free gifts',
  'Quality issue / Product is not functioning properly',
  'Other reason',
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

export function RequestReturnModal({
  isOpen,
  onClose,
  orderUuid,
  orderNumber,
  paymentMethod,
  totalAmount,
  onSuccess,
}: RequestReturnModalProps) {
  const [selectedReasonPrefix, setSelectedReasonPrefix] = useState(COMMON_REASONS[0])
  const [detailedReason, setDetailedReason] = useState('')
  const [bankName, setBankName] = useState('')
  const [bankAccount, setBankAccount] = useState('')
  const [bankAccountName, setBankAccountName] = useState('')
  const [files, setFiles] = useState<File[]>([])
  const [previews, setPreviews] = useState<Array<{ url: string; isVideo: boolean; file: File }>>([])
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [errorMsg, setErrorMsg] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  if (!isOpen) return null

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const selected = Array.from(e.target.files || [])
    if (!selected.length) return

    let currentImages = previews.filter((p) => !p.isVideo).length
    let currentVideos = previews.filter((p) => p.isVideo).length

    const validNewFiles: File[] = []
    const newPreviews: Array<{ url: string; isVideo: boolean; file: File }> = []

    for (const file of selected) {
      const isImg = file.type.startsWith('image/')
      const isVid = file.type.startsWith('video/')

      if (!isImg && !isVid) {
        toast.error(`Unsupported file type: ${file.name}`)
        continue
      }

      if (file.size > 25 * 1024 * 1024) {
        toast.error(`File ${file.name} exceeds 25MB limit.`)
        continue
      }

      if (isImg) {
        if (currentImages >= 5) {
          toast.warning('Maximum 5 proof images allowed.')
          continue
        }
        currentImages++
      } else if (isVid) {
        if (currentVideos >= 1) {
          toast.warning('Maximum 1 video allowed.')
          continue
        }
        currentVideos++
      }

      validNewFiles.push(file)
      newPreviews.push({
        url: URL.createObjectURL(file),
        isVideo: isVid,
        file,
      })
    }

    setFiles((prev) => [...prev, ...validNewFiles])
    setPreviews((prev) => [...prev, ...newPreviews])

    if (fileInputRef.current) {
      fileInputRef.current.value = ''
    }
  }

  const handleRemoveFile = (index: number) => {
    const target = previews[index]
    if (target?.url) {
      URL.revokeObjectURL(target.url)
    }
    setPreviews((prev) => prev.filter((_, i) => i !== index))
    setFiles((prev) => prev.filter((_, i) => i !== index))
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setErrorMsg(null)

    const fullReason =
      selectedReasonPrefix === 'Other reason'
        ? detailedReason.trim()
        : detailedReason.trim()
        ? `${selectedReasonPrefix}: ${detailedReason.trim()}`
        : selectedReasonPrefix

    if (fullReason.length < 10) {
      setErrorMsg('Please provide a detailed explanation of the issue (minimum 10 characters).')
      return
    }

    setIsSubmitting(true)

    try {
      const formData = new FormData()
      formData.append('reason', fullReason)

      if (bankName.trim()) formData.append('bank_name', bankName.trim())
      if (bankAccount.trim()) formData.append('bank_account', bankAccount.trim())
      if (bankAccountName.trim()) formData.append('bank_account_name', bankAccountName.trim().toUpperCase())

      files.forEach((file) => {
        formData.append('files', file)
      })

      const res = await authFetch(`/api/v1/orders/${orderUuid}/return`, {
        method: 'POST',
        body: formData,
      })

      if (!res.ok) {
        const errJson = await res.json().catch(() => null)
        const msg = extractErrorMessage(errJson, 'Failed to submit return request.')
        setErrorMsg(msg)
        toast.error(msg)
        setIsSubmitting(false)
        return
      }

      toast.success('Return & refund request submitted successfully! Our support team will review it shortly.')
      onSuccess?.()
      onClose()
    } catch {
      setErrorMsg('Network error. Please try again.')
      toast.error('Network error. Please try again.')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 overflow-y-auto animate-in fade-in">
      <div className="w-full max-w-xl bg-white rounded-3xl p-6 shadow-2xl space-y-5 border border-slate-100 max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-start justify-between gap-3 border-b border-slate-100 pb-4">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-2xl bg-amber-50 text-amber-600 flex items-center justify-center shrink-0 shadow-2xs">
              <RotateCcw className="w-5 h-5" />
            </div>
            <div>
              <h2 className="text-lg font-black text-[#0D1B2A]">
                Request Return & Refund
              </h2>
              <p className="text-xs text-slate-500">
                Order #{orderNumber || orderUuid.slice(0, 8).toUpperCase()}
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1.5 rounded-full hover:bg-slate-100 text-slate-400 hover:text-slate-600 transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {errorMsg && (
          <div className="p-3.5 rounded-2xl bg-rose-50 border border-rose-100 text-xs text-rose-600 flex items-start gap-2.5">
            <AlertCircle className="w-4 h-4 shrink-0 mt-0.5" />
            <span>{errorMsg}</span>
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          {/* Reason Selection */}
          <div className="space-y-1.5">
            <label className="text-xs font-bold text-[#0D1B2A] flex items-center gap-1.5">
              <span>Reason for Return</span>
              <span className="text-rose-500">*</span>
            </label>
            <select
              value={selectedReasonPrefix}
              onChange={(e) => setSelectedReasonPrefix(e.target.value)}
              className="w-full px-3.5 py-2.5 rounded-xl border border-slate-200 bg-white text-xs font-semibold text-slate-800 focus:outline-none focus:border-[#2196F3] focus:ring-2 focus:ring-[#2196F3]/15 transition-all"
            >
              {COMMON_REASONS.map((r) => (
                <option key={r} value={r}>
                  {r}
                </option>
              ))}
            </select>
          </div>

          {/* Detailed Reason Textarea */}
          <div className="space-y-1.5">
            <label className="text-xs font-bold text-[#0D1B2A] flex items-center justify-between">
              <span>Detailed Explanation</span>
              <span className="text-[11px] font-normal text-slate-400">
                {detailedReason.length}/1000 (Min 10 characters)
              </span>
            </label>
            <textarea
              rows={3}
              value={detailedReason}
              onChange={(e) => setDetailedReason(e.target.value)}
              placeholder="Please describe the defect or issue in detail to help our team process your request quickly..."
              maxLength={1000}
              className="w-full p-3.5 rounded-2xl border border-slate-200 text-xs text-slate-800 placeholder:text-slate-400 focus:outline-none focus:border-[#2196F3] focus:ring-2 focus:ring-[#2196F3]/15 resize-none transition-all"
            />
          </div>

          {/* Bank Info for Refund */}
          <div className="p-4 rounded-2xl bg-slate-50 border border-slate-200/80 space-y-3">
            <div className="flex items-center gap-2 text-xs font-bold text-[#0D1B2A]">
              <Building2 className="w-4 h-4 text-[#2196F3]" />
              <span>Refund Bank Account Information (Optional / for COD)</span>
            </div>
            <p className="text-[11px] text-slate-500 leading-relaxed">
              If your order was paid via Cash on Delivery (COD), providing your bank account details ensures prompt refund settlement upon approval.
            </p>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
              <div className="space-y-1 sm:col-span-2">
                <label className="text-[11px] font-semibold text-slate-600">Bank Name</label>
                <div className="relative">
                  <input
                    type="text"
                    list="vietnam-banks-list"
                    value={bankName}
                    onChange={(e) => setBankName(e.target.value)}
                    placeholder="e.g. Vietcombank, Techcombank, MB Bank..."
                    className="w-full px-3 py-2 rounded-xl border border-slate-200 bg-white text-xs text-slate-800 focus:outline-none focus:border-[#2196F3]"
                  />
                  <datalist id="vietnam-banks-list">
                    {VIETNAM_BANKS.map((b) => (
                      <option key={b} value={b} />
                    ))}
                  </datalist>
                </div>
              </div>

              <div className="space-y-1">
                <label className="text-[11px] font-semibold text-slate-600">Account Number</label>
                <div className="relative">
                  <input
                    type="text"
                    value={bankAccount}
                    onChange={(e) => setBankAccount(e.target.value.replace(/[^0-9A-Za-z]/g, ''))}
                    placeholder="e.g. 190345678910"
                    className="w-full px-3 py-2 rounded-xl border border-slate-200 bg-white text-xs font-mono text-slate-800 focus:outline-none focus:border-[#2196F3]"
                  />
                </div>
              </div>

              <div className="space-y-1">
                <label className="text-[11px] font-semibold text-slate-600">Account Holder Name</label>
                <div className="relative">
                  <input
                    type="text"
                    value={bankAccountName}
                    onChange={(e) => setBankAccountName(e.target.value)}
                    placeholder="e.g. NGUYEN VAN A"
                    className="w-full px-3 py-2 rounded-xl border border-slate-200 bg-white text-xs uppercase text-slate-800 focus:outline-none focus:border-[#2196F3]"
                  />
                </div>
              </div>
            </div>
          </div>

          {/* Proof Uploads */}
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <label className="text-xs font-bold text-[#0D1B2A] flex items-center gap-1.5">
                <span>Proof Photos & Video</span>
                <span className="text-[11px] font-normal text-slate-400">(Max 5 images, 1 video)</span>
              </label>
              <span className="text-[11px] text-slate-400">{previews.length}/6 files</span>
            </div>

            <div className="grid grid-cols-4 sm:grid-cols-6 gap-2.5">
              {previews.map((item, idx) => (
                <div
                  key={idx}
                  className="relative group aspect-square rounded-xl border border-slate-200 overflow-hidden bg-slate-100 flex items-center justify-center shadow-2xs"
                >
                  {item.isVideo ? (
                    <div className="relative w-full h-full bg-slate-900 flex flex-col items-center justify-center text-white">
                      <Film className="w-5 h-5 text-amber-400" />
                      <span className="text-[9px] font-bold mt-1">Video</span>
                    </div>
                  ) : (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img
                      src={item.url}
                      alt={`Preview ${idx + 1}`}
                      className="w-full h-full object-cover"
                    />
                  )}

                  <button
                    type="button"
                    onClick={() => handleRemoveFile(idx)}
                    className="absolute top-1 right-1 w-5 h-5 rounded-full bg-rose-600 text-white flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity shadow-xs"
                    title="Remove file"
                  >
                    <Trash2 className="w-3 h-3" />
                  </button>
                </div>
              ))}

              {previews.length < 6 && (
                <button
                  type="button"
                  onClick={() => fileInputRef.current?.click()}
                  className="aspect-square rounded-xl border-2 border-dashed border-slate-200 hover:border-[#2196F3] bg-slate-50/60 hover:bg-[#E3F2FD]/30 transition-all flex flex-col items-center justify-center gap-1 text-slate-400 hover:text-[#2196F3] cursor-pointer"
                >
                  <Upload className="w-4 h-4" />
                  <span className="text-[10px] font-bold">Upload</span>
                </button>
              )}
            </div>

            <input
              ref={fileInputRef}
              type="file"
              accept="image/*,video/*"
              multiple
              onChange={handleFileChange}
              className="hidden"
            />
          </div>

          {/* Form Actions */}
          <div className="flex gap-2.5 pt-3 border-t border-slate-100">
            <button
              type="button"
              disabled={isSubmitting}
              onClick={onClose}
              className="flex-1 py-2.5 rounded-xl bg-slate-100 hover:bg-slate-200 text-slate-700 text-xs font-bold transition-all cursor-pointer"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isSubmitting}
              className="flex-1 py-2.5 rounded-xl bg-[#2196F3] hover:bg-[#0D47A1] text-white text-xs font-bold transition-all shadow-md shadow-blue-500/20 flex items-center justify-center gap-2 cursor-pointer disabled:opacity-50"
            >
              {isSubmitting ? (
                <>
                  <Loader2 className="w-4 h-4 animate-spin" />
                  <span>Submitting Request...</span>
                </>
              ) : (
                <>
                  <RotateCcw className="w-4 h-4" />
                  <span>Submit Return Request</span>
                </>
              )}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
