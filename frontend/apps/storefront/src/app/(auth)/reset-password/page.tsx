'use client'

import React, { useState, useEffect, Suspense } from 'react'
import Link from 'next/link'
import { useRouter, useSearchParams } from 'next/navigation'
import { Store, Lock, KeyRound, ArrowRight, ArrowLeft, ShieldCheck, CheckCircle2, AlertCircle, Eye, EyeOff, RefreshCw } from 'lucide-react'
import { extractErrorMessage } from '@/lib/utils'

function ResetPasswordForm() {
  const router = useRouter()
  const searchParams = useSearchParams()

  const [token, setToken] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [loading, setLoading] = useState(false)
  const [isSuccess, setIsSuccess] = useState(false)
  const [generalError, setGeneralError] = useState<string | null>(null)
  const [fieldErrors, setFieldErrors] = useState<{
    token?: string
    newPassword?: string
    confirmPassword?: string
  }>({})

  useEffect(() => {
    const tokenFromUrl = searchParams.get('token')
    if (tokenFromUrl) {
      setToken(tokenFromUrl)
    }
  }, [searchParams])

  const validate = () => {
    const errors: { token?: string; newPassword?: string; confirmPassword?: string } = {}

    if (!token.trim()) {
      errors.token = 'Please enter the reset token received via email'
    }

    if (!newPassword) {
      errors.newPassword = 'Please enter a new password'
    } else if (newPassword.length < 6) {
      errors.newPassword = 'Password must be at least 6 characters long'
    }

    if (!confirmPassword) {
      errors.confirmPassword = 'Please confirm your new password'
    } else if (confirmPassword !== newPassword) {
      errors.confirmPassword = 'Passwords do not match'
    }

    setFieldErrors(errors)
    return Object.keys(errors).length === 0
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setGeneralError(null)

    if (!validate()) return

    setLoading(true)

    try {
      const baseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
      const apiKey = process.env.NEXT_PUBLIC_API_KEY || 'abcd-1234-1111'

      const res = await fetch(`${baseUrl}/api/v1/auth/reset-password`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-API-KEY': apiKey,
        },
        body: JSON.stringify({
          token: token.trim(),
          new_password: newPassword,
        }),
      })

      const data = await res.json().catch(() => null)

      if (!res.ok) {
        throw new Error(
          extractErrorMessage(data, 'Failed to reset password. The token may be expired or invalid.')
        )
      }

      setIsSuccess(true)
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Failed to reset password.'
      setGeneralError(message)
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
            Create New Password
          </h1>
          <p className="text-xs text-slate-500">
            Enter your verification token and choose a strong new password for your account.
          </p>
        </div>

        {/* Form Card */}
        <div className="p-8 rounded-3xl bg-white border border-slate-200/80 shadow-lg space-y-5">
          {isSuccess ? (
            <div className="text-center space-y-4 py-2">
              <div className="w-14 h-14 rounded-full bg-emerald-50 text-emerald-500 flex items-center justify-center mx-auto shadow-2xs">
                <CheckCircle2 className="w-8 h-8" />
              </div>
              <div className="space-y-1">
                <h3 className="text-base font-bold text-[#0D1B2A]">
                  Password Reset Complete!
                </h3>
                <p className="text-xs text-slate-600 leading-relaxed">
                  Your password has been changed securely. You can now sign in with your new credentials.
                </p>
              </div>

              <div className="pt-2">
                <Link
                  href="/login"
                  className="w-full py-3 px-4 rounded-xl bg-[#2196F3] text-white text-xs font-bold flex items-center justify-center gap-2 hover:bg-[#0D47A1] transition-all shadow-md"
                >
                  <span>Sign In Now</span>
                  <ArrowRight className="w-4 h-4" />
                </Link>
              </div>
            </div>
          ) : (
            <form onSubmit={handleSubmit} noValidate className="space-y-4">
              {generalError && (
                <div className="p-3.5 rounded-2xl bg-rose-50 border border-rose-200/80 text-xs text-rose-600 flex items-start gap-2.5 animate-in fade-in duration-200">
                  <AlertCircle className="w-4 h-4 shrink-0 mt-0.5" />
                  <span className="font-medium leading-relaxed">{generalError}</span>
                </div>
              )}

              {/* Reset Token Field */}
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-slate-700">
                  Reset Token / Code
                </label>
                <div className="relative">
                  <input
                    type="text"
                    value={token}
                    onChange={(e) => {
                      setToken(e.target.value)
                      if (fieldErrors.token) setFieldErrors((prev) => ({ ...prev, token: undefined }))
                      if (generalError) setGeneralError(null)
                    }}
                    placeholder="Enter the reset token received via email"
                    className={`w-full pl-9 pr-3.5 py-2.5 rounded-xl border text-xs font-mono text-[#0D1B2A] focus:outline-none transition-colors ${
                      fieldErrors.token
                        ? 'border-rose-400 bg-rose-50/20 focus:border-rose-500'
                        : 'border-slate-200 bg-slate-50 focus:border-[#2196F3]'
                    }`}
                  />
                  <KeyRound className={`w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 ${
                    fieldErrors.token ? 'text-rose-400' : 'text-slate-400'
                  }`} />
                </div>
                {fieldErrors.token && (
                  <p className="text-[11px] font-medium text-rose-500 flex items-center gap-1 mt-1 animate-in fade-in duration-150">
                    <AlertCircle className="w-3 h-3 shrink-0" />
                    <span>{fieldErrors.token}</span>
                  </p>
                )}
              </div>

              {/* New Password Field */}
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-slate-700">
                  New Password
                </label>
                <div className="relative">
                  <input
                    type={showPassword ? 'text' : 'password'}
                    value={newPassword}
                    onChange={(e) => {
                      setNewPassword(e.target.value)
                      if (fieldErrors.newPassword) setFieldErrors((prev) => ({ ...prev, newPassword: undefined }))
                      if (generalError) setGeneralError(null)
                    }}
                    placeholder="At least 6 characters"
                    className={`w-full pl-9 pr-10 py-2.5 rounded-xl border text-xs text-[#0D1B2A] focus:outline-none transition-colors ${
                      fieldErrors.newPassword
                        ? 'border-rose-400 bg-rose-50/20 focus:border-rose-500'
                        : 'border-slate-200 bg-slate-50 focus:border-[#2196F3]'
                    }`}
                  />
                  <Lock className={`w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 ${
                    fieldErrors.newPassword ? 'text-rose-400' : 'text-slate-400'
                  }`} />
                  <button
                    type="button"
                    onClick={() => setShowPassword(!showPassword)}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600"
                  >
                    {showPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                  </button>
                </div>
                {fieldErrors.newPassword && (
                  <p className="text-[11px] font-medium text-rose-500 flex items-center gap-1 mt-1 animate-in fade-in duration-150">
                    <AlertCircle className="w-3 h-3 shrink-0" />
                    <span>{fieldErrors.newPassword}</span>
                  </p>
                )}
              </div>

              {/* Confirm Password Field */}
              <div className="space-y-1.5">
                <label className="text-xs font-semibold text-slate-700">
                  Confirm New Password
                </label>
                <div className="relative">
                  <input
                    type={showPassword ? 'text' : 'password'}
                    value={confirmPassword}
                    onChange={(e) => {
                      setConfirmPassword(e.target.value)
                      if (fieldErrors.confirmPassword) setFieldErrors((prev) => ({ ...prev, confirmPassword: undefined }))
                      if (generalError) setGeneralError(null)
                    }}
                    placeholder="Repeat your new password"
                    className={`w-full pl-9 pr-10 py-2.5 rounded-xl border text-xs text-[#0D1B2A] focus:outline-none transition-colors ${
                      fieldErrors.confirmPassword
                        ? 'border-rose-400 bg-rose-50/20 focus:border-rose-500'
                        : 'border-slate-200 bg-slate-50 focus:border-[#2196F3]'
                    }`}
                  />
                  <Lock className={`w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 ${
                    fieldErrors.confirmPassword ? 'text-rose-400' : 'text-slate-400'
                  }`} />
                </div>
                {fieldErrors.confirmPassword && (
                  <p className="text-[11px] font-medium text-rose-500 flex items-center gap-1 mt-1 animate-in fade-in duration-150">
                    <AlertCircle className="w-3 h-3 shrink-0" />
                    <span>{fieldErrors.confirmPassword}</span>
                  </p>
                )}
              </div>

              <button
                type="submit"
                disabled={loading}
                className="w-full py-3 px-4 rounded-xl bg-[#2196F3] text-white text-xs font-bold flex items-center justify-center gap-2 hover:bg-[#0D47A1] transition-all shadow-md disabled:bg-slate-300 cursor-pointer"
              >
                {loading ? (
                  <div className="w-4 h-4 rounded-full border-2 border-white border-t-transparent animate-spin" />
                ) : (
                  <>
                    <span>Reset Password</span>
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
              href="/forgot-password"
              className="font-semibold text-[#2196F3] hover:underline"
            >
              Request new token
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

export default function ResetPasswordPage() {
  return (
    <Suspense
      fallback={
        <div className="min-h-[80vh] flex items-center justify-center">
          <RefreshCw className="w-6 h-6 animate-spin text-[#2196F3]" />
        </div>
      }
    >
      <ResetPasswordForm />
    </Suspense>
  )
}
