'use client'

import React, { useState } from 'react'
import { authFetch } from '@/lib/api'
import { extractErrorMessage } from '@/lib/utils'
import {
  X,
  Flag,
  Loader2,
  AlertTriangle,
  CheckCircle2,
  ShieldAlert,
} from 'lucide-react'
import { toast } from 'sonner'

interface ReportReviewModalProps {
  isOpen: boolean
  onClose: () => void
  reviewUuid: string
  reviewAuthor?: string
}

const REPORT_REASONS = [
  {
    id: 'spam',
    label: 'Spam or Advertising',
    description: 'Promotional links, irrelevant advertisements, or automated content.',
  },
  {
    id: 'inappropriate',
    label: 'Inappropriate Content',
    description: 'Hate speech, profanity, harassment, or offensive remarks.',
  },
  {
    id: 'fake',
    label: 'Fake or Dishonest Review',
    description: 'Suspicious review not reflecting a genuine customer experience.',
  },
  {
    id: 'other',
    label: 'Other Policy Violation',
    description: 'Violates GOshop customer review terms or guidelines.',
  },
] as const

export function ReportReviewModal({
  isOpen,
  onClose,
  reviewUuid,
  reviewAuthor,
}: ReportReviewModalProps) {
  const [reason, setReason] = useState<'spam' | 'inappropriate' | 'fake' | 'other'>('spam')
  const [details, setDetails] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)

  if (!isOpen) return null

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setIsSubmitting(true)

    try {
      const baseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
      const payload: { reason: string; details?: string } = {
        reason,
      }
      if (details.trim()) {
        payload.details = details.trim()
      }

      const res = await authFetch(`${baseUrl}/api/v1/reviews/${reviewUuid}/report`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(payload),
      })

      if (res.ok) {
        toast.success('Thank you. Your report has been submitted to our moderation team.')
        setDetails('')
        setReason('spam')
        onClose()
      } else {
        const errJson = await res.json().catch(() => null)
        const msg = extractErrorMessage(errJson, 'Failed to submit report. Please try again.')
        toast.error(msg)
      }
    } catch {
      toast.error('Network error submitting review report.')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs animate-fade-in">
      <div
        className="relative w-full max-w-lg bg-white rounded-3xl shadow-2xl border border-slate-200 overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="p-6 border-b border-slate-100 flex items-center justify-between bg-slate-50/50">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-2xl bg-rose-50 border border-rose-200/60 text-rose-600 flex items-center justify-center shadow-xs">
              <Flag className="w-5 h-5" />
            </div>
            <div>
              <h2 className="text-base font-extrabold text-[#0D1B2A]">Report Review</h2>
              <p className="text-xs text-slate-500">
                {reviewAuthor ? `Reporting review by ${reviewAuthor}` : 'Help us maintain a safe community'}
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="w-8 h-8 rounded-full bg-slate-100 text-slate-400 hover:text-slate-600 hover:bg-slate-200 flex items-center justify-center transition-colors cursor-pointer"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Form Body */}
        <form onSubmit={handleSubmit} className="p-6 space-y-5">
          {/* Reason Selection */}
          <div className="space-y-2">
            <label className="text-xs font-bold text-slate-700 block">
              Why are you reporting this review? <span className="text-rose-500">*</span>
            </label>
            <div className="space-y-2">
              {REPORT_REASONS.map((r) => {
                const isSelected = reason === r.id
                return (
                  <label
                    key={r.id}
                    className={`flex items-start gap-3 p-3 rounded-2xl border cursor-pointer transition-all ${
                      isSelected
                        ? 'border-[#2196F3] bg-[#E3F2FD]/30 ring-1 ring-[#2196F3]'
                        : 'border-slate-200 hover:border-slate-300 bg-white'
                    }`}
                  >
                    <input
                      type="radio"
                      name="reportReason"
                      value={r.id}
                      checked={isSelected}
                      onChange={() => setReason(r.id)}
                      className="mt-1 text-[#2196F3] focus:ring-[#2196F3] h-4 w-4"
                    />
                    <div className="text-xs space-y-0.5">
                      <div className="font-bold text-[#0D1B2A]">{r.label}</div>
                      <div className="text-slate-500 text-[11px] leading-relaxed">
                        {r.description}
                      </div>
                    </div>
                  </label>
                )
              })}
            </div>
          </div>

          {/* Details / Additional Comments */}
          <div className="space-y-1.5">
            <div className="flex items-center justify-between text-xs">
              <label className="font-bold text-slate-700">
                Additional Details <span className="text-slate-400 font-normal">(Optional)</span>
              </label>
              <span className="text-[11px] text-slate-400 font-mono">
                {details.length}/500
              </span>
            </div>
            <textarea
              rows={3}
              maxLength={500}
              value={details}
              onChange={(e) => setDetails(e.target.value)}
              placeholder="Provide any additional context to help our moderation team evaluate this report..."
              className="w-full px-3.5 py-2.5 rounded-2xl bg-slate-50 border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3] focus:bg-white resize-none"
            />
          </div>

          {/* Guidelines Notice */}
          <div className="p-3.5 rounded-2xl bg-amber-50 border border-amber-200/60 flex items-start gap-2.5 text-[11px] text-amber-900 leading-relaxed">
            <ShieldAlert className="w-4 h-4 text-amber-600 shrink-0 mt-0.5" />
            <span>
              False or malicious reporting is a violation of our community standards. All submissions are manually evaluated by our team.
            </span>
          </div>

          {/* Footer Actions */}
          <div className="pt-2 flex items-center justify-end gap-3 border-t border-slate-100">
            <button
              type="button"
              onClick={onClose}
              disabled={isSubmitting}
              className="px-5 py-2.5 rounded-xl border border-slate-200 text-slate-600 text-xs font-bold hover:bg-slate-50 transition-all cursor-pointer"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isSubmitting}
              className="inline-flex items-center gap-2 px-6 py-2.5 rounded-xl bg-rose-600 hover:bg-rose-700 text-white text-xs font-bold transition-all shadow-md cursor-pointer disabled:opacity-50"
            >
              {isSubmitting ? (
                <>
                  <Loader2 className="w-4 h-4 animate-spin" />
                  <span>Submitting Report...</span>
                </>
              ) : (
                <>
                  <Flag className="w-4 h-4" />
                  <span>Submit Report</span>
                </>
              )}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
