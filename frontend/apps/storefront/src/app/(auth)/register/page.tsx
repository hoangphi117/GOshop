'use client'

import React, { useState } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { Store, Lock, Mail, User, ArrowRight, ShieldCheck, AlertCircle } from 'lucide-react'
import { GoogleLoginButton } from '@/components/auth/google-login-button'

export default function RegisterPage() {
  const router = useRouter()

  const [fullName, setFullName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [generalError, setGeneralError] = useState<string | null>(null)
  const [fieldErrors, setFieldErrors] = useState<{
    fullName?: string
    email?: string
    password?: string
  }>({})

  const validate = () => {
    const errors: { fullName?: string; email?: string; password?: string } = {}
    const trimmedName = fullName.trim()
    const trimmedEmail = email.trim()

    if (!trimmedName) {
      errors.fullName = 'Please enter your full name'
    } else if (trimmedName.length < 2) {
      errors.fullName = 'Full name must be at least 2 characters'
    }

    if (!trimmedEmail) {
      errors.email = 'Please enter your email address'
    } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(trimmedEmail)) {
      errors.email = 'Please enter a valid email address'
    }

    if (!password) {
      errors.password = 'Please enter a password'
    } else if (password.length < 6) {
      errors.password = 'Password must be at least 6 characters long'
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
      const res = await fetch(`${baseUrl}/api/v1/auth/register`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-API-KEY': process.env.NEXT_PUBLIC_API_KEY || 'abcd-1234-1111',
        },
        body: JSON.stringify({
          email: email.trim(),
          password,
          full_name: fullName.trim(),
        }),
      })

      const data = await res.json()

      if (!res.ok) {
        throw new Error(data?.message || 'Registration failed. Please try again.')
      }

      router.push('/login')
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'An error occurred during registration.'
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
            Create Your Account
          </h1>
          <p className="text-xs text-slate-500">
            Join thousands of shoppers and unlock exclusive deals and benefits.
          </p>
        </div>

        {/* Form Card */}
        <div className="p-8 rounded-3xl bg-white border border-slate-200/80 shadow-lg space-y-5">
          {/* General Error Banner */}
          {generalError && (
            <div className="p-3.5 rounded-2xl bg-rose-50 border border-rose-200/80 text-xs text-rose-600 flex items-start gap-2.5 animate-in fade-in duration-200">
              <AlertCircle className="w-4 h-4 shrink-0 mt-0.5" />
              <span className="font-medium leading-relaxed">{generalError}</span>
            </div>
          )}

          <form onSubmit={handleSubmit} noValidate className="space-y-4">
            {/* Full Name Field */}
            <div className="space-y-1.5">
              <label className="text-xs font-semibold text-slate-700">
                Full Name
              </label>
              <div className="relative">
                <input
                  type="text"
                  value={fullName}
                  onChange={(e) => {
                    setFullName(e.target.value)
                    if (fieldErrors.fullName) setFieldErrors((prev) => ({ ...prev, fullName: undefined }))
                    if (generalError) setGeneralError(null)
                  }}
                  placeholder="e.g. Alex Johnson"
                  className={`w-full pl-9 pr-3.5 py-2.5 rounded-xl border text-xs text-[#0D1B2A] focus:outline-none transition-colors ${
                    fieldErrors.fullName
                      ? 'border-rose-400 bg-rose-50/20 focus:border-rose-500'
                      : 'border-slate-200 bg-slate-50 focus:border-[#2196F3]'
                  }`}
                />
                <User className={`w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 ${
                  fieldErrors.fullName ? 'text-rose-400' : 'text-slate-400'
                }`} />
              </div>
              {fieldErrors.fullName && (
                <p className="text-[11px] font-medium text-rose-500 flex items-center gap-1 mt-1 animate-in fade-in duration-150">
                  <AlertCircle className="w-3 h-3 shrink-0" />
                  <span>{fieldErrors.fullName}</span>
                </p>
              )}
            </div>

            {/* Email Address Field */}
            <div className="space-y-1.5">
              <label className="text-xs font-semibold text-slate-700">
                Email Address
              </label>
              <div className="relative">
                <input
                  type="email"
                  value={email}
                  onChange={(e) => {
                    setEmail(e.target.value)
                    if (fieldErrors.email) setFieldErrors((prev) => ({ ...prev, email: undefined }))
                    if (generalError) setGeneralError(null)
                  }}
                  placeholder="name@example.com"
                  className={`w-full pl-9 pr-3.5 py-2.5 rounded-xl border text-xs text-[#0D1B2A] focus:outline-none transition-colors ${
                    fieldErrors.email
                      ? 'border-rose-400 bg-rose-50/20 focus:border-rose-500'
                      : 'border-slate-200 bg-slate-50 focus:border-[#2196F3]'
                  }`}
                />
                <Mail className={`w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 ${
                  fieldErrors.email ? 'text-rose-400' : 'text-slate-400'
                }`} />
              </div>
              {fieldErrors.email && (
                <p className="text-[11px] font-medium text-rose-500 flex items-center gap-1 mt-1 animate-in fade-in duration-150">
                  <AlertCircle className="w-3 h-3 shrink-0" />
                  <span>{fieldErrors.email}</span>
                </p>
              )}
            </div>

            {/* Password Field */}
            <div className="space-y-1.5">
              <label className="text-xs font-semibold text-slate-700">
                Password
              </label>
              <div className="relative">
                <input
                  type="password"
                  value={password}
                  onChange={(e) => {
                    setPassword(e.target.value)
                    if (fieldErrors.password) setFieldErrors((prev) => ({ ...prev, password: undefined }))
                    if (generalError) setGeneralError(null)
                  }}
                  placeholder="At least 6 characters"
                  className={`w-full pl-9 pr-3.5 py-2.5 rounded-xl border text-xs text-[#0D1B2A] focus:outline-none transition-colors ${
                    fieldErrors.password
                      ? 'border-rose-400 bg-rose-50/20 focus:border-rose-500'
                      : 'border-slate-200 bg-slate-50 focus:border-[#2196F3]'
                  }`}
                />
                <Lock className={`w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 ${
                  fieldErrors.password ? 'text-rose-400' : 'text-slate-400'
                }`} />
              </div>
              {fieldErrors.password && (
                <p className="text-[11px] font-medium text-rose-500 flex items-center gap-1 mt-1 animate-in fade-in duration-150">
                  <AlertCircle className="w-3 h-3 shrink-0" />
                  <span>{fieldErrors.password}</span>
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
                  <span>Create Account</span>
                  <ArrowRight className="w-4 h-4" />
                </>
              )}
            </button>
          </form>

          <div className="relative flex items-center justify-center my-3">
            <div className="border-t border-slate-200 w-full" />
            <span className="bg-white px-3 text-[11px] font-medium text-slate-400 uppercase tracking-wider whitespace-nowrap">
              Or sign up with
            </span>
            <div className="border-t border-slate-200 w-full" />
          </div>

          <GoogleLoginButton text="signup_with" onError={(msg) => setGeneralError(msg)} />

          <div className="pt-2 text-center text-xs text-slate-500">
            Already have an account?{' '}
            <Link
              href="/login"
              className="font-bold text-[#2196F3] hover:underline"
            >
              Sign In
            </Link>
          </div>
        </div>

        <div className="flex items-center justify-center gap-2 text-xs text-slate-400">
          <ShieldCheck className="w-4 h-4 text-emerald-500" />
          <span>Your privacy is guaranteed</span>
        </div>
      </div>
    </div>
  )
}
