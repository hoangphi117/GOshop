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
        className="relative w-full max-w-lg bg-white rounded-md shadow-xl border border-[#E2E8F0] overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="p-5 border-b border-[#E2E8F0] flex items-center justify-between bg-[#F8FAFC]">
          <div className="flex items-center gap-3">
            <div className="w-9 h-9 rounded-md bg-rose-50 border border-rose-200 text-rose-600 flex items-center justify-center shadow-xs shrink-0">
              <Flag className="w-4 h-4" />
            </div>
            <div>
              <h2 className="font-heading text-sm sm:text-base font-bold text-[#0F172A]">Report Review</h2>
              <p className="text-xs text-[#64748B]">
                {reviewAuthor ? `Reporting review by ${reviewAuthor}` : 'Help us maintain a safe and honest community'}
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="w-8 h-8 rounded-md text-[#64748B] hover:text-[#0F172A] hover:bg-[#E2E8F0]/60 flex items-center justify-center transition-colors cursor-pointer"
            aria-label="Close report dialog"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Form Body */}
        <form onSubmit={handleSubmit} className="p-5 space-y-4">
          {/* Reason Selection */}
          <div className="space-y-2">
            <label className="text-xs font-bold text-[#0F172A] block">
              Why are you reporting this review? <span className="text-rose-500">*</span>
            </label>
            <div className="space-y-1.5">
              {REPORT_REASONS.map((r) => {
                const isSelected = reason === r.id
                return (
                  <label
                    key={r.id}
                    className={`flex items-start gap-2.5 p-2.5 rounded-md border cursor-pointer transition-all ${
                      isSelected
                        ? 'border-[#007D9C] bg-[#F0F9FB] ring-1 ring-[#007D9C]'
                        : 'border-[#E2E8F0] hover:border-[#CBD5E1] bg-white'
                    }`}
                  >
                    <input
                      type="radio"
                      name="reportReason"
                      value={r.id}
                      checked={isSelected}
                      onChange={() => setReason(r.id)}
                      className="mt-0.5 text-[#007D9C] focus:ring-[#007D9C] h-4 w-4"
                    />
                    <div className="text-xs space-y-0.5">
                      <div className="font-bold text-[#0F172A]">{r.label}</div>
                      <div className="text-[#64748B] text-[11px] leading-relaxed">
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
              <label className="font-bold text-[#0F172A]">
                Additional Details <span className="text-[#64748B] font-normal">(Optional)</span>
              </label>
              <span className="text-[11px] text-[#94A3B8] font-mono">
                {details.length}/500
              </span>
            </div>
            <textarea
              rows={3}
              maxLength={500}
              value={details}
              onChange={(e) => setDetails(e.target.value)}
              placeholder="Provide any additional context to help our moderation team evaluate this report..."
              className="w-full px-3 py-2 rounded-md bg-white border border-[#E2E8F0] text-xs text-[#0F172A] placeholder:text-[#94A3B8] focus:outline-none focus:border-[#007D9C] focus:ring-1 focus:ring-[#007D9C] resize-none"
            />
          </div>

          {/* Guidelines Notice */}
          <div className="p-3 rounded-md bg-amber-50 border border-amber-200/60 flex items-start gap-2 text-[11px] text-amber-900 leading-relaxed">
            <ShieldAlert className="w-4 h-4 text-amber-600 shrink-0 mt-0.5" />
            <span>
              False or malicious reporting violates our terms. All reports are investigated by our security team.
            </span>
          </div>

          {/* Footer Actions */}
          <div className="pt-2 flex items-center justify-end gap-2.5 border-t border-[#E2E8F0]">
            <button
              type="button"
              onClick={onClose}
              disabled={isSubmitting}
              className="px-4 py-2 rounded-md border border-[#E2E8F0] text-[#64748B] hover:text-[#0F172A] text-xs font-semibold hover:bg-[#F8FAFC] transition-all cursor-pointer"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isSubmitting}
              className="inline-flex items-center gap-1.5 px-5 py-2 rounded-md bg-rose-600 hover:bg-rose-700 text-white text-xs font-bold transition-all shadow-xs cursor-pointer disabled:opacity-50"
            >
              {isSubmitting ? (
                <>
                  <Loader2 className="w-3.5 h-3.5 animate-spin" />
                  <span>Submitting...</span>
                </>
              ) : (
                <>
                  <Flag className="w-3.5 h-3.5" />
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
