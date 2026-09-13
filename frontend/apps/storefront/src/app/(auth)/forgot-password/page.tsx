'use client'

import React, { useState } from 'react'
import Link from 'next/link'
import { Store, Mail, ArrowRight, ArrowLeft, ShieldCheck, CheckCircle2, RefreshCw } from 'lucide-react'
import { toast } from 'sonner'
import { extractErrorMessage } from '@/lib/utils'

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState('')
  const [loading, setLoading] = useState(false)
  const [isSubmitted, setIsSubmitted] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)

    try {
      const baseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
      const apiKey = process.env.NEXT_PUBLIC_API_KEY || 'abcd-1234-1111'

      const res = await fetch(`${baseUrl}/api/v1/auth/forgot-password`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-API-KEY': apiKey,
        },
        body: JSON.stringify({ email: email.trim().toLowerCase() }),
      })

      const data = await res.json().catch(() => null)

      if (!res.ok) {
        throw new Error(extractErrorMessage(data, 'Failed to request password reset. Please try again.'))
      }

      setIsSubmitted(true)
      toast.success('Password reset instructions sent to your email!')
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'An error occurred. Please try again.'
      toast.error(message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-[80vh] flex items-center justify-center py-16 px-4 sm:px-6 bg-[#FAFBFC]">
      <div className="max-w-md w-full space-y-6">
        {/* Brand Header */}
        <div className="text-center space-y-2">
          <Link href="/" className="inline-flex items-center gap-2 mb-2">
            <div className="w-10 h-10 rounded-xl bg-gradient-to-tr from-[#0D47A1] to-[#2196F3] flex items-center justify-center text-white shadow-md">
              <Store className="w-6 h-6" />
            </div>
            <span className="text-2xl font-black tracking-tight text-[#0D1B2A]">
              GO<span className="text-[#2196F3]">shop</span>
            </span>
          </Link>
          <h1 className="text-2xl font-extrabold text-[#0D1B2A] tracking-tight">
            Reset Password
          </h1>
          <p className="text-xs text-slate-500">
            Enter your registered email address and we&apos;ll send you instructions to reset your password.
          </p>
        </div>

        {/* Form Card */}
        <div className="p-8 rounded-3xl bg-white border border-slate-200/80 shadow-lg space-y-5">
          {isSubmitted ? (
            <div className="text-center space-y-4 py-2">
              <div className="w-14 h-14 rounded-full bg-emerald-50 text-emerald-500 flex items-center justify-center mx-auto shadow-2xs">
                <CheckCircle2 className="w-8 h-8" />
              </div>
              <div className="space-y-1">
                <h3 className="text-base font-bold text-[#0D1B2A]">
                  Check Your Email
                </h3>
                <p className="text-xs text-slate-600 leading-relaxed">
                  We have sent password reset instructions to{' '}
                  <strong className="text-slate-800 font-semibold">{email}</strong>.
                </p>
              </div>

              <div className="p-3.5 rounded-2xl bg-slate-50 border border-slate-100 text-[11px] text-slate-500 text-left space-y-1">
                <p>• If you don&apos;t see the email, please check your Spam or Junk folder.</p>
                <p>• The reset link/token is valid for a limited period for security reasons.</p>
              </div>

              <div className="pt-2 space-y-2.5">
                <Link
                  href="/reset-password"
                  className="w-full py-2.5 px-4 rounded-xl bg-[#2196F3] text-white text-xs font-bold flex items-center justify-center gap-2 hover:bg-[#0D47A1] transition-all shadow-md"
                >
                  <span>Enter Reset Token</span>
                  <ArrowRight className="w-4 h-4" />
                </Link>

                <button
                  type="button"
                  onClick={() => setIsSubmitted(false)}
                  className="w-full py-2.5 px-4 rounded-xl bg-white border border-slate-200 text-slate-700 text-xs font-semibold hover:bg-slate-50 transition-all"
                >
                  Resend Email
                </button>
              </div>
            </div>
          ) : (
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-slate-700">
                  Email Address
                </label>
                <div className="relative">
                  <input
                    type="email"
                    required
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder="name@example.com"
                    className="w-full pl-9 pr-3.5 py-2.5 rounded-xl bg-slate-50 border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3]"
                  />
                  <Mail className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
                </div>
              </div>

              <button
                type="submit"
                disabled={loading || !email.trim()}
                className="w-full py-3 px-4 rounded-xl bg-[#2196F3] text-white text-xs font-bold flex items-center justify-center gap-2 hover:bg-[#0D47A1] transition-all shadow-md disabled:bg-slate-300 cursor-pointer"
              >
                {loading ? (
                  <div className="w-4 h-4 rounded-full border-2 border-white border-t-transparent animate-spin" />
                ) : (
                  <>
                    <span>Send Reset Instructions</span>
                    <ArrowRight className="w-4 h-4" />
                  </>
                )}
              </button>
            </form>
          )}

          <div className="pt-2 border-t border-slate-100 flex items-center justify-between text-xs text-slate-500">
            <Link
              href="/login"
              className="inline-flex items-center gap-1 font-semibold text-slate-600 hover:text-[#2196F3] transition-colors"
            >
              <ArrowLeft className="w-3.5 h-3.5" />
              <span>Back to Sign In</span>
            </Link>

            <Link
              href="/reset-password"
              className="font-semibold text-[#2196F3] hover:underline"
            >
              Have a token?
            </Link>
          </div>
        </div>

        {/* Security badge */}
        <div className="flex items-center justify-center gap-2 text-xs text-slate-400">
          <ShieldCheck className="w-4 h-4 text-emerald-500" />
          <span>Secure authentication protected by JWT & SSL</span>
        </div>
      </div>
    </div>
  )
}
